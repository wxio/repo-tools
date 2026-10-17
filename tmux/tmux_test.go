package tmux

import (
	"slices"
	"testing"
)

func TestMatching(t *testing.T) {
	var (
		names = []string{
			"main",
			"main-review",
			"notes",
			"two words",
		}
		cases = []struct {
			prefix string
			want   []string
		}{
			{
				prefix: "",
				want:   names,
			},
			{
				prefix: "main",
				want: []string{
					"main",
					"main-review",
				},
			},
			{
				prefix: "two ",
				want: []string{
					"two words",
				},
			},
			{
				prefix: "x",
				want:   nil,
			},
		}
	)
	for _, c := range cases {
		t.Run(c.prefix, func(t *testing.T) {
			var got = matching(names, c.prefix)
			if !slices.Equal(got, c.want) {
				t.Fatalf("matching(%q) = %q, want %q", c.prefix, got, c.want)
			}
		})
	}
}
