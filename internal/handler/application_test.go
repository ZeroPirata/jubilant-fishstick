package handler

import (
	"testing"
	"time"
)

func TestParseApplicationDate(t *testing.T) {
	// 4/11 às 01h em Brasília (ainda 3/11 é "ontem").
	now := time.Date(2026, 11, 4, 4, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		ok   bool
		want time.Time
	}{
		{"", true, now},
		{"2026-11-04", true, now},
		{"2026-11-03", true, time.Date(2026, 11, 3, 12, 0, 0, 0, brt)},
		{"2026-11-05", false, time.Time{}},
		{"2020-01-01", false, time.Time{}},
		{"04/11/2026", false, time.Time{}},
	}
	for _, c := range cases {
		got, ok := parseApplicationDate(c.in, now)
		if ok != c.ok || (ok && !got.Equal(c.want)) {
			t.Errorf("parseApplicationDate(%q) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}
