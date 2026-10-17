package tmux

import "os"

// AttachCmd is the command line of tmux attach. The session is a list of
// at most one, because opts makes a single argument required, and attach
// without a session attaches to the most recent one.
type AttachCmd struct {
	Sessions []Session `opts:"mode=arg,name=session,max=1" help:"the session to attach to (default: the most recent)"`
}

// NewAttachCmd returns the command line of tmux attach.
func NewAttachCmd() *AttachCmd {
	return &AttachCmd{}
}

// Run attaches the terminal to the session. Inside tmux, attach would nest
// one client in another, so it switches the client to the session instead.
func (c *AttachCmd) Run() error {
	var args []string
	if os.Getenv("TMUX") != "" {
		args = []string{"switch-client"}
	} else {
		args = []string{"attach-session"}
	}
	for _, s := range c.Sessions {
		args = append(args, "-t", string(s))
	}
	return run(args...)
}
