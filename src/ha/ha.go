// Package ha implements the websocket connection to Home Assistant.
package ha

import (
	"context"
	"errors"
	"fmt"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// StatisticMetadata is the metadata of a statistic value.
type StatisticMetadata struct {
	recorderSource

	HasMean           bool   `json:"has_mean"`
	HasSum            bool   `json:"has_sum"`
	Name              string `json:"name"`
	StatisticID       string `json:"statistic_id"`
	UnitOfMeasurement string `json:"unit_of_measurement"`
}

// recorderSource is just a hack to force the json encoder to add a
// constant field `"source":"recorder"`
type recorderSource struct {
	Source recorderString `json:"source"`
}

// recorderString is a string which is always marshalled in json as
// the constant "recorder"
type recorderString string

func (recorderString) MarshalText() ([]byte, error) { return []byte(`recorder`), nil }

// StatisticValue is a single data point to import to Home Assistant.
type StatisticValue struct {
	Start     time.Time `json:"start"`
	Mean      float64   `json:"mean,omitempty"`
	Min       float64   `json:"min,omitempty"`
	Max       float64   `json:"max,omitempty"`
	LastReset time.Time `json:"last_reset,omitempty"`
	State     float64   `json:"state,omitempty"`
	Sum       float64   `json:"sum,omitempty"`
}

// Statistics is bundle of metadata and values to be sent to Home Assistant.
type Statistics struct {
	Metadata StatisticMetadata `json:"metadata"`
	// TODO: it would be nice to have the int version too.
	Stats []StatisticValue `json:"stats"`
}

type response struct {
	ID          int            `json:"id"`
	MessageType string         `json:"type"`
	Success     bool           `json:"success"`
	Result      map[string]any `json:"result"`
	Error       struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Connection is a websocket connection to Home Assistant.
type Connection struct {
	conn  *websocket.Conn
	msgID int
	// ServerVersion contains the version of the connected Home Assistant server.
	ServerVersion string
}

// NewConnection returns a new Connection.
//
// The host is just name:port, name, ip, ip:port without any protocol handler.
// To get the token you can follow instructions at
// https://www.home-assistant.io/docs/authentication/#your-account-profile
func NewConnection(ctx context.Context, host, accessToken string) (*Connection, error) {
	url := "ws://" + host + "/api/websocket"
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	ret := &Connection{
		conn:  ws,
		msgID: 42,
	}
	if err := ret.login(ctx, accessToken); err != nil {
		ret.Close()
		return nil, err
	}
	return ret, nil
}

func (c *Connection) incMessageID() int {
	r := c.msgID
	c.msgID++
	return r
}

// Close gracefully closes the connection and invalidates the connection.
func (c *Connection) Close() error {
	return c.conn.Close(websocket.StatusGoingAway, "bye")
}

// login sends the access token to authenticate to home assistant.
func (c *Connection) login(ctx context.Context, accessToken string) error {

	type authResponseMessage struct {
		HAVersion   string `json:"ha_version"`
		MessageType string `json:"type"`
	}

	// https://developers.home-assistant.io/docs/api/websocket/
	var rsp authResponseMessage
	if err := wsjson.Read(ctx, c.conn, &rsp); err != nil {
		return err
	}
	if rsp.HAVersion == "" {
		return errors.New("cannot get home assistant version")
	}
	if rsp.MessageType != "auth_required" {
		return fmt.Errorf("expected auth_required, got %q", rsp.MessageType)
	}
	c.ServerVersion = rsp.HAVersion

	auth := map[string]string{
		"type":         "auth",
		"access_token": accessToken,
	}
	if err := wsjson.Write(ctx, c.conn, auth); err != nil {
		return err
	}

	rsp = authResponseMessage{}
	if err := wsjson.Read(ctx, c.conn, &rsp); err != nil {
		return err
	}
	if rsp.MessageType != "auth_ok" {
		return fmt.Errorf("invalid auth: %s", rsp.MessageType)
	}
	return nil
}

// SendStatistics sends the statistic to home assistant.
//
// This function is NOT safe for concurrent calls.
func (c *Connection) SendStatistics(ctx context.Context, stat Statistics) error {
	// server: https://github.com/home-assistant/core/blob/dev/homeassistant/components/recorder/websocket_api.py#L449
	// ex: https://gitlab.com/hydroqc/hydroqc2mqtt/-/blob/main/hydroqc2mqtt/hourly_consump_handler.py

	id := c.incMessageID()

	msg := struct {
		Type string `json:"type"`
		ID   int    `json:"id"`
		Statistics
	}{
		"recorder/import_statistics",
		id,
		stat,
	}

	if err := wsjson.Write(ctx, c.conn, msg); err != nil {
		return err
	}

	_, err := c.waitResponse(ctx, id)
	return err
}

func (c *Connection) waitResponse(ctx context.Context, id int) (rsp response, err error) {
	if err := wsjson.Read(ctx, c.conn, &rsp); err != nil {
		return rsp, err
	}
	if rsp.ID != id {
		return rsp, fmt.Errorf("protocol out of sync: got ack for %d, want %d", rsp.ID, id)
	}
	if rsp.MessageType != "result" || !rsp.Success {
		return rsp, fmt.Errorf("error %s: %s", rsp.Error.Code, rsp.Error.Message)
	}
	return rsp, nil
}
