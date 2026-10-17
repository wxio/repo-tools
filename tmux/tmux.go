// Package tmux is the tmux subcommand. Its own subcommands make a tmux
// session, attach to one and kill one:
//
//	repo-tools tmux new [name]
//	repo-tools tmux attach [session]
//	repo-tools tmux kill-session <session>
//
// The point is the tab completion. `attach <tab>` lists the sessions of the
// running tmux server, so a session is picked by name rather than typed.
// Install the completion once with `repo-tools --install`.
//
// Each subcommand runs tmux with the terminal of this process. tmux reports
// its own errors, so a failure of tmux becomes the exit code of repo-tools
// with no message added.
package tmux

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/wxio/repo-tools/internal/exitcode"
)

// Session is the argument of attach and kill-session: the name of a tmux
// session. Its Complete lists the sessions. opts v1.8.0 completes every
// argument as a file name and does not call it, so main answers the shell
// first; see internal/argcomplete.
//
// It is a string type, not a struct. opts takes a slice of strings as an
// optional argument, and refuses a slice of structs.
type Session string

// Complete lists the sessions whose name starts with what the user has
// typed. It lists nothing when no tmux server runs, as the shell then has
// nothing to offer.
func (s *Session) Complete(user string) []string {
	var (
		names []string
		err   error
	)
	if names, err = sessions(); err != nil {
		return nil
	}
	return matching(names, user)
}

// sessions asks the tmux server for the names of its sessions, one to a
// line. A name can hold a space, so the split is on the line.
func sessions() ([]string, error) {
	var (
		cmd = exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
		out []byte
		err error
	)
	if out, err = cmd.Output(); err != nil {
		return nil, err
	}
	if out = bytes.TrimSpace(out); len(out) == 0 {
		return nil, nil
	}
	return strings.Split(string(out), "\n"), nil
}

// matching keeps the names that start with prefix. opts asks a completer
// for results that start with the input of the user.
func matching(names []string, prefix string) []string {
	var found []string
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			found = append(found, n)
		}
	}
	return found
}

// run runs tmux with the terminal of this process. A non-zero exit of tmux
// comes back as an exitcode.Code, because tmux has already said why.
func run(args ...string) error {
	var (
		cmd  = exec.Command("tmux", args...)
		exit *exec.ExitError
		err  error
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); errors.As(err, &exit) {
		return exitcode.Code(exit.ExitCode())
	}
	return err
}

// output runs tmux and returns what it prints, with the trailing newline
// cut. tmux writes its errors to the terminal, as it does in run.
func output(args ...string) (string, error) {
	var (
		cmd  = exec.Command("tmux", args...)
		exit *exec.ExitError
		out  []byte
		err  error
	)
	cmd.Stderr = os.Stderr
	if out, err = cmd.Output(); errors.As(err, &exit) {
		return "", exitcode.Code(exit.ExitCode())
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
