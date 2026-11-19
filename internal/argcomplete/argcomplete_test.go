package argcomplete

import (
	"slices"
	"testing"

	"github.com/jpillora/opts"
)

// names is a Completer that offers a fixed list, so the test needs no tmux.
type names []string

func (n names) Complete(string) []string {
	return n
}

// positional is an ArgsCompleter that offers one fixed list per argument
// position, so the test needs no repository.
type positional [][]string

func (p positional) Complete(last string) []string {
	return p.CompleteArgs(nil, last)
}

func (p positional) CompleteArgs(prior []string, last string) []string {
	if len(prior) >= len(p) {
		return nil
	}
	return p[len(prior)]
}

func TestComplete(t *testing.T) {
	var (
		completers = map[string]opts.Completer{
			"tmux attach": names{
				"main",
				"main-review",
				"notes",
			},
			"features attach": positional{
				{
					"axis",
					"axle",
				},
				{
					"beta",
					"gamma",
				},
			},
		}
		cases = []struct {
			line string
			want []string
			ok   bool
		}{
			{
				line: "repo-tools tmux attach ",
				want: []string{
					"main",
					"main-review",
					"notes",
				},
				ok: true,
			},
			{
				line: "repo-tools tmux attach ma",
				want: []string{
					"main",
					"main-review",
				},
				ok: true,
			},
			{
				line: "repo-tools   tmux  attach  n",
				want: []string{
					"notes",
				},
				ok: true,
			},
			{
				line: "repo-tools tmux attach x",
				want: nil,
				ok:   true,
			},
			{
				line: "repo-tools tmux att",
				want: nil,
				ok:   false,
			},
			{
				line: "repo-tools tmux attach main ",
				want: nil,
				ok:   false,
			},
			{
				line: "repo-tools features attach ax",
				want: []string{
					"axis",
					"axle",
				},
				ok: true,
			},
			{
				line: "repo-tools features attach axis ",
				want: []string{
					"beta",
					"gamma",
				},
				ok: true,
			},
			{
				line: "repo-tools features attach --dry-run axis g",
				want: []string{
					"gamma",
				},
				ok: true,
			},
			{
				line: "repo-tools features attach axis beta ",
				want: nil,
				ok:   true,
			},
			{
				line: "repo-tools features attach --d",
				want: nil,
				ok:   false,
			},
			{
				line: "repo-tools md-table-fmt ",
				want: nil,
				ok:   false,
			},
			{
				line: "repo-tools",
				want: nil,
				ok:   false,
			},
		}
	)
	for _, c := range cases {
		t.Run(c.line, func(t *testing.T) {
			var (
				got []string
				ok  bool
			)
			if got, ok = Complete(c.line, completers); ok != c.ok || !slices.Equal(got, c.want) {
				t.Fatalf("Complete(%q) = %q, %v; want %q, %v", c.line, got, ok, c.want, c.ok)
			}
		})
	}
}
