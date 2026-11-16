package notify

import "strings"

// transportNames are the chat transports this package knows how to send
// to. Transports drops a name outside this set in silence, so a typo in
// --to or NOTIFY_TO neither errors nor reaches a transport that does not
// exist.
var transportNames = map[string]bool{
	"slack":    true,
	"telegram": true,
}

// Transports splits s on commas into the transports it names. --to and
// NOTIFY_TO take the same form, `slack,telegram`, so they share this one
// parser. Each name is trimmed of the whitespace around it; an empty name
// is dropped, and so is a name this package does not know, both in
// silence.
func Transports(s string) []string {
	var (
		fields = strings.Split(s, ",")
		out    []string
		name   string
	)
	for _, f := range fields {
		name = strings.TrimSpace(f)
		if name == "" || !transportNames[name] {
			continue
		}
		out = append(out, name)
	}
	return out
}
