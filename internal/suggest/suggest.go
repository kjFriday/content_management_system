// ==== ACME Inc. portfolio ===============================================
// Project: content_management_system | File: internal/suggest/suggest.go
// About: Package suggest offers a "did you mean" hint for a missed lookup, such as a 404 on an...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package suggest offers a "did you mean" hint for a missed lookup, such
// as a 404 on an unknown post slug.
package suggest

import (
	"strings"
	"unicode/utf8"
)

// EditDistance returns the Levenshtein distance between a and b,
// counted in runes, so "café" and "cafe" are one edit apart.
func EditDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// Suggest returns the vocab entry closest to word, or false if nothing
// is close enough. Matching ignores case and surrounding space; the
// entry is returned as it appears in vocab.
//
// An entry is close enough when it is within 2 edits of word, or within
// 1 edit when either side is 3 runes or shorter. An entry that starts
// with word (3 runes or more) also counts, ranked at that edit limit, so
// a truncated slug still finds its post. An exact match is not a
// suggestion. Ties go to the alphabetically first entry.
func Suggest(word string, vocab []string) (string, bool) {
	m := strings.ToLower(strings.TrimSpace(word))
	if m == "" {
		return "", false
	}
	mLen := utf8.RuneCountInString(m)

	best, bestKey, bestD := "", "", -1
	for _, entry := range vocab {
		w := strings.ToLower(strings.TrimSpace(entry))
		if w == "" || w == m {
			continue
		}
		limit := 2
		if mLen <= 3 || utf8.RuneCountInString(w) <= 3 {
			limit = 1
		}
		d := EditDistance(m, w)
		if d > limit {
			if mLen < 3 || !strings.HasPrefix(w, m) {
				continue
			}
			d = limit
		}
		if bestD < 0 || d < bestD || (d == bestD && w < bestKey) {
			best, bestKey, bestD = entry, w, d
		}
	}
	return best, bestD >= 0
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=4ea7637eafed5c86305cad61e10d5564d787a7e4756fdd45271b7148d61ec77f ====
