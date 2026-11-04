// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/related/body.go
// About: BodyText turns a post's JSON block body into plain text for search and the related index.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// BodyText turns a post's JSON block body into plain text for search and the related index.
//
// A body is block JSON as written by an editor such as Editor.js or TipTap,
// not HTML. Editor.js stores {"blocks": [{"type": "paragraph", "data":
// {"text": "..."}}, ...]}; TipTap (built on ProseMirror) stores a tree of
// {"type": "doc", "content": [{"type": "text", "text": "..."}, ...]}.
// Rather than know each format, BodyText keeps every string that reads as
// text and skips the keys that hold layout, ids or links. store.New calls it
// once per post; the related index, search and excerpts are built from the
// result. The rest of this file is its helpers.

package related

import (
	"encoding/json"
	"html"
	"sort"
	"strings"
)

// structuralKeys hold layout or link data in block JSON, not readable
// text. "link" is the Editor.js link-card URL; "source", "embed" and
// "service" belong to its embed block.
var structuralKeys = map[string]bool{
	"type": true, "id": true, "version": true, "style": true,
	"alignment": true, "url": true, "src": true, "href": true,
	"target": true, "rel": true, "class": true,
	"link": true, "source": true, "embed": true, "service": true,
}

// isBareLink reports whether s is a single http, https or mailto link
// with nothing else around it.
func isBareLink(s string) bool {
	f := strings.Fields(s)
	if len(f) != 1 {
		return false
	}
	l := strings.ToLower(f[0])
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") ||
		strings.HasPrefix(l, "mailto:")
}

// BodyText flattens a JSON block body (Editor.js, TipTap/ProseMirror or
// similar) into plain text for indexing. It keeps every string value
// except those under structural keys such as "type" or "url" and values
// that are a bare link, drops inline HTML tags and decodes entities.
// Arrays keep their order; object keys are visited alphabetically. A body
// that is not valid JSON is treated as plain text. The result is meant
// for indexing, not display.
//
// Steps:
//  1. Decode the body into generic values (maps, slices, strings, numbers).
//  2. Walk them with collectText, gathering the readable strings in order.
//  3. Join the pieces with single spaces.
func BodyText(body []byte) string {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return plainText(string(body))
	}
	var parts []string
	collectText(v, "", &parts)
	return strings.Join(parts, " ")
}

// collectText appends the readable strings under v to out, depth first.
// key is the nearest enclosing object key, so array elements are judged
// by their parent's key. Numbers, booleans and null are skipped.
func collectText(v any, key string, out *[]string) {
	switch x := v.(type) {
	case string:
		if structuralKeys[key] || isBareLink(x) {
			return
		}
		if s := plainText(x); s != "" {
			*out = append(*out, s)
		}
	case []any:
		for _, e := range x {
			collectText(e, key, out)
		}
	case map[string]any:
		// Go map order is random; sorting makes the text, and so the index, the same every run
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			collectText(x[k], k, out)
		}
	}
}

// plainText replaces anything shaped like an HTML tag ("<b>", "</a>",
// "<br/>") with a space, decodes entities such as "&amp;" and collapses
// white space. A "<" not followed by a letter, "/" or "!" is kept as text.
func plainText(s string) string {
	var b strings.Builder
	// byte by byte is safe for UTF-8: '<' and '>' never occur inside a multi-byte character
	for i := 0; i < len(s); i++ {
		if s[i] == '<' && i+1 < len(s) && isTagStart(s[i+1]) {
			// a tag with no closing '>' is kept as text
			if end := strings.IndexByte(s[i:], '>'); end >= 0 {
				b.WriteByte(' ')
				i += end
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return strings.Join(strings.Fields(html.UnescapeString(b.String())), " ")
}

// isTagStart reports whether c, right after a "<", starts a tag: a
// letter, "/" (a closing tag) or "!" (a comment or doctype).
func isTagStart(c byte) bool {
	return c == '/' || c == '!' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=24eea245b3c27914aeddcb766ca84669f85a438c89895463b6af6de8f3eb29ac ====
