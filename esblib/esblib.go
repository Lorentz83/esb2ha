package esblib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

const (
	baseURL = `https://myaccount.esbnetworks.ie`
	dataURL = `https://myaccount.esbnetworks.ie/DataHub/DownloadHdf?mprn=`
)

// attributesToMap returns a map of key value attributes on the default namespace.
func attributesToMap(aa []html.Attribute) map[string]string {
	ret := map[string]string{}
	for _, a := range aa {
		if a.Namespace != "" {
			continue
		}
		ret[a.Key] = a.Val
	}
	return ret
}

// htmlFormToRequest parses an HTML fragment looking for a form and populating
// an http.Request with the hidden data from the form.
func htmlFormToRequest(fragment []byte) (*http.Request, error) {
	doc, err := html.Parse(bytes.NewReader(fragment))
	if err != nil {
		return nil, fmt.Errorf("cannot parse HTML: %w", err)
	}

	var (
		method string
		reqURL string
		data   = url.Values{}
	)

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			attrs := attributesToMap(n.Attr)
			if attrs["type"] == "hidden" {
				data.Set(attrs["name"], attrs["value"])
			}
		}
		if n.Type == html.ElementNode && n.Data == "form" {
			attrs := attributesToMap(n.Attr)
			method = attrs["method"]
			reqURL = attrs["action"]
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)

	req, err := http.NewRequest(method, reqURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	return req, nil
}

// Client connects to esbnetworks.ie website to download usage data.
type Client struct {
	username string
	password string
	mprn     string
	hc       *http.Client
}

// NewClient returns a new ESB client.
func NewClient(user, password, mprn string) (*Client, error) {
	if user == "" {
		return nil, errors.New("missing user name")
	}
	if password == "" {
		return nil, errors.New("missing password")
	}
	if mprn == "" {
		return nil, errors.New("missing mprn")
	}

	j, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	return &Client{
		username: user,
		password: password,
		mprn:     mprn,
		hc: &http.Client{
			Jar: j,
		},
	}, nil
}

func (c *Client) sendLogin(pr initPhaseResult) error {
	u, err := pr.postURL()
	if err != nil {
		return err
	}

	data := url.Values{}
	data.Set("signInName", c.username)
	data.Set("password", c.password)
	data.Set("request_type", "RESPONSE")

	req, err := http.NewRequest("POST", u.String(), strings.NewReader(data.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	req.Header.Set("Cookie", pr.brokenCookieHeader)

	csrf, err := pr.csrf()
	if err != nil {
		return err
	}

	req.Header.Set("X-CSRF-TOKEN", csrf)

	rsp, err := c.hc.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	if string(body) == "Bad Request" {
		return errors.New("bad request")
	}

	var rs struct{ Status string }
	if err := json.Unmarshal(body, &rs); err != nil {
		return fmt.Errorf("cannot parse response: %w", err)
	}
	if rs.Status != "200" {
		return fmt.Errorf("invalid status %v", string(body))
	}

	return nil
}

func (c *Client) getRedirect(pr initPhaseResult) (*http.Request, error) {
	url, err := pr.reidirectURL()
	if err != nil {
		return nil, err
	}

	rsp, err := c.hc.Get(url.String())
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read http response: %w", err)
	}

	return htmlFormToRequest(body)
}

func (c *Client) prepare() (initPhaseResult, error) {
	rsp, err := c.hc.Get(baseURL)
	if err != nil {
		return initPhaseResult{}, err
	}

	const brokenCookieName = "x-ms-cpim-sso:esbntwkscustportalprdb2c01.onmicrosoft.com_0"
	var brokenCookieHeader string
	for _, raw := range rsp.Header.Values("Set-Cookie") {
		if !strings.HasPrefix(raw, brokenCookieName) {
			continue
		}
		r := http.Request{
			Header: http.Header{},
		}

		r.Header.Add("Cookie", strings.Replace(raw, brokenCookieName, "c", 1))
		cc := r.Cookies()[0]
		brokenCookieValue := cc.Value

		cookie := http.Cookie{
			Name:  "brokenCookieName",
			Value: brokenCookieValue,
		}
		brokenCookieHeader = strings.Replace(cookie.String(), "brokenCookieName", brokenCookieName, 1)
		break
	}
	if brokenCookieHeader == "" {
		return initPhaseResult{}, errors.New("cannot find broken cookie")
	}

	b, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return initPhaseResult{}, err
	}
	const settingsPrefix = "var SETTINGS = "
	var settings string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, settingsPrefix) {
			settings = l
			break
		}
	}
	if settings == "" {
		return initPhaseResult{}, errors.New("cannot find page settings")
	}
	settings = strings.TrimPrefix(settings, settingsPrefix)
	settings = strings.TrimRightFunc(settings, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t' || r == ';' || r == ' '
	})
	sj := map[string]any{}
	if err := json.Unmarshal(([]byte)(settings), &sj); err != nil {
		return initPhaseResult{}, err
	}

	return initPhaseResult{
		brokenCookieHeader: brokenCookieHeader,
		loginURL:           rsp.Request.URL,
		settings:           sj,
	}, nil
}

// Login logs in into esb.
func (c *Client) Login() error {
	pr, err := c.prepare()
	if err != nil {
		return err
	}

	if err := c.sendLogin(pr); err != nil {
		return err
	}

	req, err := c.getRedirect(pr)
	if err != nil {
		return err
	}

	rsp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("cannot auth on ESB: status %v", rsp.Status)
	}
	return nil
}

func (c *Client) Download() error {
	rsp, err := c.hc.Get(dataURL + c.mprn)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("cannot download: status %v", rsp.Status)
	}
	body, _ := ioutil.ReadAll(rsp.Body)
	fmt.Printf("rsp %s\n\n", string(body))
	return nil
}

type initPhaseResult struct {
	brokenCookieHeader string
	loginURL           *url.URL
	settings           map[string]any
}

func (rs initPhaseResult) csrf() (string, error) {
	c, ok := rs.settings["csrf"]
	if !ok {
		return "", errors.New("cannot find csrf settings")
	}
	ret, ok := c.(string)
	if !ok {
		return "", fmt.Errorf("csrf setting is of type %T instead of string", c)
	}
	return ret, nil
}

func (rs initPhaseResult) postURL() (*url.URL, error) {
	e := rs.settings
	h, ok := e["hosts"].(map[string]any)
	if !ok {
		return nil, errors.New("cannot find hosts settings")
	}

	ret := h["tenant"].(string) + "/SelfAsserted" // somethins seems override h[api] with SelfAsserted
	q := "tx=" + e["transId"].(string) + "&p=" + h["policy"].(string)

	return &url.URL{
		Scheme:   rs.loginURL.Scheme,
		Host:     rs.loginURL.Host,
		Path:     ret,
		RawQuery: q,
	}, nil
}

func (rs initPhaseResult) reidirectURL() (*url.URL, error) {
	// URL from getRedirectLink js function
	// called by $i2e.redirectToServer("confirmed?rememberMe="+i,!0),!1}
	e := rs.settings
	h, ok := e["hosts"].(map[string]any)
	if !ok {
		return nil, errors.New("cannot find hosts settings")
	}
	csrf, err := rs.csrf()
	if err != nil {
		return nil, err
	}
	q := "rememberMe=false&csrf_token=" + csrf + "&tx=" + e["transId"].(string) + "&p" + h["policy"].(string)
	path := h["tenant"].(string) + "/api/" + e["api"].(string) + "/confirmed"

	return &url.URL{
		Scheme:   rs.loginURL.Scheme,
		Host:     rs.loginURL.Host,
		Path:     path,
		RawQuery: q,
	}, nil
}
