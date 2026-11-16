package notify

import (
	"reflect"
	"testing"
)

func TestTransports(t *testing.T) {
	var cases = []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "one name",
			in:   "slack",
			want: []string{"slack"},
		},
		{
			name: "two names",
			in:   "slack,telegram",
			want: []string{"slack", "telegram"},
		},
		{
			name: "whitespace around each name",
			in:   " slack , telegram ",
			want: []string{"slack", "telegram"},
		},
		{
			name: "empty string",
			in:   "",
			want: nil,
		},
		{
			name: "empty name between two commas is dropped",
			in:   "slack,,telegram",
			want: []string{"slack", "telegram"},
		},
		{
			name: "an unknown name is dropped, not refused",
			in:   "slack,carrier-pigeon",
			want: []string{"slack"},
		},
		{
			name: "only unknown names gives none",
			in:   "carrier-pigeon",
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got = Transports(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Transports(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}
