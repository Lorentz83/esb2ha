// Package esblib implements an interface to myaccount.esbnetworks.ie to read power consumption data.
package esblib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
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
	// Both the clients share the same cookie jar, but the second
	// is configured to not follow redirects. It is useful to identify expired logins.
	hc         *http.Client
	noRedirect *http.Client
}

// NewClient returns a new ESB client.
func NewClient() (*Client, error) {
	j, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return nil, err
	}

	return &Client{
		hc: &http.Client{
			Jar: j,
		},
		noRedirect: &http.Client{
			Jar: j,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// Login logs in into esb.
//
// Note that login expires after a short amount of minutes (currently 20).
// Unless you need to download usage data for multiple smart meters from the
// same account, you should always call Login immediately before DownloadPowerConsumption.
func (c *Client) Login(user, password string) error {
	if user == "" {
		return errors.New("missing user name")
	}
	if password == "" {
		return errors.New("missing password")
	}

	pr, err := c.loadLoginPage()
	if err != nil {
		return err
	}

	if err := c.postLogin(pr, user, password); err != nil {
		return err
	}

	req, err := c.getRedirect(pr)
	if err != nil {
		return err
	}

	if err := c.finalizeLogin(req); err != nil {
		return err
	}

	return nil
}

// loadLoginPage is the 1st step of the login process.
//
// It returns the login settings required by the next steps.
func (c *Client) loadLoginPage() (loginSettings, error) {
	rsp, err := c.hc.Get(baseURL)
	if err != nil {
		return loginSettings{}, err
	}
	defer rsp.Body.Close()

	b, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return loginSettings{}, err
	}

	const settingsPrefix = "var SETTINGS = "
	// Here we assume that SETTINGS is on a single line.
	// To make it more robust we should use some JS parser.
	var settings string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, settingsPrefix) {
			settings = l
			break
		}
	}
	if settings == "" {
		return loginSettings{}, errors.New("cannot find page settings")
	}
	settings = strings.TrimPrefix(settings, settingsPrefix)
	settings = strings.TrimRightFunc(settings, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t' || r == ';' || r == ' '
	})

	var sj pageSettings
	if err := json.Unmarshal(([]byte)(settings), &sj); err != nil {
		return loginSettings{}, err
	}

	return loginSettings{
		loginURL: rsp.Request.URL,
		settings: sj,
	}, nil
}

// postLogin is the 2nd step of the login process.
//
// It is the one which actually sends the login information for authentication.
func (c *Client) postLogin(ls loginSettings, user, password string) error {
	u := ls.PostLoginURL()

	data := url.Values{}
	data.Set("signInName", user)
	data.Set("password", password)
	data.Set("request_type", "RESPONSE")

	req, err := http.NewRequest("POST", u.String(), strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-CSRF-TOKEN", ls.CSRF())

	rsp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	if string(body) == "Bad Request" {
		return errors.New("bad request")
	}

	var rs struct{ Status, ErrorCode, Message string }
	if err := json.Unmarshal(body, &rs); err != nil {
		return fmt.Errorf("cannot parse response: %w", err)
	}
	if rs.Status != "200" {
		if rs.Message == "" {
			return fmt.Errorf("invalid status %v", string(body))
		}
		return fmt.Errorf("error %s: %s", rs.ErrorCode, rs.Message)
	}

	return nil
}

// getRedirect is the 3rd step of the login process.
//
// It loads teh redirect page and parses its content to return
// the last request required to move back the authentication results to
// the ESB website.
func (c *Client) getRedirect(pr loginSettings) (*http.Request, error) {
	url := pr.RedirectURL()

	rsp, err := c.hc.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read http response: %w", err)
	}

	return htmlFormToRequest(body)
}

// finalizeLogin is the 4th and last step of the login.
//
// Here we load the actual ESB website and authenticate on it.
func (c *Client) finalizeLogin(req *http.Request) error {
	rsp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("cannot auth on ESB: status %v", rsp.Status)
	}
	return nil
}

// DownloadPowerConsumption downloads the electricity usage data.
//
// You have to had a successful call of login in the last few minutes
// (currently 20) or you'll get an error here.
//
// It is the equivalent of "Download HDF" button on the website.
// The format is what is called an "Harmonised Downloadable File (HDF)",
// which is just a CSV with the header in the 1st line.
//
// Currently, the format is:
//
//	MPRN,Meter Serial Number,Read Value,Read Type,Read Date and End Time
//
// To the best of my knowledge, the only useful fields are "Read Value"
// and "Read Date and End Time" because all the others have fixed value.
func (c *Client) DownloadPowerConsumption(mprn string) ([]byte, error) {
	if mprn == "" {
		return nil, errors.New("missing mprn")
	}

	rsp, err := c.noRedirect.Get(dataURL + mprn)
	if err != nil {
		return nil, err
	}
	switch rsp.StatusCode {
	case http.StatusOK:
		// No error, move on.
		break
	case http.StatusFound:
		return nil, errors.New("login expired or invalid")
	case http.StatusNotFound:
		return nil, fmt.Errorf("not found, is the mprn %q correct and linked to this account?", mprn)
	default:
		return nil, fmt.Errorf("status %v", rsp.Status)
	}
	defer rsp.Body.Close()

	// We could stream data to save some memory, but I don't like the idea of
	// having a pending HTTP request around for too long.
	// We can change and optimize later if needed.
	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// pageSettings is a struct to de-serialize the "SETTINGS" json defined on top of the login page.
// Only the fields required to perform the login are defined here.
type pageSettings struct {
	CSRF    string `json:"csrf"`
	TransID string `json:"transId"`
	API     string `json:"api"`
	Hosts   struct {
		Tenant string `json:"tenant"`
		Policy string `json:"policy"`
	} `json:"hosts"`
}

type loginSettings struct {
	// LoginURL is actual login URL, which is the one we get redirected from baseURL.
	loginURL *url.URL
	// internal page settings.
	settings pageSettings
}

// CSRF returns the CSRF value to set as header.
func (ls loginSettings) CSRF() string {
	return ls.settings.CSRF
}

// PostLoginURL is the URL to post login data.
func (ls loginSettings) PostLoginURL() *url.URL {
	// URL used by the _signIn js function.
	s := ls.settings
	return &url.URL{
		Scheme:   ls.loginURL.Scheme,
		Host:     ls.loginURL.Host,
		Path:     s.Hosts.Tenant + "/SelfAsserted", // h[api] is overridden with SelfAsserted
		RawQuery: "tx=" + s.TransID + "&p=" + s.Hosts.Policy,
	}
}

func (ls loginSettings) RedirectURL() *url.URL {
	// URL from getRedirectLink js function
	// called by $i2e.redirectToServer("confirmed?rememberMe="+i,!0),!1}
	s := ls.settings

	csrf := ls.CSRF()
	return &url.URL{
		Scheme:   ls.loginURL.Scheme,
		Host:     ls.loginURL.Host,
		Path:     s.Hosts.Tenant + "/api/" + s.API + "/confirmed",
		RawQuery: "rememberMe=false&csrf_token=" + csrf + "&tx=" + s.TransID + "&p" + s.Hosts.Policy,
	}
}
