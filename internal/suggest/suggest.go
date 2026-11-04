// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/suggest/suggest.go
// About: Package suggest offers a "did you mean" hint for a missed lookup.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package suggest offers a "did you mean" hint for a missed lookup.
//
// A typical miss is a 404 on an unknown post slug: in the CMS, store.Get
// calls Suggest with the published slugs, and the API and the
// missing-post page show the result. The package has two parts:
//
//   - EditDistance measures how far apart two strings are (Levenshtein).
//   - Suggest picks the one vocabulary entry close enough to offer.
//
// Lengths and edits are counted in runes, so accented and CJK text is
// treated fairly.
package suggest

import (
	"strings"
	"unicode/utf8"
)

// EditDistance returns the Levenshtein distance between a and b,
// counted in runes, so "café" and "cafe" are one edit apart.
//
// The Levenshtein distance is the fewest single-rune insertions,
// deletions and substitutions that turn a into b. A swap of two
// neighbours counts as two edits. The classic dynamic programme fills a
// table where cell (i, j) is the distance between the first i runes of a
// and the first j runes of b; each cell needs only the row above, so two
// rows are kept and swapped, using O(len(b)) memory and O(len(a)*len(b))
// time.
func EditDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	// row 0: turning "" into the first j runes of b takes j insertions
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		// column 0: turning i runes of a into "" takes i deletions
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			// insert b[j-1], delete a[i-1], or substitute (free when equal)
			cur[j] = min(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	// after the last swap the finished row is in prev
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
//
// Steps:
//  1. Normalize word (trim, lowercase); a blank word suggests nothing.
//  2. For each entry, normalized the same way, skip blanks and exact
//     matches, then pick the edit limit from the shorter side's length.
//  3. Over the limit, keep the entry only when word has 3 or more runes
//     and is a prefix of it, scored at the limit so a closer near-miss
//     still wins.
//  4. Keep the entry with the smallest distance, then the smallest
//     normalized spelling.
//
// The limits keep hints plausible: two edits on a three-letter word can
// turn it into almost any other short word.
func Suggest(word string, vocab []string) (string, bool) {
	m := strings.ToLower(strings.TrimSpace(word))
	if m == "" {
		return "", false
	}
	mLen := utf8.RuneCountInString(m)

	// bestD < 0 means nothing found yet; bestKey is the normalized form
	// used for the alphabetical tie-break
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

// ==== stamp v1 | author=kjFriday | hmac-sha256=c83c31071ceec39cbc6d9093818752f8f96f0471bb6c4b6ba184787ec49404c5 ====
