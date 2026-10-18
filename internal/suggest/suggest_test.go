// ==== ACME Inc. portfolio ===============================================
// Project: content_management_system | File: internal/suggest/suggest_test.go
// About: Tests for package suggest.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

package suggest

import "testing"

func TestEditDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"pricing", "pricing", 0},
		{"kitten", "sitting", 3},
		{"flaw", "lawn", 2},
		{"about", "abuot", 2},
		{"café", "cafe", 1}, // one rune apart, two bytes apart
		{"日本語", "日本", 1},
	}
	for _, c := range cases {
		if got := EditDistance(c.a, c.b); got != c.want {
			t.Errorf("EditDistance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := EditDistance(c.b, c.a); got != c.want {
			t.Errorf("EditDistance(%q, %q) = %d, want %d (not symmetric)", c.b, c.a, got, c.want)
		}
	}
}

// slugs is a fixture of ACME Inc. CMS post and page slugs.
var slugs = []string{
	"about",
	"blog",
	"careers",
	"contact",
	"pricing",
	"q3-revenue-results",
	"spring-product-launch",
}

func TestSuggest(t *testing.T) {
	cases := []struct {
		word   string
		want   string
		wantOK bool
	}{
		{"pricng", "pricing", true},                       // one deletion
		{"PRICNG", "pricing", true},                       // case is ignored
		{"  pricng ", "pricing", true},                    // so is surrounding space
		{"abuot", "about", true},                          // a swap is two edits
		{"carers", "careers", true},                       // one insertion
		{"blg", "blog", true},                             // short word, one edit
		{"bg", "", false},                                 // short word, two edits is too far
		{"spring-product", "spring-product-launch", true}, // truncated slug
		{"q3-revenue-result", "q3-revenue-results", true},
		{"pricing", "", false}, // exact match is not a suggestion
		{"Pricing", "", false}, // exact match ignoring case
		{"xyz", "", false},     // nothing close
		{"newsletter", "", false},
		{"", "", false},
		{"   ", "", false},
	}
	for _, c := range cases {
		got, ok := Suggest(c.word, slugs)
		if got != c.want || ok != c.wantOK {
			t.Errorf("Suggest(%q) = (%q, %v), want (%q, %v)", c.word, got, ok, c.want, c.wantOK)
		}
	}
}

func TestSuggestReturnsTheEntryAsWritten(t *testing.T) {
	got, ok := Suggest("carers", []string{"Careers"})
	if !ok || got != "Careers" {
		t.Errorf("Suggest = (%q, %v), want (\"Careers\", true)", got, ok)
	}
}

func TestSuggestBreaksTiesAlphabetically(t *testing.T) {
	got, ok := Suggest("plog", []string{"slog", "blog"})
	if !ok || got != "blog" {
		t.Errorf("Suggest = (%q, %v), want (\"blog\", true)", got, ok)
	}
}

func TestSuggestEmptyVocabulary(t *testing.T) {
	if got, ok := Suggest("pricing", nil); ok || got != "" {
		t.Errorf("Suggest with no vocab = (%q, %v), want (\"\", false)", got, ok)
	}
}

// The short-word limits count runes, not bytes.
func TestSuggestCountsRunes(t *testing.T) {
	// "naï" is 3 runes (4 bytes), so only 1 edit is allowed; "maïs" is 2 away.
	if got, ok := Suggest("naï", []string{"maïs"}); ok {
		t.Errorf("Suggest(naï) = %q; a 3-rune word must stay within 1 edit", got)
	}
	// "日本" is 2 runes (6 bytes), too short for a prefix match.
	if got, ok := Suggest("日本", []string{"日本語ニュース"}); ok {
		t.Errorf("Suggest(日本) = %q; a 2-rune word must not prefix-match", got)
	}
	// A 3-rune prefix does match.
	if got, ok := Suggest("日本語", []string{"日本語ニュース"}); !ok || got != "日本語ニュース" {
		t.Errorf("Suggest(日本語) = (%q, %v), want a prefix match", got, ok)
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=dd50cdec4d2705d44a687ddd33df54cd5c4b39c6a9ae0b3c2e0c5d0836513144 ====
