package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// slackBody is the one field a Slack incoming webhook reads from the
// POST body. encoding/json escapes a quote and a newline in text on its
// own, so a message built from a turn's own words round-trips whatever
// punctuation it holds.
type slackBody struct {
	Text string `json:"text"`
}

// SendSlack posts text to the Slack incoming webhook at url. An empty
// url returns nil and sends nothing: PRD section 4.3 calls a transport
// with no key configured skipped in silence, not an error, so a setup
// that only has one of the two transports still works. ctx carries the
// caller's --timeout deadline.
//
// Every error SendSlack returns is redacted of url first, with
// redactURL. A non-2xx response is the case the caller asks about by
// name, but it is not the only one: net/http wraps url straight into a
// *url.Error for a dial failure, a timeout or a malformed address, and
// that error's text would otherwise carry the webhook's secret path
// into whatever the caller logs it to (PRD section 4.1: the URL is a
// password).
func SendSlack(ctx context.Context, url, text string) error {
	var (
		body []byte
		err  error
		req  *http.Request
		resp *http.Response
	)
	if url == "" {
		return nil
	}
	if body, err = json.Marshal(slackBody{
		Text: text,
	}); err != nil {
		return err
	}
	if req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body)); err != nil {
		return fmt.Errorf("notify: building the slack request: %s", redactURL(err, url))
	}
	req.Header.Set("Content-Type", "application/json")
	if resp, err = http.DefaultClient.Do(req); err != nil {
		return fmt.Errorf("notify: the slack request failed: %s", redactURL(err, url))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notify: slack webhook answered status %d", resp.StatusCode)
	}
	return nil
}

// redactURL returns err's message with every occurrence of url replaced
// by a fixed placeholder. net/http's own errors, such as the *url.Error
// http.Client.Do wraps a dial failure or a timeout in, hold the request
// URL verbatim in their own Error() text; this is the one place that
// text is built, so it is the one place that has to strip it back out
// before the message goes anywhere near a log file.
func redactURL(err error, url string) string {
	if url == "" {
		return err.Error()
	}
	return strings.ReplaceAll(err.Error(), url, "<redacted>")
}
