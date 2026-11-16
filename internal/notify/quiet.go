package notify

import "time"

// ShouldSend decides whether a turn is worth a message, given how long it
// took and whether a start mark was found for it.
//
// A missing mark (haveMark == false) always sends. The session's
// UserPromptSubmit hook writes the mark at the start of a turn; a session
// that began before that hook was installed, or whose mark was swept or
// lost, has no measured elapsed time at all. Treating that as a short
// turn would drop the notification in silence for exactly the session
// that most needs it, so a missing mark is always treated as a long one.
//
// minSeconds of 0 or less turns the quiet rule off: every turn sends.
// Otherwise a turn sends when elapsed is at or above minSeconds, and
// stays quiet when it is shorter.
func ShouldSend(elapsed time.Duration, minSeconds int, haveMark bool) bool {
	if !haveMark {
		return true
	}
	if minSeconds <= 0 {
		return true
	}
	return elapsed >= time.Duration(minSeconds)*time.Second
}
