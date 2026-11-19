// Package argcomplete completes the positional arguments of a subcommand.
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

// An ArgsCompleter completes every positional argument of its subcommand,
// not only the first. Complete gives it the arguments already typed, with
// the flags dropped, and the word under the cursor. A plain Completer
// answers for the first argument only.
type ArgsCompleter interface {
	CompleteArgs(prior []string, last string) []string
}

// Run answers a completion request when COMP_LINE asks for an argument
// of a subcommand in completers. The key of completers is the path of
// the subcommand, for example "tmux attach". Run returns true when it
// has answered, and main then exits 0.
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
// its path starts the words after the program name; the words between the
// path and the last word are the prior arguments. An ArgsCompleter gets
// the prior arguments, with the flags dropped. A plain Completer owns the
// first argument only. Only the candidates that start with the word are
// returned. A word that starts with a dash is a flag, which opts
// completes.
func Complete(line string, completers map[string]opts.Completer) ([]string, bool) {
	var (
		words = strings.Fields(line)
		last  string
	)
	// A line that ends in a space starts a new, empty word.
	if len(line) > 0 && unicode.IsSpace(rune(line[len(line)-1])) {
		words = append(words, "")
	}
	if len(words) < 2 {
		return nil, false
	}
	last = words[len(words)-1]
	if strings.HasPrefix(last, "-") {
		return nil, false
	}
	// The longest path wins: on "features attach 064 <tab>" the key
	// "features attach" owns the word, and 064 is a prior argument.
	for end := len(words) - 1; end >= 2; end-- {
		var (
			c       opts.Completer
			ac      ArgsCompleter
			prior   []string
			results []string
			found   []string
			ok      bool
		)
		if c, ok = completers[strings.Join(words[1:end], " ")]; !ok {
			continue
		}
		for _, w := range words[end : len(words)-1] {
			if !strings.HasPrefix(w, "-") {
				prior = append(prior, w)
			}
		}
		if ac, ok = c.(ArgsCompleter); ok {
			results = ac.CompleteArgs(prior, last)
		} else if len(prior) == 0 {
			results = c.Complete(last)
		} else {
			return nil, false
		}
		for _, f := range results {
			if strings.HasPrefix(f, last) {
				found = append(found, f)
			}
		}
		return found, true
	}
	return nil, false
}
