package related

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

// posts is a small fixture of ACME Inc. blog posts, keyed by slug.
var posts = []Doc{
	{Name: "orbit-dashboard-launch", Text: "ACME Inc. launches Orbit, a dashboard for live sales reports. Orbit charts revenue by region and updates every minute."},
	{Name: "orbit-dashboard-tips", Text: "Five tips for the Orbit dashboard: filter sales reports by region, pin revenue charts, and share links with your team."},
	{Name: "q3-revenue-results", Text: "Third quarter revenue grew 12 percent. Sales in every region beat the forecast."},
	{Name: "hybrid-work-policy", Text: "The updated hybrid work policy sets core office hours, an equipment stipend and travel rules."},
	{Name: "new-office-opening", Text: "Our new office opens in March with bike parking, quiet rooms and longer office hours."},
	{Name: "volunteer-day-recap", Text: "Forty employees spent volunteer day planting trees along the river trail."},
}

func norm(v vector) float64 {
	var s float64
	for _, w := range v {
		s += w * w
	}
	return math.Sqrt(s)
}

func names(recs []Recommendation) []string {
	out := make([]string, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.Name)
	}
	return out
}

func TestTokenizeKeepsSignalAndDropsStopWords(t *testing.T) {
	got := tokenize("The launch, 2026 -- quarterly roadmap ON the dashboard. a")
	want := map[string]bool{"launch": true, "2026": true, "quarterly": true,
		"roadmap": true, "dashboard": true}
	for _, tok := range got {
		if !want[tok] {
			t.Errorf("tokenize kept %q, which carries no signal", tok)
		}
		delete(want, tok)
	}
	for tok := range want {
		t.Errorf("tokenize dropped %q, which is signal", tok)
	}
	if len(tokenize("")) != 0 || len(tokenize("!!! -- ,")) != 0 {
		t.Error("punctuation alone produced terms")
	}
}

func TestTokenizeKeepsNonASCIIWordsWhole(t *testing.T) {
	got := tokenize("Café menu: crème brûlée, é")
	want := []string{"café", "menu", "crème", "brûlée"}
	if len(got) != len(want) {
		t.Fatalf("tokenize = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTokenizeKeepsCombiningMarksInWords(t *testing.T) {
	// "cafe" + U+0301 is the decomposed spelling of café. The Hindi word
	// has vowel signs and a virama between its letters. A mark with no
	// letter before it does not start a token.
	got := tokenize("café हिन्दी ́xy")
	want := []string{"café", "हिन्दी", "xy"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize = %q, want %q", got, want)
	}
}

// The smoothed IDF keeps a term shared by every document positive, and a
// rarer term always outweighs a common one.
func TestIDFIsSmoothedAndOrdered(t *testing.T) {
	ix := New([]Doc{
		{Name: "a", Text: "launch rare"},
		{Name: "b", Text: "launch"},
		{Name: "c", Text: "launch"},
	})
	if ix.idf["launch"] <= 0 {
		t.Errorf("a term in every document weighs %.4f; smoothing should keep it positive", ix.idf["launch"])
	}
	if ix.idf["rare"] <= ix.idf["launch"] {
		t.Errorf("rare (%.4f) does not outweigh common (%.4f)", ix.idf["rare"], ix.idf["launch"])
	}
	want := math.Log((1+3)/(1+float64(3))) + 1
	if math.Abs(ix.idf["launch"]-want) > 1e-12 {
		t.Errorf("idf(launch) = %.12f, want %.12f", ix.idf["launch"], want)
	}
}

func TestCosineIsWellBehaved(t *testing.T) {
	ix := New([]Doc{
		{Name: "a", Text: "pricing roadmap dashboard"},
		{Name: "b", Text: "pricing roadmap dashboard"},
		{Name: "c", Text: "garden picnic weekend"},
	})
	if s := cosine(ix.vecs[0], ix.vecs[1]); math.Abs(s-1) > 1e-9 {
		t.Errorf("identical documents scored %.9f, want 1", s)
	}
	if s := cosine(ix.vecs[0], ix.vecs[2]); s != 0 {
		t.Errorf("documents sharing nothing scored %.9f, want 0", s)
	}
	if a, b := cosine(ix.vecs[0], ix.vecs[2]), cosine(ix.vecs[2], ix.vecs[0]); a != b {
		t.Errorf("cosine is not symmetric: %.9f vs %.9f", a, b)
	}
	for i := range ix.vecs {
		for j := range ix.vecs {
			if s := cosine(ix.vecs[i], ix.vecs[j]); s < -1e-12 || s > 1+1e-12 {
				t.Errorf("cosine(%d,%d) = %.9f, outside [0,1]", i, j, s)
			}
		}
	}
}

func TestCentroidIsNormalizedAndFavoursSharedTerms(t *testing.T) {
	if len(centroid(nil)) != 0 {
		t.Error("an empty set produced a profile")
	}
	ix := New([]Doc{
		{Name: "a", Text: "pricing roadmap"},
		{Name: "b", Text: "pricing dashboard"},
	})
	c := centroid(ix.vecs)
	if math.Abs(norm(c)-1) > 1e-9 {
		t.Errorf("the profile has norm %.9f, want 1", norm(c))
	}
	if !(c["pricing"] > c["roadmap"] && c["pricing"] > c["dashboard"]) {
		t.Errorf("the profile does not favour the shared term: %v", c)
	}
}

func TestEveryVectorIsUnitLength(t *testing.T) {
	ix := New(posts)
	if len(ix.vecs) != len(posts) {
		t.Fatalf("fitted %d vectors for %d posts", len(ix.vecs), len(posts))
	}
	for i, v := range ix.vecs {
		if n := norm(v); math.Abs(n-1) > 1e-9 {
			t.Errorf("%s has norm %.9f, want 1", ix.docs[i].Name, n)
		}
	}
}

func TestRelatedFindsTheClosestPostFirst(t *testing.T) {
	ix := New(posts)
	got := ix.Related("orbit-dashboard-launch", 3)
	if len(got) == 0 || got[0].Name != "orbit-dashboard-tips" {
		t.Fatalf("Related(launch) = %v, want orbit-dashboard-tips first", names(got))
	}
	if got := ix.Related("hybrid-work-policy", 1); len(got) != 1 || got[0].Name != "new-office-opening" {
		t.Errorf("Related(hybrid-work-policy, 1) = %v, want [new-office-opening]", names(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Score > got[i-1].Score {
			t.Errorf("results out of order: %v", got)
		}
	}
}

// Related never returns the seed post and never pads the list with
// zero-score posts.
func TestRelatedNeverReturnsTheSeedAndNeverPads(t *testing.T) {
	ix := New(posts)
	recs := ix.Related("orbit-dashboard-launch", 5)
	if len(recs) != 2 {
		t.Errorf("Related(launch, 5) = %v, want only the 2 posts that share a term", names(recs))
	}
	for _, r := range recs {
		if r.Name == "orbit-dashboard-launch" {
			t.Error("the seed post was recommended back")
		}
		if r.Score <= 0 {
			t.Errorf("%s was returned on a zero score", r.Name)
		}
		if len(r.Shared) == 0 {
			t.Errorf("%s scored %.3f with no shared term", r.Name, r.Score)
		}
	}
	if got := ix.Related("no-such-post", 5); got != nil {
		t.Errorf("an unknown slug returned %v", names(got))
	}
	if got := ix.Related("orbit-dashboard-launch", 0); got != nil {
		t.Errorf("k=0 returned %v", names(got))
	}
	if got := New(nil).Related("anything", 3); got != nil {
		t.Errorf("an empty index returned %v", names(got))
	}
}

func TestSharedTermsAreInBothPosts(t *testing.T) {
	ix := New(posts)
	for _, seed := range posts {
		a := ix.vecs[ix.byName[seed.Name]]
		for _, r := range ix.Related(seed.Name, len(posts)) {
			b := ix.vecs[ix.byName[r.Name]]
			if len(r.Shared) > 4 {
				t.Errorf("%s -> %s lists %d shared terms, want at most 4", seed.Name, r.Name, len(r.Shared))
			}
			for _, term := range r.Shared {
				if a[term] == 0 || b[term] == 0 {
					t.Errorf("%s -> %s: %q shown as shared but one side lacks it", seed.Name, r.Name, term)
				}
			}
		}
	}
}

func TestRelatedToSet(t *testing.T) {
	ix := New(posts)
	if got := ix.RelatedToSet(nil, 5); got != nil {
		t.Errorf("an empty set returned %v", names(got))
	}
	if got := ix.RelatedToSet([]string{"no-such-post"}, 5); got != nil {
		t.Errorf("a set of unknown slugs returned %v", names(got))
	}

	read := []string{"orbit-dashboard-launch", "orbit-dashboard-tips", "orbit-dashboard-launch"}
	got := ix.RelatedToSet(read, 5)
	if len(got) == 0 || got[0].Name != "q3-revenue-results" {
		t.Fatalf("RelatedToSet(orbit posts) = %v, want q3-revenue-results first", names(got))
	}
	for _, r := range got {
		if r.Name == "orbit-dashboard-launch" || r.Name == "orbit-dashboard-tips" {
			t.Errorf("a post already in the set was recommended: %s", r.Name)
		}
	}

	// A repeated slug counts once toward the profile.
	once := ix.RelatedToSet([]string{"hybrid-work-policy"}, 5)
	thrice := ix.RelatedToSet([]string{"hybrid-work-policy", "hybrid-work-policy", "hybrid-work-policy"}, 5)
	if len(once) == 0 || len(once) != len(thrice) {
		t.Fatalf("repeats changed the result: %v vs %v", once, thrice)
	}
	for i := range once {
		if once[i].Name != thrice[i].Name || math.Abs(once[i].Score-thrice[i].Score) > 1e-12 {
			t.Errorf("repeats changed result %d: %v vs %v", i, once[i], thrice[i])
		}
	}
}

// Sums run in a fixed term order, so the same posts give bit-identical
// results on every call and every build, whatever order the maps range in.
func TestResultsAreDeterministic(t *testing.T) {
	first := New(posts)
	want := map[string][]Recommendation{}
	for _, p := range posts {
		want[p.Name] = first.Related(p.Name, len(posts))
	}
	set := []string{"orbit-dashboard-launch", "hybrid-work-policy"}
	wantSet := first.RelatedToSet(set, len(posts))
	for run := 0; run < 50; run++ {
		ix := New(posts)
		for _, p := range posts {
			if got := ix.Related(p.Name, len(posts)); !reflect.DeepEqual(got, want[p.Name]) {
				t.Fatalf("run %d: Related(%s) = %v, want %v", run, p.Name, got, want[p.Name])
			}
		}
		if got := ix.RelatedToSet(set, len(posts)); !reflect.DeepEqual(got, wantSet) {
			t.Fatalf("run %d: RelatedToSet = %v, want %v", run, got, wantSet)
		}
	}
}

// Two posts with the same text score exactly the same and come back in
// Name order, not in index order or at random.
func TestEqualScoresTieByName(t *testing.T) {
	docs := []Doc{
		{Name: "launch-recap", Text: "Orbit dashboard launch: sales reports by region."},
		{Name: "copy-b", Text: "Orbit dashboard sales reports by region and revenue charts."},
		{Name: "copy-a", Text: "Orbit dashboard sales reports by region and revenue charts."},
		{Name: "garden", Text: "Volunteer day planting trees."},
	}
	for run := 0; run < 50; run++ {
		got := New(docs).Related("launch-recap", 5)
		if len(got) != 2 || got[0].Name != "copy-a" || got[1].Name != "copy-b" {
			t.Fatalf("run %d: Related = %v, want [copy-a copy-b]", run, names(got))
		}
		if got[0].Score != got[1].Score {
			t.Fatalf("run %d: identical posts scored %v and %v", run, got[0].Score, got[1].Score)
		}
	}
}

// Link cards to the same site must not make two unrelated posts match on
// "https", "example" and "com".
func TestLinkCardsDoNotMatchPosts(t *testing.T) {
	card := func(u string) string {
		return BodyText([]byte(`{"blocks":[{"type":"linkTool","data":{"link":"` + u + `"}}]}`))
	}
	ix := New([]Doc{
		{Name: "q3", Text: "Quarterly revenue grew. " + card("https://example.com/q3")},
		{Name: "trees", Text: "Volunteer day planting. " + card("https://example.com/trees")},
	})
	if got := ix.Related("q3", 5); len(got) != 0 {
		t.Errorf("posts sharing only a link domain matched: %v", got)
	}
}

func TestRecommendationJSONFieldNames(t *testing.T) {
	b, err := json.Marshal(Recommendation{Name: "a", Score: 0.5, Shared: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `{"name":"a","score":0.5,"shared_terms":["x"]}`; got != want {
		t.Errorf("json = %s, want %s", got, want)
	}
}

func TestBodyText(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "editorjs",
			body: `{"time":1760000000000,"blocks":[
				{"id":"a1","type":"header","data":{"text":"Orbit is live","level":2}},
				{"id":"a2","type":"paragraph","data":{"text":"Sales &amp; revenue <b>by region</b>"}},
				{"id":"a3","type":"list","data":{"style":"unordered","items":["Pin charts","Share links"]}}
			],"version":"2.28.0"}`,
			want: "Orbit is live Sales & revenue by region Pin charts Share links",
		},
		{
			name: "tiptap",
			body: `{"type":"doc","content":[
				{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Office hours"}]},
				{"type":"paragraph","content":[
					{"type":"text","text":"Core hours are "},
					{"type":"text","marks":[{"type":"bold"}],"text":"10 to 3"},
					{"type":"text","marks":[{"type":"link","attrs":{"href":"https://example.com/policy"}}],"text":"policy"}
				]}
			]}`,
			want: "Office hours Core hours are 10 to 3 policy",
		},
		{
			name: "editorjs link card and embed",
			body: `{"blocks":[
				{"type":"linkTool","data":{"link":"https://example.com/orbit","meta":{"title":"Orbit tour","image":{"url":"https://example.com/o.png"}}}},
				{"type":"embed","data":{"service":"youtube","source":"https://youtu.be/x1","embed":"https://www.youtube.com/embed/x1","width":580,"height":320,"caption":"Launch video"}}
			]}`,
			want: "Orbit tour Launch video",
		},
		{name: "bare links under other keys", body: `{"cta":"https://example.com/more","label":"Read more","mail":" MAILTO:press@example.com "}`, want: "Read more"},
		{name: "prose that starts with a link", body: `{"text":"https://example.com is live"}`, want: "https://example.com is live"},
		{name: "line break tag splits words", body: `{"text":"one<br>two"}`, want: "one two"},
		{name: "escaped tag is text", body: `{"text":"Use &lt;b&gt; for bold"}`, want: "Use <b> for bold"},
		{name: "bare less-than", body: `{"text":"a < b and c > d"}`, want: "a < b and c > d"},
		{name: "json string", body: `"Just a sentence"`, want: "Just a sentence"},
		{name: "json null", body: `null`, want: ""},
		{name: "not json", body: "Plain <i>text</i>  body", want: "Plain text body"},
		{name: "empty", body: "", want: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BodyText([]byte(c.body)); got != c.want {
				t.Errorf("BodyText = %q, want %q", got, c.want)
			}
		})
	}
}
