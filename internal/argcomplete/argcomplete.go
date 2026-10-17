// Package argcomplete completes the positional argument of a subcommand.
//
// opts v1.8.0 completes every positional argument as a file name. It reads
// the Completer of a flag, but not the Completer of an argument. So main
// calls Run before opts parses the command line, and Run answers the shell
// for the subcommands that it has a Completer for. Every other completion
// goes on to opts.
//
// The shell protocol is the one that `repo-tools --install` sets up: bash
// `complete -C`, which zsh runs through bashcompinit. The shell sets
// COMP_LINE and COMP_POINT, runs the binary, and reads one candidate from
// each line of stdout.
package argcomplete

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/jpillora/opts"
)

// Run answers a completion request when COMP_LINE asks for the first
// argument of a subcommand in completers. The key of completers is the
// path of the subcommand, for example "tmux attach". Run returns true when
// it has answered, and main then exits 0.
func Run(completers map[string]opts.Completer) bool {
	var (
		line  = os.Getenv("COMP_LINE")
		point int
		found []string
		ok    bool
		err   error
	)
	if line == "" {
		return false
	}
	if point, err = strconv.Atoi(os.Getenv("COMP_POINT")); err == nil && point >= 0 && point < len(line) {
		line = line[:point]
	}
	if found, ok = Complete(line, completers); !ok {
		return false
	}
	for _, f := range found {
		fmt.Println(f)
	}
	return true
}

// Complete returns the candidates for the last word of line, and whether a
// completer in completers owns that word. A completer owns the word when
// the words before it, after the program name, are its path. Only the
// candidates that start with the word are returned.
func Complete(line string, completers map[string]opts.Completer) ([]string, bool) {
	var (
		words = strings.Fields(line)
		last  string
		c     opts.Completer
		found []string
		ok    bool
	)
	// A line that ends in a space starts a new, empty word.
	if len(line) > 0 && unicode.IsSpace(rune(line[len(line)-1])) {
		words = append(words, "")
	}
	if len(words) < 2 {
		return nil, false
	}
	last = words[len(words)-1]
	if c, ok = completers[strings.Join(words[1:len(words)-1], " ")]; !ok {
		return nil, false
	}
	for _, f := range c.Complete(last) {
		if strings.HasPrefix(f, last) {
			found = append(found, f)
		}
	}
	return found, true
}
