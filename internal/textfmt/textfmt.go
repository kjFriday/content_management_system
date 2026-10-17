// Package textfmt holds small string helpers for listings, logs and the
// admin views. Lengths are counted in runes (code points), not bytes and
// not display columns: a wide CJK character still counts as one, and an
// emoji built from several code points counts as several.
package textfmt

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

// Truncate shortens s to at most n runes. When it cuts, the last rune
// kept is "…" so the reader can see text is missing. n <= 0 returns "".
func Truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

// Plural returns one when n is 1 and many otherwise (including 0).
func Plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ShortDuration formats d as its largest whole unit and the next one
// down: "45s", "3m07s", "2h05m", "4d03h". Units are truncated, never
// rounded up. Negative durations get a leading "-"; anything under one
// second, either sign, is "0s".
func ShortDuration(d time.Duration) string {
	sign := ""
	if d < 0 {
		sign = "-"
		if d == math.MinInt64 {
			d = math.MaxInt64
		} else {
			d = -d
		}
	}
	if d < time.Second {
		return "0s"
	}
	switch {
	case d >= 24*time.Hour:
		h := int64(d / time.Hour)
		return fmt.Sprintf("%s%dd%02dh", sign, h/24, h%24)
	case d >= time.Hour:
		return fmt.Sprintf("%s%dh%02dm", sign, int64(d/time.Hour), int64(d/time.Minute)%60)
	case d >= time.Minute:
		return fmt.Sprintf("%s%dm%02ds", sign, int64(d/time.Minute), int64(d/time.Second)%60)
	default:
		return fmt.Sprintf("%s%ds", sign, int64(d/time.Second))
	}
}

// OneLine collapses every run of white space (tabs and line breaks
// included) to one space and trims both ends, so s fits in a single log
// field or table cell.
func OneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Wrap breaks s into lines of at most width runes, splitting only at
// white space.
//
//   - Runs of white space, line breaks included, collapse to one space,
//     so existing paragraphs are not preserved.
//   - A word longer than width is never split. It goes on a line of its
//     own, and that line is longer than width.
//   - No line is indented; prefix the lines yourself if you need to.
//   - width <= 0 means no limit: the words come back as one line.
//   - Empty or all-space input returns nil.
func Wrap(s string, width int) []string {
	var lines []string
	var cur strings.Builder
	curLen := 0
	for _, w := range strings.Fields(s) {
		wl := utf8.RuneCountInString(w)
		switch {
		case curLen == 0:
		case width <= 0 || curLen+1+wl <= width:
			cur.WriteByte(' ')
			curLen++
		default:
			lines = append(lines, cur.String())
			cur.Reset()
			curLen = 0
		}
		cur.WriteString(w)
		curLen += wl
	}
	if curLen > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

// PadRight appends spaces until s is n runes long. A string that is
// already n runes or longer comes back unchanged; it is not cut, so pair
// it with Truncate for a fixed-width column.
func PadRight(s string, n int) string {
	if gap := n - utf8.RuneCountInString(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}
