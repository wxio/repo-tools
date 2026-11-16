package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSendSlack_PostsTextAsJSON checks the method, the content type, and
// that the decoded body's text field is exactly the message, including a
// quote and a newline the JSON encoding must escape and restore.
func TestSendSlack_PostsTextAsJSON(t *testing.T) {
	var (
		gotMethod      string
		gotContentType string
		gotBody        slackBody
		want           = "a line with a \"quote\"\nand a second line"
		srv            *httptest.Server
		err            error
	)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		if err = json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err = SendSlack(context.Background(), srv.URL, want); err != nil {
		t.Fatalf("SendSlack: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody.Text != want {
		t.Errorf("text = %q, want %q", gotBody.Text, want)
	}
}

// TestSendSlack_EmptyURLSendsNothing checks PRD section 4.3: a transport
// with no key configured is skipped in silence. A server that fails the
// test if it is ever reached proves nothing was sent.
func TestSendSlack_EmptyURLSendsNothing(t *testing.T) {
	var (
		srv *httptest.Server
		err error
	)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the server was reached with an empty url")
	}))
	defer srv.Close()

	if err = SendSlack(context.Background(), "", "a message"); err != nil {
		t.Errorf("SendSlack with an empty url: %v", err)
	}
}

// TestSendSlack_FailureStatusIsAnErrorWithoutTheURL checks that a
// non-2xx response is an error, and that the error string never holds
// the webhook url: the url is a secret and the error can reach a log
// file on disk.
func TestSendSlack_FailureStatusIsAnErrorWithoutTheURL(t *testing.T) {
	var (
		srv *httptest.Server
		err error
	)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err = SendSlack(context.Background(), srv.URL, "a message"); err == nil {
		t.Fatal("SendSlack with a 500 response: want an error, got nil")
	}
	if strings.Contains(err.Error(), srv.URL) {
		t.Errorf("error %q holds the url %q", err.Error(), srv.URL)
	}
}

// TestSendSlack_DialFailureIsAnErrorWithoutTheURL checks the case that
// is not a status code at all: a connection net/http never completes.
// http.Client.Do wraps that as a *url.Error, whose own Error() text
// holds the request URL verbatim, so this is the case that proved the
// non-2xx redaction alone was not enough.
func TestSendSlack_DialFailureIsAnErrorWithoutTheURL(t *testing.T) {
	var (
		url = "http://127.0.0.1:1/a-secret-path-in-the-url"
		err error
	)
	if err = SendSlack(context.Background(), url, "a message"); err == nil {
		t.Fatal("SendSlack against a closed port: want an error, got nil")
	}
	if strings.Contains(err.Error(), url) {
		t.Errorf("error %q holds the url %q", err.Error(), url)
	}
	if strings.Contains(err.Error(), "a-secret-path-in-the-url") {
		t.Errorf("error %q holds the url's secret path", err.Error())
	}
}
