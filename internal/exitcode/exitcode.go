// Package exitcode holds the error that sets the exit code of repo-tools.
//
// A subcommand returns a Code when the code means something to its caller,
// as the code of a prek hook does. The subcommand reports the problem
// itself, so main prints nothing more.
package exitcode

import "fmt"

// Code is an error that sets the exit code of the process.
type Code int

func (c Code) Error() string {
	return fmt.Sprintf("exit status %d", int(c))
}
