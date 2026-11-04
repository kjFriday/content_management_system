// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/related/related.go
// About: Package related ranks posts by content similarity, for a "related posts" list.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package related ranks posts by content similarity, for a "related
// posts" list. It is a small TF-IDF and cosine-similarity engine with no
// dependencies outside the standard library.
//
// The IDF is the smoothed form log((1+N)/(1+df)) + 1, the default in
// scikit-learn's TfidfVectorizer, so scores can be cross-checked there.
// Every result lists the terms that contributed most to its score.
//
// # TF-IDF and cosine similarity in brief
//
// Each document becomes a vector with one dimension per word (a "term").
// The weight of a term in a document is TF times IDF:
//
//   - TF (term frequency) is how often the term occurs in the document,
//     divided by the document's token count, so long posts are not
//     favoured just for being long.
//   - IDF (inverse document frequency) is large for a term few documents
//     use and small for one most documents use. With N documents and df
//     of them containing the term, the smoothed IDF here is
//     log((1+N)/(1+df)) + 1. The +1 inside acts as if one extra document
//     held every term, which damps the jump for very rare terms; the +1
//     outside keeps a term found everywhere at weight 1 instead of 0.
//
// Every vector is then scaled to length 1 (L2 normalization: divide by
// the square root of the sum of squared weights). The cosine of the angle
// between two such vectors is just their dot product: 1 for the same mix
// of terms, 0 for no term in common. Weights are never negative, so
// scores stay in [0, 1].
//
// # Flow
//
//  1. A caller (package store) builds one Doc per published post and
//     calls New.
//  2. New tokenizes every Doc, counts document frequencies, computes the
//     IDF table and one normalized vector per Doc.
//  3. Related scores one Doc's vector against all others; RelatedToSet
//     first averages several vectors (a centroid) and scores that.
//  4. rank keeps positive scores, sorts them and attaches the shared
//     terms that explain each score.
//
// Types: Doc (input), Index (the fitted corpus), Recommendation (output)
// and the internal sparse vector. body.go adds BodyText, which turns a
// post's JSON body into the text a Doc is built from.
//
// Start reading at New.
package related

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// stopWords are common function words that carry no topical signal. The
// list is short on purpose: on a small corpus an aggressive stop list
// removes real terms.
var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "its": true,
	"that": true, "this": true, "from": true, "into": true, "out": true,
	"you": true, "your": true, "own": true, "not": true, "any": true,
	"one": true, "two": true, "all": true, "but": true, "then": true,
	"has": true, "have": true, "are": true, "was": true, "were": true,
	"on": true, "in": true, "at": true, "by": true, "to": true,
	"of": true, "it": true, "is": true, "as": true, "an": true,
	"or": true, "no": true, "so": true, "up": true, "if": true,
}

// tokenize lowercases a document and splits it on anything that is not a
// letter or digit, dropping stop words and one-rune tokens. Digits are
// kept: years and version numbers are useful features. A combining mark
// (an accent, or a vowel sign in scripts such as Devanagari) stays inside
// the word it follows. Text is expected in NFC: the standard library has
// no normalizer, so a decomposed (NFD) spelling is a different term.
//
// Steps:
//  1. Lowercase the whole document once.
//  2. Append letters and digits to the current token; append a combining
//     mark only when a token is already open, so a stray mark at a word
//     start never begins a token.
//  3. On any other rune, flush: keep the token if it has two or more
//     runes and is not a stop word.
func tokenize(doc string) []string {
	var out []string
	var b strings.Builder
	// flush ends the current token and keeps it if it carries signal.
	flush := func() {
		if b.Len() == 0 {
			return
		}
		t := b.String()
		b.Reset()
		if utf8.RuneCountInString(t) > 1 && !stopWords[t] {
			out = append(out, t)
		}
	}
	for _, r := range strings.ToLower(doc) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || (b.Len() > 0 && unicode.IsMark(r)) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

// Doc is one item in the corpus. Name identifies it (a post slug) and
// Text is what it is compared by (title, tags, body text). Name is also
// what results report and what Related and RelatedToSet look up.
type Doc struct {
	Name string
	Text string
}

// vector is a sparse, L2-normalized TF-IDF vector: term -> weight. A
// term absent from the map has weight zero, so only the terms a document
// uses are stored.
type vector map[string]float64

// Index is a fitted corpus. Build it with New. It is read-only after
// that and safe for concurrent use.
type Index struct {
	docs   []Doc              // a private copy of the input, in input order
	byName map[string]int     // Name -> position in docs (the first for a repeated Name)
	idf    map[string]float64 // term -> smoothed IDF, for every term in the corpus
	vecs   []vector           // one normalized TF-IDF vector per doc, same order as docs
}

// New fits an index over docs: document frequencies, smoothed IDF,
// TF-IDF weights and L2 normalization. Name is tokenized along with
// Text, so the words in a slug count toward its content. If two docs
// share a Name, lookups use the first.
//
// Steps:
//  1. Copy docs, so a caller that later changes its slice cannot change
//     the index.
//  2. Tokenize each doc and count, per term, how many docs contain it
//     (document frequency: a term counts once per doc however often it
//     repeats there).
//  3. Turn each document frequency into a smoothed IDF.
//  4. Weigh each doc's tokens into a normalized TF-IDF vector.
//
// An empty docs gives an empty index whose lookups all return nil.
func New(docs []Doc) *Index {
	docs = append([]Doc(nil), docs...)
	ix := &Index{docs: docs, byName: map[string]int{}, idf: map[string]float64{}}
	if len(docs) == 0 {
		return ix
	}
	n := float64(len(docs))

	toks := make([][]string, len(docs))
	df := map[string]int{}
	for i, d := range docs {
		if _, dup := ix.byName[d.Name]; !dup {
			ix.byName[d.Name] = i
		}
		// a slug such as "orbit-dashboard-launch" splits on its hyphens
		// into ordinary terms
		toks[i] = tokenize(d.Name + " " + d.Text)
		// ranging over the distinct terms counts each term once per doc
		for t := range countTerms(toks[i]) {
			df[t]++
		}
	}
	// Smoothed IDF never reaches zero, so a term every document shares
	// still carries a little weight.
	for t, c := range df {
		ix.idf[t] = math.Log((1+n)/(1+float64(c))) + 1
	}

	ix.vecs = make([]vector, len(docs))
	for i := range docs {
		ix.vecs[i] = ix.weigh(toks[i])
	}
	return ix
}

// countTerms returns how many times each term occurs in toks. New uses
// only its keys (the distinct terms); weigh uses the counts as raw TF.
func countTerms(toks []string) map[string]int {
	c := map[string]int{}
	for _, t := range toks {
		c[t]++
	}
	return c
}

// weigh turns tokens into an L2-normalized TF-IDF vector. Terms the
// index has never seen have no IDF and are skipped. TF is the count
// divided by all of toks, unknown terms included. An empty toks gives an
// empty vector, which scores zero against everything.
func (ix *Index) weigh(toks []string) vector {
	if len(toks) == 0 {
		return vector{}
	}
	counts := countTerms(toks)
	total := float64(len(toks))
	v := vector{}
	for t, c := range counts {
		w, ok := ix.idf[t]
		if !ok {
			continue
		}
		v[t] = (float64(c) / total) * w
	}
	return normalize(v)
}

// sortedTerms returns the terms of v in byte order. Sums run in this
// order because map order changes on every range, and float addition in
// a different order can differ in the last bit.
func sortedTerms(v vector) []string {
	ts := make([]string, 0, len(v))
	for t := range v {
		ts = append(ts, t)
	}
	sort.Strings(ts)
	return ts
}

// normalize scales v in place to unit length (L2 norm 1) and returns it.
// The squares are summed in sorted term order so the norm, and so every
// weight, is the same on every run. A vector with no weight is returned
// unchanged rather than divided by zero.
func normalize(v vector) vector {
	var sum float64
	for _, t := range sortedTerms(v) {
		sum += v[t] * v[t]
	}
	if sum == 0 {
		return v
	}
	norm := math.Sqrt(sum)
	for t := range v {
		v[t] /= norm
	}
	return v
}

// cosine returns the dot product of two L2-normalized vectors. It scans
// the shorter vector for shared terms and sums them in sorted order, so
// the score is bit-for-bit the same on every call and in either argument
// order.
func cosine(a, b vector) float64 {
	// a term missing from either side adds nothing, so only the shorter
	// vector's terms need checking
	if len(a) > len(b) {
		a, b = b, a
	}
	var shared []string
	for t := range a {
		if _, ok := b[t]; ok {
			shared = append(shared, t)
		}
	}
	sort.Strings(shared)
	var dot float64
	for _, t := range shared {
		dot += a[t] * b[t]
	}
	return dot
}

// centroid averages vectors and renormalizes. An empty input gives an
// empty vector, which scores zero against everything. Terms shared by
// several inputs add up, so the profile leans toward what they have in
// common. The averaging sum runs in map order; that cannot change a
// result here, because each term receives at most one weight per input
// vector in the same input order.
func centroid(vs []vector) vector {
	if len(vs) == 0 {
		return vector{}
	}
	c := vector{}
	for _, v := range vs {
		for t, w := range v {
			c[t] += w
		}
	}
	for t := range c {
		c[t] /= float64(len(vs))
	}
	return normalize(c)
}

// sharedTerms returns up to top terms present in both vectors, ordered
// by the product of their weights (ties alphabetical). That product is
// each term's share of the cosine score, so the first term is the one
// that explains the score best.
func sharedTerms(a, b vector, top int) []string {
	// pair is one shared term and its contribution to the dot product.
	type pair struct {
		term string
		w    float64
	}
	var ps []pair
	for t, w := range a {
		// a weight can be zero only through underflow; such a term
		// explains nothing, so it is not shown
		if bw, ok := b[t]; ok && w*bw > 0 {
			ps = append(ps, pair{t, w * bw})
		}
	}
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].w != ps[j].w {
			return ps[i].w > ps[j].w
		}
		return ps[i].term < ps[j].term
	})
	if len(ps) > top {
		ps = ps[:top]
	}
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.term)
	}
	return out
}

// Recommendation is one ranked result. Shared lists up to four terms
// that contributed most to Score, highest first. Score is a cosine in
// (0, 1], up to float rounding. The JSON names are part of the API and pinned by a test.
type Recommendation struct {
	Name   string   `json:"name"`
	Score  float64  `json:"score"`
	Shared []string `json:"shared_terms"`
}

// rank scores every document against profile, skips names in skip and
// returns the top k with a positive score. A zero score means no shared
// term at all, so it is dropped rather than used as padding. Equal scores
// are ordered by Name, then by position in the index.
//
// Steps:
//  1. Score every document not in skip by cosine against profile.
//  2. Keep positive scores, each with its top four shared terms.
//  3. Sort by score, highest first; a stable sort with Name as the
//     tie-break keeps the order the same on every run.
//  4. Cut the list to k.
//
// Every document is scored on each call, so the cost grows with the
// corpus; that is fine for a blog-sized corpus.
func (ix *Index) rank(profile vector, skip map[string]bool, k int) []Recommendation {
	var out []Recommendation
	for i, d := range ix.docs {
		if skip[d.Name] {
			continue
		}
		s := cosine(profile, ix.vecs[i])
		if s <= 0 {
			continue
		}
		out = append(out, Recommendation{
			Name:   d.Name,
			Score:  s,
			Shared: sharedTerms(profile, ix.vecs[i], 4),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > k {
		out = out[:k]
	}
	return out
}

// Related returns up to k documents most similar to the one called name,
// best first, never including name itself. It returns nil if name is not
// in the index or k <= 0. Fewer than k results means fewer documents
// share any term with it.
func (ix *Index) Related(name string, k int) []Recommendation {
	i, ok := ix.byName[name]
	if !ok || k <= 0 {
		return nil
	}
	return ix.rank(ix.vecs[i], map[string]bool{name: true}, k)
}

// RelatedToSet returns up to k documents closest to the average of the
// named ones (for example, the posts a reader has already opened). The
// named documents are excluded, repeats count once and unknown names are
// ignored. It returns nil if none of the names is in the index.
//
// Steps:
//  1. Walk names once, marking each name to skip and collecting the
//     vector of each known name the first time it appears.
//  2. Average the collected vectors into one normalized profile.
//  3. Rank the rest of the corpus against that profile.
func (ix *Index) RelatedToSet(names []string, k int) []Recommendation {
	if k <= 0 {
		return nil
	}
	skip := map[string]bool{}
	var seen []vector
	for _, name := range names {
		if skip[name] {
			continue
		}
		skip[name] = true
		if i, ok := ix.byName[name]; ok {
			seen = append(seen, ix.vecs[i])
		}
	}
	if len(seen) == 0 {
		return nil
	}
	return ix.rank(centroid(seen), skip, k)
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=024640ff199df45d49dc7c5077bc1a87aaa69b4747085bd0b50d4b437d169f0f ====
