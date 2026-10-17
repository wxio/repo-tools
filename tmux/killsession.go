package tmux

// KillSessionCmd is the command line of tmux kill-session. The session is
// required: without a target, tmux kills the most recent session, which is
// not a guess to make for the user.
type KillSessionCmd struct {
	Session Session `opts:"mode=arg" help:"the session to kill"`
}

// NewKillSessionCmd returns the command line of tmux kill-session.
func NewKillSessionCmd() *KillSessionCmd {
	return &KillSessionCmd{}
}

// Run kills the session.
func (c *KillSessionCmd) Run() error {
	return run("kill-session", "-t", string(c.Session))
}
