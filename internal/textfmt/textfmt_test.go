// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/textfmt/textfmt_test.go
// About: Tests for the string helpers.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Tests for the string helpers. Most cases mix ASCII with accented and CJK
// text, because every length here is counted in runes and a byte-counting
// bug would only show on multi-byte input.

package textfmt

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// TestTruncate pins the ellipsis rule, rune counting, the n <= 0 cases and
// that no result is ever longer than n runes.
func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hell…"},
		{"héllo wörld", 6, "héllo…"},
		{"日本語テキスト", 3, "日本…"},
		{"abc", 1, "…"},
		{"abc", 0, ""},
		{"abc", -1, ""},
		{"", 3, ""},
	}
	for _, c := range cases {
		got := Truncate(c.s, c.n)
		if got != c.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
		if c.n > 0 && utf8.RuneCountInString(got) > c.n {
			t.Errorf("Truncate(%q, %d) is %d runes long", c.s, c.n, utf8.RuneCountInString(got))
		}
	}
}

// TestPlural pins that only exactly 1 is singular, 0 and -1 included.
func TestPlural(t *testing.T) {
	for n, want := range map[int]string{0: "posts", 1: "post", 2: "posts", -1: "posts"} {
		if got := Plural(n, "post", "posts"); got != want {
			t.Errorf("Plural(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestShortDuration pins each unit pair, zero padding of the smaller unit,
// truncation instead of rounding, negatives, and that MinInt64 gets one
// minus sign rather than overflowing.
func TestShortDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{500 * time.Millisecond, "0s"},
		{45 * time.Second, "45s"},
		{59*time.Second + 999*time.Millisecond, "59s"}, // truncated, not rounded
		{3*time.Minute + 7*time.Second, "3m07s"},
		{2*time.Hour + 5*time.Minute + 59*time.Second, "2h05m"},
		{4*24*time.Hour + 3*time.Hour + 30*time.Minute, "4d03h"},
		{400 * 24 * time.Hour, "400d00h"},
		{-90 * time.Second, "-1m30s"},
		{-500 * time.Millisecond, "0s"},
	}
	for _, c := range cases {
		if got := ShortDuration(c.d); got != c.want {
			t.Errorf("ShortDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
	if got := ShortDuration(math.MinInt64); !strings.HasPrefix(got, "-") || strings.HasPrefix(got, "--") {
		t.Errorf("ShortDuration(MinInt64) = %q, want one leading minus", got)
	}
}

// TestOneLine pins that tabs, CRLF and blank lines collapse to single
// spaces and that blank input becomes "".
func TestOneLine(t *testing.T) {
	in := "  Quarterly\tresults\r\n\n  are   in  "
	if got, want := OneLine(in), "Quarterly results are in"; got != want {
		t.Errorf("OneLine = %q, want %q", got, want)
	}
	if got := OneLine(" \t\n "); got != "" {
		t.Errorf("OneLine(blank) = %q, want empty", got)
	}
}

// TestWrap pins each documented Wrap rule with one named case.
func TestWrap(t *testing.T) {
	cases := []struct {
		name  string
		s     string
		width int
		want  []string
	}{
		{"basic", "the quick brown fox jumps", 10, []string{"the quick", "brown fox", "jumps"}},
		{"exact fit", "aaaa bbbb", 9, []string{"aaaa bbbb"}},
		{"counts runes", "été été été", 7, []string{"été été", "été"}},
		{"long word kept whole", "a supercalifragilistic b", 5, []string{"a", "supercalifragilistic", "b"}},
		{"line breaks collapse", "line one\n\nline two", 80, []string{"line one line two"}},
		{"no limit", "one two three", 0, []string{"one two three"}},
		{"empty", "", 10, nil},
		{"blank", " \n\t ", 10, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Wrap(c.s, c.width)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Wrap(%q, %d) = %q, want %q", c.s, c.width, got, c.want)
			}
		})
	}
}

// Every line fits the width unless it is a single word that cannot.
func TestWrapRespectsWidth(t *testing.T) {
	text := "Generic Company Inc. publishes product news, release notes and quarterly results on this blog every week."
	for width := 1; width <= 40; width++ {
		for _, line := range Wrap(text, width) {
			if utf8.RuneCountInString(line) > width && strings.Contains(line, " ") {
				t.Errorf("width %d: line %q is too long", width, line)
			}
		}
	}
}

// TestPadRight pins padding by runes, that long strings are not cut, and
// the Truncate-then-pad pairing for a fixed-width column.
func TestPadRight(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"abc", 5, "abc  "},
		{"été", 5, "été  "},
		{"abcdef", 3, "abcdef"},
		{"abc", 0, "abc"},
		{"abc", -2, "abc"},
		{"", 2, "  "},
	}
	for _, c := range cases {
		if got := PadRight(c.s, c.n); got != c.want {
			t.Errorf("PadRight(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
	if got := PadRight(Truncate("Quarterly results", 8), 8); got != "Quarter…" {
		t.Errorf("fixed-width column = %q", got)
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=b77cbf378dd4096bcc7a286f71131be4d9ddeabcfae85a4faa60ea077a1b3904 ====
