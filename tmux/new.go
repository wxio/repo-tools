package tmux

import "os"

// NewCmd is the command line of tmux new. The name is a plain string: the
// name of a new session is not one of the names that exist, so it has no
// completion. It is a list of at most one, because opts makes a single
// argument required, and tmux picks a number when no name is given.
type NewCmd struct {
	Names []string `opts:"mode=arg,name=name,max=1" help:"the name of the new session (default: tmux picks a number)"`
}

// NewNewCmd returns the command line of tmux new.
func NewNewCmd() *NewCmd {
	return &NewCmd{}
}

// Run makes the session and attaches the terminal to it. Inside tmux, a
// new attached session would nest one client in another, so it makes the
// session detached and switches the client to it.
func (c *NewCmd) Run() error {
	var (
		args = []string{"new-session"}
		name string
		err  error
	)
	for _, n := range c.Names {
		args = append(args, "-s", n)
	}
	if os.Getenv("TMUX") == "" {
		return run(args...)
	}
	// -P -F prints the name of the session, which is the only way to know
	// it when tmux picks the name.
	args = append(args, "-d", "-P", "-F", "#{session_name}")
	if name, err = output(args...); err != nil {
		return err
	}
	return run("switch-client", "-t", name)
}
