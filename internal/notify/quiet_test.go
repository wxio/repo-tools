package notify

import (
	"testing"
	"time"
)

func TestShouldSend(t *testing.T) {
	var cases = []struct {
		name       string
		elapsed    time.Duration
		minSeconds int
		haveMark   bool
		want       bool
	}{
		{
			name:       "short turn with a mark stays quiet",
			elapsed:    10 * time.Second,
			minSeconds: 60,
			haveMark:   true,
			want:       false,
		},
		{
			name:       "long turn with a mark sends",
			elapsed:    90 * time.Second,
			minSeconds: 60,
			haveMark:   true,
			want:       true,
		},
		{
			name:       "no mark at all sends",
			elapsed:    0,
			minSeconds: 60,
			haveMark:   false,
			want:       true,
		},
		{
			name:       "minSeconds of 0 always sends",
			elapsed:    1 * time.Second,
			minSeconds: 0,
			haveMark:   true,
			want:       true,
		},
		{
			name:       "negative minSeconds always sends",
			elapsed:    1 * time.Second,
			minSeconds: -1,
			haveMark:   true,
			want:       true,
		},
		{
			name:       "elapsed exactly at the threshold sends",
			elapsed:    60 * time.Second,
			minSeconds: 60,
			haveMark:   true,
			want:       true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got = ShouldSend(c.elapsed, c.minSeconds, c.haveMark)
			if got != c.want {
				t.Errorf("ShouldSend(%v, %d, %v) = %v, want %v", c.elapsed, c.minSeconds, c.haveMark, got, c.want)
			}
		})
	}
}
