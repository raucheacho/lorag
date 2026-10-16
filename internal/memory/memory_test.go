package memory

import (
	"bytes"
	"hash/fnv"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorag/lorag/internal/core/engine"
)

// words is a deterministic embedder: texts sharing words are close.
type wordsEmbedder struct{ dim int }

func (w wordsEmbedder) Embed(text string) ([]float32, error) {
	v := make([]float32, w.dim)
	for _, t := range strings.Fields(strings.ToLower(text)) {
		t = strings.Trim(t, ".,;:!?'\"`()[]")
		if len(t) < 3 {
			continue
		}
		h := fnv.New32a()
		h.Write([]byte(t))
		v[h.Sum32()%uint32(w.dim)]++
	}
	var n float64
	for _, x := range v {
		n += float64(x * x)
	}
	if n == 0 {
		v[0] = 1
		return v, nil
	}
	for i := range v {
		v[i] /= float32(math.Sqrt(n))
	}
	return v, nil
}
func (w wordsEmbedder) EmbedBatch(ts []string) ([][]float32, error) {
	out := make([][]float32, len(ts))
	for i, t := range ts {
		out[i], _ = w.Embed(t)
	}
	return out, nil
}
func (w wordsEmbedder) Dimension() int { return w.dim }

func open(t *testing.T, dir string, dim int) (*Service, func()) {
	t.Helper()
	eng, err := engine.Open(engine.Config{Dir: dir, Dim: dim})
	if err != nil {
		t.Fatal(err)
	}
	s, err := Open(eng.DB(), wordsEmbedder{dim}, dim)
	if err != nil {
		t.Fatal(err)
	}
	return s, func() { eng.Close() }
}

const metroErr = `Error: Unable to resolve module ./App from /Users/alice/shop/index.js: ./App could not be found
    at ModuleResolver.resolveDependency (/Users/alice/shop/node_modules/metro/src/ModuleResolution.js:144:15)`

func TestSaveRecallFeedback(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	s, done := open(t, dir, 64)

	r, err := s.Save(SaveRequest{
		Problem: "Metro cannot resolve ./App after moving files", Error: metroErr,
		Solution: "Clear caches: rm -rf .expo node_modules/.cache && npx expo start -c",
		Versions: []string{"expo 52", "react-native@0.76"}, Origin: "cursor", SolveCost: 4000,
	})
	if err != nil || r.Action != "created" || r.Entry.Variant != 1 {
		t.Fatalf("save: %v %+v", err, r)
	}
	if got := r.Entry.Versions; strings.Join(got, ",") != "expo@52,react-native@0.76" {
		t.Fatalf("versions %v", got)
	}

	// The same error on another machine is recalled by signature.
	other := strings.ReplaceAll(metroErr, "/Users/alice/shop", "/home/bob/work/app")
	rec, err := s.Recall(RecallRequest{Error: other, Context: map[string]string{"expo": "52.0.3"}})
	if err != nil || len(rec.Matches) != 1 || rec.Matches[0].By != "signature" || rec.Matches[0].Status != Valid {
		t.Fatalf("recall: %v %+v", err, rec)
	}
	if !strings.Contains(rec.Text, "Fix: Clear caches") || strings.Contains(rec.Text, "Problem:") {
		t.Fatalf("text:\n%s", rec.Text)
	}
	if rec.Tokens > 40 {
		t.Fatalf("recall costs %d tokens:\n%s", rec.Tokens, rec.Text)
	}

	// Another major version: still returned, marked recheck for this caller.
	rec, _ = s.Recall(RecallRequest{Error: metroErr, Context: map[string]string{"expo": "51.1.0"}})
	if m := rec.Matches[0]; m.Status != Recheck || !strings.Contains(m.Reason, "expo 51.1.0≠52") || !strings.Contains(rec.Text, "recheck (expo") {
		t.Fatalf("context: %+v\n%s", m, rec.Text)
	}

	// Saving the same fix again confirms it; a different fix is a variant.
	again, _ := s.Save(SaveRequest{Error: other, Solution: "rm -rf .expo node_modules/.cache then npx expo start -c"})
	if again.Action != "confirmed" || again.Entry.Successes != 2 {
		t.Fatalf("confirm: %+v", again)
	}
	alt, _ := s.Save(SaveRequest{Error: metroErr, Solution: "Fix the import path case: ./app → ./App (macOS is case-insensitive, Linux is not)"})
	if alt.Action != "variant" || alt.Entry.ID != alt.Entry.Signature+"-2" {
		t.Fatalf("variant: %+v", alt)
	}

	// Two failures make variant 2 obsolete; it is not recalled any more.
	s.Feedback(alt.Entry.ID, false, "did not help", "cursor")
	e, _ := s.Feedback(alt.Entry.ID, false, "", "cursor")
	if e.Status != Obsolete {
		t.Fatalf("status %s after failures %d/%d", e.Status, e.Failures, e.Successes)
	}
	rec, _ = s.Recall(RecallRequest{Error: metroErr, K: 3})
	if len(rec.Matches) != 1 || rec.Matches[0].Entry.Variant != 1 {
		t.Fatalf("obsolete variant recalled: %+v", rec.Matches)
	}

	// History keeps every step.
	_, hist, _ := s.Get(r.Entry.ID)
	var types []string
	for _, ev := range hist {
		types = append(types, ev.Type)
	}
	if strings.Join(types, ",") != "create,recall,recall,confirm,recall" {
		t.Fatalf("history %v", types)
	}

	st, _ := s.Stats()
	if st.Entries != 2 || st.Hits != 3 || st.Misses != 0 || st.Feedback.Failed != 2 || st.TokensSaved < 3*(4000-60) {
		t.Fatalf("stats %+v", st)
	}
	done()

	// Everything survives a reopen.
	s, done = open(t, dir, 64)
	if rec, _ := s.Recall(RecallRequest{Error: other}); len(rec.Matches) != 1 {
		t.Fatal("entry lost after reopen")
	}
	done()

	// A new embedding dimension re-embeds the entries.
	s, done = open(t, dir, 32)
	defer done()
	if rec, _ := s.Recall(RecallRequest{Query: "metro cannot resolve App after moving files"}); len(rec.Matches) == 0 || rec.Matches[0].By != "similarity" {
		t.Fatalf("not recalled after migration: %+v", rec)
	}
	if es, _ := s.List(ListRequest{}); len(es) != 2 {
		t.Fatalf("%d entries after migration", len(es))
	}
}

func TestRecallSimilarityAndMiss(t *testing.T) {
	s, done := open(t, t.TempDir(), 64)
	defer done()
	s.Save(SaveRequest{Problem: "postgres connection refused inside docker compose network", Solution: "Use the service name db as host, not localhost"})

	rec, err := s.Recall(RecallRequest{Query: "connection refused to postgres from docker compose"})
	if err != nil || len(rec.Matches) != 1 || rec.Matches[0].By != "similarity" || !strings.Contains(rec.Text, "Problem: postgres") {
		t.Fatalf("similar: %v %+v", err, rec)
	}
	rec, _ = s.Recall(RecallRequest{Query: "how to bake sourdough bread"})
	if len(rec.Matches) != 0 || rec.Text != "No memory for this problem." {
		t.Fatalf("unrelated recall: %+v", rec)
	}
	st, _ := s.Stats()
	if st.Hits != 1 || st.Misses != 1 || st.HitRate != 0.5 {
		t.Fatalf("stats %+v", st)
	}
}

func TestSpacesAndSecrets(t *testing.T) {
	s, done := open(t, t.TempDir(), 64)
	defer done()
	r, _ := s.Save(SaveRequest{
		Problem: "billing webhook signature mismatch", Project: "shop",
		Solution: "Use the raw body; STRIPE_SECRET_KEY=sk_live_abcdefghijklmnopqrstuvwxyz",
	})
	if r.Entry.Space != "shop" || strings.Contains(r.Entry.Solution, "abcdefghijklmnop") {
		t.Fatalf("entry %+v", r.Entry)
	}
	if rec, _ := s.Recall(RecallRequest{Query: "billing webhook signature mismatch", Project: "blog"}); len(rec.Matches) != 0 {
		t.Fatal("project entry leaked to another project")
	}
	if rec, _ := s.Recall(RecallRequest{Query: "billing webhook signature mismatch", Project: "shop"}); len(rec.Matches) != 1 {
		t.Fatal("project entry not recalled in its project")
	}
	var buf bytes.Buffer
	if err := s.Export(&buf); err != nil || strings.Count(buf.String(), "\n") != 1 || strings.Contains(buf.String(), "sk_live") {
		t.Fatalf("export: %v %q", err, buf.String())
	}
}

func TestUpdateAndDeleteKeepHistory(t *testing.T) {
	s, done := open(t, t.TempDir(), 64)
	defer done()
	r, _ := s.Save(SaveRequest{Problem: "go test hangs on CI", Solution: "add -timeout 5m"})
	sol := "run go test -race -timeout 5m ./..."
	e, err := s.Update(r.Entry.ID, Edit{Solution: &sol})
	if err != nil || e.Revision != 2 || e.Solution != sol || e.Author != "person" {
		t.Fatalf("update: %v %+v", err, e)
	}
	if err := s.Delete(r.Entry.ID); err != nil {
		t.Fatal(err)
	}
	e, hist, err := s.Get(r.Entry.ID)
	if err != nil || e != nil || len(hist) != 3 || hist[1].After.Solution != sol {
		t.Fatalf("after delete: %v %+v %+v", err, e, hist)
	}
	if rec, _ := s.Recall(RecallRequest{Query: "go test hangs on CI"}); len(rec.Matches) != 0 {
		t.Fatal("deleted entry recalled")
	}
}

func TestFormatBudget(t *testing.T) {
	long := strings.Repeat("step ", 200)
	text, used := format([]Match{
		{Entry: &Entry{ID: "a-1", Solution: long, Successes: 1}, By: "signature", Status: Valid},
		{Entry: &Entry{ID: "b-1", Solution: "short", Successes: 1}, By: "signature", Status: Valid},
	}, 40)
	if used != 1 || EstimateTokens(text) > 48 || !strings.HasSuffix(text, "…") {
		t.Fatalf("used %d, %d tokens:\n%s", used, EstimateTokens(text), text)
	}
}
