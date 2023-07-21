package esblib

import (
	"testing"
)

const fragment = `<!DOCTYPE html PUBLIC '-//W3C//DTD XHTML 1.0 Transitional//EN' 'http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd'>
<html xmlns='http://www.w3.org/1999/xhtml'>
<head><title>Logging in...</title>
<meta name='CACHE-CONTROL' content='NO-CACHE'/>
<meta name='PRAGMA' content='NO-CACHE'/>
<meta name='EXPIRES' content='-1'/>
</head>
<body>
<form id='auto' method='post' action='https://myaccount.esbnetworks.ie/signin-oidc'>
<div><input type='hidden' name='state' id='state_id' value='state-val-1-2'/>
<input type='hidden' name='client_info' id='client_info' value='client_val'/>
<input type='hidden' name='code' id='code' value='code.val-1-2-3'/>
</div>
<div id='noJavascript' style='visibility: visible; font-family: Verdana'>
<p>Although we have detected that you have Javascript disabled, you will be able to use the site as normal.</p>
<p>As part of the authentication process this page may be displayed several times. Please use the continue button below.</p>
<input type='submit' value='Continue' /></div><script type='text/javascript'>
<!-- 
	document.getElementById('noJavascript').innerHTML = ''; document.getElementById('auto').submit(); 
//--></script></form></body></html>`

func TestHTMLFormToRequest(t *testing.T) {
	req, err := htmlFormToRequest(([]byte)(fragment))
	if err != nil {
		t.Errorf("htmlFormToRequest() unexpected error: %v", err)
	}

	if req.Method != "post" {
		t.Errorf("htmlFormToRequest() got method %q want POST", req.Method)
	}
	const wantURL = "https://myaccount.esbnetworks.ie/signin-oidc"
	if got := req.URL.String(); got != wantURL {
		t.Errorf("htmlFormToRequest() got URL %q want %q", got, wantURL)
	}
	// TODO: it would be nice to test we pass the form data.
}
