// Package related ranks posts by content similarity, for a "related
// posts" list. It is a small TF-IDF and cosine-similarity engine with no
// dependencies outside the standard library.
//
// The IDF is the smoothed form log((1+N)/(1+df)) + 1, the default in
// scikit-learn's TfidfVectorizer, so scores can be cross-checked there.
// Every result lists the terms that contributed most to its score.
package related

import (
	"encoding/json"
	"html"
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
func tokenize(doc string) []string {
	var out []string
	var b strings.Builder
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
// Text is what it is compared by (title, tags, body text).
type Doc struct {
	Name string
	Text string
}

// vector is a sparse, L2-normalized TF-IDF vector: term -> weight.
type vector map[string]float64

// Index is a fitted corpus. Build it with New. It is read-only after
// that and safe for concurrent use.
type Index struct {
	docs   []Doc
	byName map[string]int
	idf    map[string]float64
	vecs   []vector
}

// New fits an index over docs: document frequencies, smoothed IDF,
// TF-IDF weights and L2 normalization. Name is tokenized along with
// Text, so the words in a slug count toward its content. If two docs
// share a Name, lookups use the first.
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
		toks[i] = tokenize(d.Name + " " + d.Text)
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

func countTerms(toks []string) map[string]int {
	c := map[string]int{}
	for _, t := range toks {
		c[t]++
	}
	return c
}

// weigh turns tokens into an L2-normalized TF-IDF vector. Terms the
// index has never seen have no IDF and are skipped.
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
// empty vector, which scores zero against everything.
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
// by the product of their weights (ties alphabetical).
func sharedTerms(a, b vector, top int) []string {
	type pair struct {
		term string
		w    float64
	}
	var ps []pair
	for t, w := range a {
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
// that contributed most to Score, highest first.
type Recommendation struct {
	Name   string   `json:"name"`
	Score  float64  `json:"score"`
	Shared []string `json:"shared_terms"`
}

// rank scores every document against profile, skips names in skip and
// returns the top k with a positive score. A zero score means no shared
// term at all, so it is dropped rather than used as padding. Equal scores
// are ordered by Name, then by position in the index.
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
func BodyText(body []byte) string {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return plainText(string(body))
	}
	var parts []string
	collectText(v, "", &parts)
	return strings.Join(parts, " ")
}

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
	for i := 0; i < len(s); i++ {
		if s[i] == '<' && i+1 < len(s) && isTagStart(s[i+1]) {
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

func isTagStart(c byte) bool {
	return c == '/' || c == '!' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
