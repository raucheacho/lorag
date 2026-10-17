// Package memory is what agents already know: fixes (a problem and what
// solved it) and knowledge (a question and its verified answer, e.g. found
// after a web search). Agents recall an entry in a few tokens instead of
// solving the problem or searching the web again.
//
// Entries live in a VectixDB collection next to the document chunks; every
// change is also appended to an event collection, so an entry is never
// overwritten silently and its history stays readable.
package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/lorag/lorag/internal/core/engine"
	"github.com/lorag/lorag/internal/embeddings"
	vtx "github.com/raucheacho/vectixdb/bindings/go"
)

// Status of an entry.
type Status string

const (
	Valid    Status = "valid"
	Recheck  Status = "recheck"
	Obsolete Status = "obsolete"
)

// Global is the space shared by every project.
const Global = "global"

// Kinds of entries.
const (
	// KindFix is a problem and what solved it.
	KindFix = "fix"
	// KindKnowledge is a question and its verified answer: a command, a
	// configuration value, an API detail, a decision. Not a task.
	KindKnowledge = "knowledge"
)

const (
	metaID          = "memory"
	entryPrefix     = "memory_"
	eventsName      = "memory_events"
	schemaVersion   = 3
	listLimit       = 10_000
	defaultSolve    = 1500 // tokens assumed spent solving a problem from scratch
	staleAfter      = 180 * 24 * time.Hour
	defaultBudget   = 60
	maxRecallK      = 5
	similarSolution = 0.6 // word overlap above which two solutions are the same fix
)

// Entry is one problem with one solution (a variant). Several variants can
// share a signature when different fixes worked.
type Entry struct {
	ID        string    `json:"id"`
	Signature string    `json:"signature"`
	Kind      string    `json:"kind"` // fix or knowledge
	Variant   int       `json:"variant"`
	Revision  int       `json:"revision"`
	Problem   string    `json:"problem"`
	Error     string    `json:"error,omitempty"`
	Solution  string    `json:"solution"`
	Space     string    `json:"space"`
	Project   string    `json:"project,omitempty"`
	Platform  string    `json:"platform,omitempty"`
	Versions  []string  `json:"versions,omitempty"` // "expo@52"
	Tags      []string  `json:"tags,omitempty"`
	Status    Status    `json:"status"`
	Successes int       `json:"successes"`
	Failures  int       `json:"failures"`
	Recalls   int       `json:"recalls"`
	Author    string    `json:"author"`           // agent or person
	Origin    string    `json:"origin,omitempty"` // MCP client name
	SolveCost int       `json:"solve_tokens"`
	Source    string    `json:"source,omitempty"` // where the knowledge comes from: URL, document, person
	Verified  time.Time `json:"verified_at"`
	Created   time.Time `json:"created_at"`
	Updated   time.Time `json:"updated_at"`
}

// Confidence is the Laplace estimate of the fix working, nudged by who wrote
// it: a person's entry starts higher than an agent's.
func (e *Entry) Confidence() float64 {
	prior := 1.0
	if e.Author == "person" {
		prior = 3
	}
	return (float64(e.Successes) + prior) / (float64(e.Successes+e.Failures) + prior + 1)
}

// Event is one change in the journal.
type Event struct {
	ID      string    `json:"id"`
	At      time.Time `json:"at"`
	Type    string    `json:"type"` // create, variant, confirm, recall, miss, feedback, edit, status, delete
	Entry   string    `json:"entry,omitempty"`
	Actor   string    `json:"actor,omitempty"`
	Summary string    `json:"summary,omitempty"`
	Tokens  int       `json:"tokens,omitempty"` // recall: tokens returned
	Worked  *bool     `json:"worked,omitempty"`
	Query   string    `json:"query,omitempty"`
	After   *Entry    `json:"after,omitempty"` // snapshot after the change
}

// Service is safe for concurrent use.
type Service struct {
	db       *vtx.DB
	embedder embeddings.Provider
	dim      int
	entries  string // active entry collection

	thresholds Thresholds

	mu   sync.Mutex // serializes writes (dedupe and counters)
	seq  atomic.Int64
	vecs map[string][]float32 // stored vectors (reads do not return them), guarded by mu
}

// Open opens the memory collections in db, creating them on first use. When
// the embedding dimension changed, entries are re-embedded into a new
// generation before it becomes active.
func Open(db *vtx.DB, embedder embeddings.Provider, dim int, opts Options) (*Service, error) {
	s := &Service{db: db, embedder: embedder, dim: dim, vecs: map[string][]float32{}, thresholds: thresholdsFor(opts)}
	s.seq.Store(time.Now().UnixNano())
	names, err := db.Collections()
	if err != nil {
		return nil, err
	}
	if !contains(names, eventsName) {
		if err := db.CreateCollection(eventsName, vtx.Schema{Fields: []vtx.Field{
			{Name: "entry", Type: vtx.TypeKeyword},
			{Name: "type", Type: vtx.TypeKeyword},
			{Name: "at", Type: vtx.TypeTime},
			{Name: "summary", Type: vtx.TypeText},
		}}); err != nil {
			return nil, fmt.Errorf("memory: create events: %w", err)
		}
	}

	want := fmt.Sprintf("schema=%d dim=%d model=%s", schemaVersion, dim, opts.Model)
	var active, config string
	if meta, err := db.Get(engine.MetaCollection, metaID); err != nil {
		return nil, err
	} else if meta != nil {
		a, _ := meta.Fields["active"].(vtx.Keyword)
		c, _ := meta.Fields["config"].(vtx.Keyword)
		active, config = string(a), string(c)
	}
	gen := 0
	for _, n := range names {
		if g, err := strconv.Atoi(strings.TrimPrefix(n, entryPrefix)); err == nil && strings.HasPrefix(n, entryPrefix) && g > gen {
			gen = g
		}
	}
	switch {
	case active != "" && config == want && contains(names, active):
		s.entries = active
	default:
		next := fmt.Sprintf("%s%d", entryPrefix, gen+1)
		if err := db.CreateCollection(next, s.schema()); err != nil {
			return nil, fmt.Errorf("memory: create %s: %w", next, err)
		}
		if active != "" && contains(names, active) {
			if err := s.migrate(active, next); err != nil {
				db.DropCollection(next)
				return nil, fmt.Errorf("memory: re-embed entries: %w", err)
			}
		}
		if _, err := db.PutDocs(engine.MetaCollection, vtx.Doc{ID: metaID, Fields: map[string]vtx.Value{
			"active": vtx.Keyword(next), "config": vtx.Keyword(want),
		}}); err != nil {
			return nil, err
		}
		s.entries = next
	}
	for _, n := range names {
		if strings.HasPrefix(n, entryPrefix) && n != s.entries && n != eventsName {
			if _, err := strconv.Atoi(strings.TrimPrefix(n, entryPrefix)); err == nil {
				db.DropCollection(n)
			}
		}
	}
	return s, nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func (s *Service) schema() vtx.Schema {
	return vtx.Schema{
		Vector: &vtx.VectorSpec{Dim: uint32(s.dim), Metric: vtx.Cosine},
		Fields: []vtx.Field{
			{Name: "signature", Type: vtx.TypeKeyword},
			{Name: "problem", Type: vtx.TypeText},
			{Name: "error", Type: vtx.TypeText},
			{Name: "solution", Type: vtx.TypeText},
			{Name: "space", Type: vtx.TypeKeyword},
			{Name: "status", Type: vtx.TypeKeyword},
			{Name: "tags", Type: vtx.TypeKeywords},
			{Name: "kind", Type: vtx.TypeKeyword},
			{Name: "updated", Type: vtx.TypeTime},
		},
	}
}

// migrate copies every entry of old into next with fresh embeddings.
func (s *Service) migrate(old, next string) error {
	res, err := s.db.Search(old, vtx.Query{K: listLimit})
	if err != nil {
		return err
	}
	for _, r := range res {
		var e Entry
		if json.Unmarshal(r.Payload, &e) != nil {
			continue
		}
		e.Kind = kindOf(&e)
		vec, err := s.embed(&e)
		if err != nil {
			return err
		}
		if _, err := s.db.PutDocs(next, doc(&e, vec)); err != nil {
			return err
		}
		s.vecs[e.ID] = vec
	}
	log.Printf("memory: %d entries re-embedded into %s", len(res), next)
	return nil
}

// embedText is what an entry's vector is computed from. A fix is found by
// its problem and error; an answer also by its content, since that is
// where the meaning often is ("Personal reminder" → "Eat before noon").
func embedText(e *Entry) string {
	text := e.Problem
	if e.Error != "" {
		text += "\n" + errorLine(e.Error)
	}
	if kindOf(e) == KindKnowledge {
		text += "\n" + e.Solution
	}
	return text
}

func (s *Service) embed(e *Entry) ([]float32, error) {
	text := embedText(e)
	v, err := s.embedder.Embed(text)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	if len(v) != s.dim {
		return nil, fmt.Errorf("embedding has %d dimensions, memory expects %d", len(v), s.dim)
	}
	return v, nil
}

func doc(e *Entry, vec []float32) vtx.Doc {
	payload, _ := json.Marshal(e)
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	return vtx.Doc{
		ID:     e.ID,
		Vector: vec,
		Fields: map[string]vtx.Value{
			"signature": vtx.Keyword(e.Signature),
			"problem":   vtx.Text(e.Problem),
			"error":     vtx.Text(e.Error),
			"solution":  vtx.Text(e.Solution),
			"space":     vtx.Keyword(e.Space),
			"status":    vtx.Keyword(string(e.Status)),
			"tags":      vtx.Keywords(tags),
			"kind":      vtx.Keyword(kindOf(e)),
			"updated":   vtx.Time(e.Updated),
		},
		Payload: payload,
	}
}

// kindOf is the entry's kind; entries saved before kinds existed are fixes.
func kindOf(e *Entry) string {
	if e.Kind == KindKnowledge {
		return KindKnowledge
	}
	return KindFix
}

func decode(r *vtx.Result) (*Entry, bool) {
	var e Entry
	if json.Unmarshal(r.Payload, &e) != nil {
		return nil, false
	}
	e.Kind = kindOf(&e)
	return &e, true
}

// ErrNotFound is returned for an unknown entry id.
var ErrNotFound = errors.New("memory entry not found")

func (s *Service) eventID() string {
	return fmt.Sprintf("%020d", s.seq.Add(1))
}

func (s *Service) eventDoc(ev Event) vtx.Op {
	payload, _ := json.Marshal(ev)
	return vtx.Put(eventsName, vtx.Doc{
		ID: ev.ID,
		Fields: map[string]vtx.Value{
			"entry":   vtx.Keyword(ev.Entry),
			"type":    vtx.Keyword(ev.Type),
			"at":      vtx.Time(ev.At),
			"summary": vtx.Text(ev.Summary),
		},
		Payload: payload,
	})
}

// write stores entries and their events in one atomic batch. vecs[i] nil
// keeps entry i's vector (from the cache, else embedded again); the caller
// holds s.mu.
func (s *Service) write(entries []*Entry, vecs [][]float32, events []Event, deletes ...string) error {
	var ops []vtx.Op
	fresh := map[string][]float32{}
	for i, e := range entries {
		var vec []float32
		if vecs != nil {
			vec = vecs[i]
		}
		if vec == nil {
			vec = s.vecs[e.ID]
		}
		if vec == nil {
			v, err := s.embed(e)
			if err != nil {
				return err
			}
			vec = v
		}
		fresh[e.ID] = vec
		ops = append(ops, vtx.Put(s.entries, doc(e, vec)))
	}
	for _, id := range deletes {
		ops = append(ops, vtx.Delete(s.entries, id))
	}
	for _, ev := range events {
		ops = append(ops, s.eventDoc(ev))
	}
	if _, err := s.db.Write(ops...); err != nil {
		return err
	}
	for id, v := range fresh {
		s.vecs[id] = v
	}
	for _, id := range deletes {
		delete(s.vecs, id)
	}
	return nil
}

// ---- save -------------------------------------------------------------------

// SaveRequest is what an agent (or the person) knows once a problem is solved.
type SaveRequest struct {
	// Kind is fix (default when there is an error message) or knowledge.
	Kind      string   `json:"kind,omitempty"`
	Problem   string   `json:"problem"`  // the problem, or the question
	Solution  string   `json:"solution"` // the fix, or the answer
	Source    string   `json:"source,omitempty"`
	Error     string   `json:"error,omitempty"`
	Project   string   `json:"project,omitempty"`
	Space     string   `json:"space,omitempty"` // default: the project, else global
	Platform  string   `json:"platform,omitempty"`
	Versions  []string `json:"versions,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	SolveCost int      `json:"solve_tokens,omitempty"`
	Author    string   `json:"author,omitempty"` // agent (default) or person
	Origin    string   `json:"origin,omitempty"`
}

// SaveResult says what happened to the saved knowledge.
type SaveResult struct {
	Entry  *Entry `json:"entry"`
	Action string `json:"action"` // created, variant (another fix for a known problem), confirmed (same fix again)
}

// Save records a solved problem. A known problem with the same fix is
// confirmed (counted as a success); a different fix becomes a new variant.
// Nothing is overwritten.
func (s *Service) Save(r SaveRequest) (*SaveResult, error) {
	r.Problem = clean(MaskSecrets(r.Problem), 300)
	r.Solution = clean(MaskSecrets(r.Solution), 4000)
	r.Error = clean(MaskSecrets(r.Error), 2000)
	if r.Problem == "" && r.Error != "" {
		r.Problem = clean(errorLine(r.Error), 300)
	}
	if r.Problem == "" || r.Solution == "" {
		return nil, errors.New("problem (or question, or error) and solution (or answer) are required")
	}
	switch r.Kind {
	case KindFix, KindKnowledge:
	case "":
		// An error message means a fix; otherwise it is knowledge.
		r.Kind = KindKnowledge
		if r.Error != "" {
			r.Kind = KindFix
		}
	default:
		return nil, fmt.Errorf("kind must be %s or %s", KindFix, KindKnowledge)
	}
	r.Source = clean(MaskSecrets(r.Source), 500)
	if r.Author != "person" {
		r.Author = "agent"
	}
	space := r.Space
	if space == "" {
		space = r.Project
	}
	if space == "" {
		space = Global
	}
	now := time.Now().UTC()
	sig := Signature(r.Error, r.Problem)

	s.mu.Lock()
	defer s.mu.Unlock()
	variants, err := s.bySignature(sig)
	if err != nil {
		return nil, err
	}
	for _, v := range variants {
		if v.Space == space && overlap(v.Solution, r.Solution) >= similarSolution {
			v.Successes++
			v.Verified, v.Updated = now, now
			v.Versions = union(v.Versions, r.Versions)
			v.Tags = union(v.Tags, r.Tags)
			if v.Source == "" {
				v.Source = r.Source
			}
			if v.Status != Valid {
				v.Status = Valid
			}
			ev := Event{ID: s.eventID(), At: now, Type: "confirm", Entry: v.ID, Actor: actor(r.Author, r.Origin), Summary: v.Problem, After: v}
			if err := s.write([]*Entry{v}, nil, []Event{ev}); err != nil {
				return nil, err
			}
			return &SaveResult{Entry: v, Action: "confirmed"}, nil
		}
	}

	variant := 1
	for _, v := range variants {
		if v.Variant >= variant {
			variant = v.Variant + 1
		}
	}
	cost := r.SolveCost
	if cost <= 0 {
		cost = defaultSolve
	}
	e := &Entry{
		ID: fmt.Sprintf("%s-%d", sig, variant), Signature: sig, Kind: r.Kind, Variant: variant, Revision: 1, Source: r.Source,
		Problem: r.Problem, Error: r.Error, Solution: r.Solution,
		Space: space, Project: r.Project, Platform: r.Platform,
		Versions: normVersions(r.Versions), Tags: union(nil, r.Tags),
		Status: Valid, Successes: 1, Author: r.Author, Origin: r.Origin, SolveCost: cost,
		Verified: now, Created: now, Updated: now,
	}
	vec, err := s.embed(e)
	if err != nil {
		return nil, err
	}
	typ, action := "create", "created"
	if variant > 1 {
		typ, action = "variant", "variant"
	}
	ev := Event{ID: s.eventID(), At: now, Type: typ, Entry: e.ID, Actor: actor(r.Author, r.Origin), Summary: e.Problem, After: e}
	if err := s.write([]*Entry{e}, [][]float32{vec}, []Event{ev}); err != nil {
		return nil, err
	}
	return &SaveResult{Entry: e, Action: action}, nil
}

func actor(author, origin string) string {
	if origin != "" {
		return origin
	}
	return author
}

func (s *Service) bySignature(sig string) ([]*Entry, error) {
	res, err := s.db.Search(s.entries, vtx.Query{Filter: vtx.Eq("signature", vtx.Keyword(sig)), K: 100})
	if err != nil {
		return nil, err
	}
	out := make([]*Entry, 0, len(res))
	for i := range res {
		if e, ok := decode(&res[i]); ok {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Variant < out[j].Variant })
	return out, nil
}

// Get returns an entry and its history, oldest first.
func (s *Service) Get(id string) (*Entry, []Event, error) {
	r, err := s.db.Get(s.entries, id)
	if err != nil {
		return nil, nil, err
	}
	var e *Entry
	if r != nil {
		e, _ = decode(r)
	}
	events, err := s.events(vtx.Eq("entry", vtx.Keyword(id)), listLimit)
	if err != nil {
		return nil, nil, err
	}
	if e == nil && len(events) == 0 {
		return nil, nil, ErrNotFound
	}
	sort.Slice(events, func(i, j int) bool { return events[i].ID < events[j].ID })
	return e, events, nil
}

// ---- recall -----------------------------------------------------------------

// RecallRequest is an agent asking whether a problem was solved before.
type RecallRequest struct {
	Query   string            `json:"query"`
	Error   string            `json:"error,omitempty"`
	Project string            `json:"project,omitempty"`
	Context map[string]string `json:"context,omitempty"` // package → version in use
	K       int               `json:"k,omitempty"`
	Budget  int               `json:"budget_tokens,omitempty"`
	Origin  string            `json:"-"`
}

// Match is a recalled entry with how it matched and whether it holds in the
// caller's context.
type Match struct {
	Entry      *Entry  `json:"entry"`
	By         string  `json:"by"` // signature or similarity
	Similarity float32 `json:"similarity,omitempty"`
	// Status for the caller: the entry's status, or recheck when the
	// caller's versions differ from the ones it was verified with.
	Status Status `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// RecallResult is the compact answer and what it is made of.
type RecallResult struct {
	Text    string  `json:"text"`
	Tokens  int     `json:"tokens"`
	Matches []Match `json:"matches"`
	// Nearest is the best candidate that fell below the thresholds, to
	// understand a miss (Studio and API only, never sent to agents).
	Nearest *Match `json:"nearest,omitempty"`
	// Thresholds in use.
	Thresholds Thresholds `json:"thresholds"`
}

// Options configure the memory.
type Options struct {
	// Model is the embedding model: entries are re-embedded when it
	// changes, and it picks the default thresholds.
	Model string
	// MinSimilarity and MinSimilarityText override the thresholds (0: the
	// calibrated defaults for Model).
	MinSimilarity, MinSimilarityText float64
}

// Thresholds accept a match that is not by signature: cosine similarity at
// least Similarity, or at least SimilarityText when the words match too
// (BM25 hit). A wrong recall costs more tokens than it saves; similarity
// matches also show their problem line so the agent can judge.
type Thresholds struct {
	Similarity     float64 `json:"similarity"`
	SimilarityText float64 `json:"similarity_text"`
	Calibrated     bool    `json:"calibrated"` // defaults measured for this model
}

// calibrated holds thresholds measured on French and English paraphrases
// against unrelated questions (docs/MEMORY.md). Similarities depend on the
// model, so an unknown model gets the strict ones.
var calibrated = map[string]Thresholds{
	// True matches 0.59-0.77 (French, English, across languages); unrelated
	// questions up to 0.49, and 0.575 for a neighbor problem (MySQL vs a
	// Postgres entry).
	"bge-m3": {Similarity: 0.55, SimilarityText: 0.50, Calibrated: true},
	// English-centric: English paraphrases 0.72-0.84, unrelated up to 0.70;
	// French is compressed and cross-language fails. Prefer bge-m3.
	"nomic-embed-text": {Similarity: 0.80, SimilarityText: 0.705, Calibrated: true},
}

func thresholdsFor(o Options) Thresholds {
	name := strings.ToLower(o.Model)
	name, _, _ = strings.Cut(name, ":") // "bge-m3:latest"
	t, ok := calibrated[name]
	if !ok {
		t = Thresholds{Similarity: 0.80, SimilarityText: 0.705}
	}
	if o.MinSimilarity > 0 {
		t.Similarity, t.Calibrated = o.MinSimilarity, false
	}
	if o.MinSimilarityText > 0 {
		t.SimilarityText, t.Calibrated = o.MinSimilarityText, false
	}
	return t
}

// Thresholds returns the thresholds in use.
func (s *Service) Thresholds() Thresholds { return s.thresholds }

// Recall looks the problem up: by signature first, then by meaning and
// words, keeping only confident matches. Obsolete entries are never
// returned. The answer fits in the token budget.
func (s *Service) Recall(r RecallRequest) (*RecallResult, error) {
	if strings.TrimSpace(r.Query) == "" && strings.TrimSpace(r.Error) == "" {
		return nil, errors.New("query or error is required")
	}
	errText := r.Error
	if errText == "" && LooksLikeError(r.Query) {
		errText = r.Query
	}
	if r.K <= 0 {
		r.K = 1
	}
	r.K = min(r.K, maxRecallK)
	if r.Budget <= 0 {
		r.Budget = defaultBudget
	}

	var matches []Match
	var nearest *Match
	seen := map[string]bool{}
	if errText != "" {
		vs, err := s.bySignature(Signature(errText, ""))
		if err != nil {
			return nil, err
		}
		for _, e := range vs {
			if e.Status != Obsolete && inSpace(e, r.Project) {
				matches = append(matches, Match{Entry: e, By: "signature"})
				seen[e.ID] = true
			}
		}
	}
	if len(matches) < r.K {
		text := strings.TrimSpace(r.Query + "\n" + errorLine(errText))
		vec, err := s.embedder.Embed(text)
		if err != nil {
			return nil, fmt.Errorf("embed query: %w", err)
		}
		filter := vtx.Not(vtx.Eq("status", vtx.Keyword(string(Obsolete))))
		if r.Project != "" {
			filter = vtx.And(filter, vtx.In("space", vtx.Keyword(Global), vtx.Keyword(r.Project)))
		}
		res, err := s.db.Search(s.entries, vtx.Query{
			Vector: vec, Text: text, TextFields: []string{"problem", "error", "solution"},
			Filter: filter, K: uint32(r.K * 4), Fusion: vtx.Fusion{RRFK: 60},
		})
		if err != nil {
			return nil, err
		}
		for i := range res {
			h := &res[i]
			sim := float32(0)
			if h.VectorRank > 0 {
				sim = 1 - h.VectorDistance
			}
			e, ok := decode(h)
			if !ok || seen[e.ID] {
				continue
			}
			if float64(sim) < s.thresholds.Similarity && (h.TextRank == 0 || float64(sim) < s.thresholds.SimilarityText) {
				if nearest == nil || sim > nearest.Similarity {
					nearest = &Match{Entry: e, By: "similarity", Similarity: sim, Status: e.Status}
				}
				continue
			}
			seen[e.ID] = true
			matches = append(matches, Match{Entry: e, By: "similarity", Similarity: sim})
		}
	}

	for i := range matches {
		matches[i].Status, matches[i].Reason = contextStatus(matches[i].Entry, r.Context)
	}
	sort.SliceStable(matches, func(i, j int) bool { return better(&matches[i], &matches[j]) })
	if len(matches) > r.K {
		matches = matches[:r.K]
	}

	text, used := format(matches, r.Budget)
	tokens := EstimateTokens(text)
	if len(matches) > used {
		matches = matches[:used]
	}

	// Count the recall: on each returned entry, and as one event.
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	var changed []*Entry
	var events []Event
	for _, m := range matches {
		fresh, err := s.db.Get(s.entries, m.Entry.ID)
		if err != nil || fresh == nil {
			continue
		}
		e, ok := decode(fresh)
		if !ok {
			continue
		}
		e.Recalls++
		m.Entry.Recalls = e.Recalls
		changed = append(changed, e)
	}
	ev := Event{ID: s.eventID(), At: now, Type: "recall", Actor: r.Origin, Tokens: tokens, Query: clean(MaskSecrets(firstNonEmpty(r.Error, r.Query)), 300)}
	if len(matches) == 0 {
		ev.Type = "miss"
	} else {
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.Entry.ID
		}
		ev.Entry, ev.Summary = ids[0], strings.Join(ids, " ")
	}
	events = append(events, ev)
	if err := s.write(changed, nil, events); err != nil {
		log.Printf("memory: record recall: %v", err)
	}
	if len(matches) > 0 {
		nearest = nil
	}
	return &RecallResult{Text: text, Tokens: tokens, Matches: matches, Nearest: nearest, Thresholds: s.thresholds}, nil
}

func inSpace(e *Entry, project string) bool {
	return project == "" || e.Space == Global || e.Space == project
}

// better orders matches: by signature first, then usable for the caller,
// then proven by use, then recently verified.
func better(a, b *Match) bool {
	if (a.By == "signature") != (b.By == "signature") {
		return a.By == "signature"
	}
	if (a.Status == Valid) != (b.Status == Valid) {
		return a.Status == Valid
	}
	sa := a.Entry.Successes - 2*a.Entry.Failures
	sb := b.Entry.Successes - 2*b.Entry.Failures
	if sa != sb {
		return sa > sb
	}
	if a.Similarity != b.Similarity {
		return a.Similarity > b.Similarity
	}
	return a.Entry.Verified.After(b.Entry.Verified)
}

// contextStatus compares the caller's package versions with the ones the
// entry was verified with: a different major version means recheck. Old
// verifications are flagged too.
func contextStatus(e *Entry, ctx map[string]string) (Status, string) {
	if e.Status != Valid {
		return e.Status, ""
	}
	for _, v := range e.Versions {
		pkg, ver, ok := strings.Cut(v, "@")
		if !ok {
			continue
		}
		cur, ok := ctx[pkg]
		if !ok || cur == "" {
			continue
		}
		if major(cur) != major(ver) {
			return Recheck, fmt.Sprintf("%s %s≠%s", pkg, cur, ver)
		}
	}
	if time.Since(e.Verified) > staleAfter {
		return Recheck, "verified " + e.Verified.Format("2006-01-02")
	}
	return Valid, ""
}

func major(v string) string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	m, _, _ := strings.Cut(v, ".")
	return m
}

// format renders matches compactly within budget tokens and returns how many
// fit. The first match is always included, its fix cut if needed. The
// header is the id, successes/failures and the last verification; the
// status only shows when it is not valid.
//
//	[3f2a9c41b7-1] +4/-0 2026-10-02
//	Fix: rm -rf .expo node_modules && npx expo start -c
//	Ctx: expo@52 react-native@0.76
func format(ms []Match, budget int) (string, int) {
	if len(ms) == 0 {
		return "No memory for this problem.", 0
	}
	var b strings.Builder
	used := 0
	for i, m := range ms {
		e := m.Entry
		var block strings.Builder
		fmt.Fprintf(&block, "[%s] +%d/-%d %s", e.ID, e.Successes, e.Failures, e.Verified.Format("2006-01-02"))
		if m.Status != Valid {
			block.WriteString(" " + strings.ToUpper(string(m.Status)))
			if m.Reason != "" {
				block.WriteString(" (" + m.Reason + ")")
			}
		}
		block.WriteString("\n")
		label, question := "Fix: ", "Problem: "
		if kindOf(e) == KindKnowledge {
			label, question = "Answer: ", "Q: "
		}
		if m.By != "signature" {
			block.WriteString(question + e.Problem + "\n")
		}
		fix := label + e.Solution
		// The fix or answer comes first: the source line goes when it would
		// cut it.
		src, ctx := sourceLine(e), contextLine(e)
		rest := budget - EstimateTokens(b.String()+block.String()+src+ctx)
		if src != "" && rest < EstimateTokens(fix) {
			src = ""
			rest = budget - EstimateTokens(b.String()+block.String()+ctx)
		}
		if i > 0 && rest < EstimateTokens(fix) {
			break // later matches only when they fit whole
		}
		fix = cutTokens(fix, max(rest, 12))
		block.WriteString(fix + "\n")
		block.WriteString(src + ctx)
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(block.String())
		used++
	}
	return strings.TrimRight(b.String(), "\n"), used
}

func sourceLine(e *Entry) string {
	if src := shortSource(e.Source); src != "" {
		return "Src: " + src + "\n"
	}
	return ""
}

func contextLine(e *Entry) string {
	var b strings.Builder
	parts := append([]string{}, e.Versions...)
	if e.Platform != "" {
		parts = append(parts, e.Platform)
	}
	if e.Space != "" && e.Space != Global {
		parts = append(parts, "project "+e.Space)
	}
	if len(parts) > 0 {
		b.WriteString("Ctx: " + strings.Join(parts, " ") + "\n")
	}
	return b.String()
}

// shortSource keeps a source readable in a few tokens: for a URL, the host
// and the last path segment; otherwise the source, cut to 50 characters.
func shortSource(src string) string {
	src = strings.TrimSpace(src)
	if u, err := url.Parse(src); err == nil && u.Host != "" {
		host := strings.TrimPrefix(u.Host, "www.")
		path := strings.Trim(u.Path, "/")
		switch i := strings.LastIndex(path, "/"); {
		case path == "":
			return host
		case i < 0:
			return host + "/" + path
		default:
			return host + "/…/" + path[i+1:]
		}
	}
	if r := []rune(src); len(r) > 50 {
		src = string(r[:49]) + "…"
	}
	return src
}

// EstimateTokens approximates the tokenizer of most models: about 3.5
// characters per token for code and English, a little less for punctuation.
func EstimateTokens(s string) int {
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return 0
	}
	return (n*2 + 6) / 7
}

// CutTokens shortens s to about tokens tokens, at a word boundary.
func CutTokens(s string, tokens int) string { return cutTokens(s, tokens) }

func cutTokens(s string, tokens int) string {
	maxRunes := tokens * 7 / 2
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	cut := string(r[:maxRunes-1])
	if i := strings.LastIndexAny(cut, " \n"); i > len(cut)/2 {
		cut = cut[:i]
	}
	return cut + "…"
}

// ---- feedback and edits -------------------------------------------------------

// Feedback records whether a recalled entry helped: the fix worked, or the
// answer was correct and useful. A failure makes the entry
// recheck; repeated failures that outnumber successes make it obsolete. A
// success makes it valid again.
func (s *Service) Feedback(id string, worked bool, note, origin string) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.db.Get(s.entries, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrNotFound
	}
	e, _ := decode(r)
	now := time.Now().UTC()
	if worked {
		e.Successes++
		e.Verified = now
		e.Status = Valid
	} else {
		e.Failures++
		if e.Failures >= 2 && e.Failures > e.Successes {
			e.Status = Obsolete
		} else {
			e.Status = Recheck
		}
	}
	e.Updated = now
	w := worked
	ev := Event{ID: s.eventID(), At: now, Type: "feedback", Entry: id, Actor: origin, Worked: &w, Summary: clean(MaskSecrets(note), 300), After: e}
	if err := s.write([]*Entry{e}, nil, []Event{ev}); err != nil {
		return nil, err
	}
	return e, nil
}

// Edit is a correction by the person: any field left nil is kept.
type Edit struct {
	Kind     *string  `json:"kind,omitempty"`
	Source   *string  `json:"source,omitempty"`
	Problem  *string  `json:"problem,omitempty"`
	Solution *string  `json:"solution,omitempty"`
	Status   *Status  `json:"status,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// Update applies a correction as a new revision; the previous one stays in
// the history.
func (s *Service) Update(id string, ed Edit) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.db.Get(s.entries, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrNotFound
	}
	e, _ := decode(r)
	typ := "edit"
	if ed.Problem != nil {
		e.Problem = clean(MaskSecrets(*ed.Problem), 300)
	}
	if ed.Solution != nil {
		e.Solution = clean(MaskSecrets(*ed.Solution), 4000)
	}
	if ed.Tags != nil {
		e.Tags = union(nil, ed.Tags)
	}
	if ed.Source != nil {
		e.Source = clean(MaskSecrets(*ed.Source), 500)
	}
	if ed.Kind != nil {
		if *ed.Kind != KindFix && *ed.Kind != KindKnowledge {
			return nil, fmt.Errorf("kind must be %s or %s", KindFix, KindKnowledge)
		}
		e.Kind = *ed.Kind
	}
	if ed.Status != nil {
		switch *ed.Status {
		case Valid, Recheck, Obsolete:
		default:
			return nil, fmt.Errorf("status must be valid, recheck or obsolete")
		}
		if ed.Problem == nil && ed.Solution == nil {
			typ = "status"
		}
		e.Status = *ed.Status
		if e.Status == Valid {
			e.Verified = time.Now().UTC()
		}
	}
	if e.Problem == "" || e.Solution == "" {
		return nil, errors.New("problem and solution cannot be empty")
	}
	e.Revision++
	e.Updated = time.Now().UTC()
	e.Author = "person"
	if ed.Problem != nil {
		delete(s.vecs, id) // the vector follows the problem text
	}
	ev := Event{ID: s.eventID(), At: e.Updated, Type: typ, Entry: id, Actor: "person", Summary: e.Problem, After: e}
	if err := s.write([]*Entry{e}, nil, []Event{ev}); err != nil {
		return nil, err
	}
	return e, nil
}

// Delete removes an entry from recall; its history stays in the journal.
func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.db.Get(s.entries, id)
	if err != nil {
		return err
	}
	if r == nil {
		return ErrNotFound
	}
	e, _ := decode(r)
	ev := Event{ID: s.eventID(), At: time.Now().UTC(), Type: "delete", Entry: id, Actor: "person", Summary: e.Problem}
	return s.write(nil, nil, []Event{ev}, id)
}

// ---- listing, journal, statistics --------------------------------------------

// ListRequest filters entries for the Studio and the CLI.
type ListRequest struct {
	Kind   string
	Status Status
	Space  string
	Query  string // words or meaning
	Limit  int
}

// List returns entries, most recently updated first (or by relevance when
// Query is set).
func (s *Service) List(r ListRequest) ([]*Entry, error) {
	var filters []*vtx.Filter
	if r.Kind != "" {
		filters = append(filters, vtx.Eq("kind", vtx.Keyword(r.Kind)))
	}
	if r.Status != "" {
		filters = append(filters, vtx.Eq("status", vtx.Keyword(string(r.Status))))
	}
	if r.Space != "" {
		filters = append(filters, vtx.Eq("space", vtx.Keyword(r.Space)))
	}
	var filter *vtx.Filter
	switch len(filters) {
	case 0:
	case 1:
		filter = filters[0]
	default:
		filter = vtx.And(filters...)
	}
	limit := r.Limit
	if limit <= 0 || limit > listLimit {
		limit = listLimit
	}
	q := vtx.Query{Filter: filter, K: uint32(limit)}
	if r.Query != "" {
		vec, err := s.embedder.Embed(r.Query)
		if err == nil {
			q.Vector = vec
		}
		q.Text = r.Query
		q.TextFields = []string{"problem", "error", "solution"}
		q.Fusion = vtx.Fusion{RRFK: 60}
		q.K = uint32(min(limit, 200))
	}
	res, err := s.db.Search(s.entries, q)
	if err != nil {
		return nil, err
	}
	out := make([]*Entry, 0, len(res))
	for i := range res {
		if e, ok := decode(&res[i]); ok {
			out = append(out, e)
		}
	}
	if r.Query == "" {
		sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	}
	return out, nil
}

func (s *Service) events(filter *vtx.Filter, limit int) ([]Event, error) {
	res, err := s.db.Search(eventsName, vtx.Query{Filter: filter, K: uint32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(res))
	for i := range res {
		var ev Event
		if json.Unmarshal(res[i].Payload, &ev) == nil {
			out = append(out, ev)
		}
	}
	return out, nil
}

// Events returns the journal, newest first.
func (s *Service) Events(limit int) ([]Event, error) {
	evs, err := s.events(nil, listLimit)
	if err != nil {
		return nil, err
	}
	sort.Slice(evs, func(i, j int) bool { return evs[i].ID > evs[j].ID })
	if limit > 0 && len(evs) > limit {
		evs = evs[:limit]
	}
	for i := range evs {
		evs[i].After = nil // the timeline does not need snapshots
	}
	return evs, nil
}

// Stats sums up what the memory holds and what it saved.
type Stats struct {
	Entries     int            `json:"entries"`
	ByStatus    map[Status]int `json:"by_status"`
	BySpace     map[string]int `json:"by_space"`
	ByKind      map[string]int `json:"by_kind"`
	Recalls     int            `json:"recalls"`
	Hits        int            `json:"hits"`
	Misses      int            `json:"misses"`
	HitRate     float64        `json:"hit_rate"`
	TokensSent  int            `json:"tokens_returned"`
	AvgTokens   float64        `json:"avg_recall_tokens"`
	TokensSaved int            `json:"tokens_saved_estimate"`
	Saves       int            `json:"saves"`
	Feedback    struct {
		Worked int `json:"worked"`
		Failed int `json:"failed"`
	} `json:"feedback"`
	Daily     []Day    `json:"daily"`     // last 30 days, oldest first
	Recurring []Ranked `json:"recurring"` // most recalled
	Useful    []Ranked `json:"useful"`    // most successes
}

// Day counts the journal for one day.
type Day struct {
	Date   string `json:"date"`
	Saves  int    `json:"saves"`
	Hits   int    `json:"hits"`
	Misses int    `json:"misses"`
	Worked int    `json:"worked"`
	Failed int    `json:"failed"`
}

// Ranked is an entry with the number that ranks it.
type Ranked struct {
	ID      string `json:"id"`
	Problem string `json:"problem"`
	Count   int    `json:"count"`
}

// Stats computes the statistics from the entries and the journal. Tokens
// saved is an estimate: for each hit, the tokens the fix cost to find
// (given at save, 1,500 by default) minus the tokens of the recall, not
// counted when the agent reported that the fix failed.
func (s *Service) Stats() (*Stats, error) {
	entries, err := s.List(ListRequest{})
	if err != nil {
		return nil, err
	}
	evs, err := s.events(nil, listLimit)
	if err != nil {
		return nil, err
	}
	st := &Stats{Entries: len(entries), ByStatus: map[Status]int{}, BySpace: map[string]int{}, ByKind: map[string]int{}}
	cost := map[string]int{}
	for _, e := range entries {
		st.ByStatus[e.Status]++
		st.BySpace[e.Space]++
		st.ByKind[kindOf(e)]++
		cost[e.ID] = e.SolveCost
	}

	days := map[string]*Day{}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	for i := 29; i >= 0; i-- {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		days[d] = &Day{Date: d}
		st.Daily = append(st.Daily, Day{Date: d})
	}
	failed := map[string]bool{}
	for _, ev := range evs {
		if ev.Type == "feedback" && ev.Worked != nil && !*ev.Worked {
			failed[ev.Entry] = true
		}
	}
	for _, ev := range evs {
		d := days[ev.At.UTC().Format("2006-01-02")]
		switch ev.Type {
		case "create", "variant", "confirm":
			st.Saves++
			if d != nil {
				d.Saves++
			}
		case "recall":
			st.Recalls++
			st.Hits++
			st.TokensSent += ev.Tokens
			if c, ok := cost[ev.Entry]; ok && !failed[ev.Entry] {
				st.TokensSaved += max(c-ev.Tokens, 0)
			}
			if d != nil {
				d.Hits++
			}
		case "miss":
			st.Recalls++
			st.Misses++
			st.TokensSent += ev.Tokens
			if d != nil {
				d.Misses++
			}
		case "feedback":
			if ev.Worked != nil && *ev.Worked {
				st.Feedback.Worked++
				if d != nil {
					d.Worked++
				}
			} else {
				st.Feedback.Failed++
				if d != nil {
					d.Failed++
				}
			}
		}
	}
	for i := range st.Daily {
		st.Daily[i] = *days[st.Daily[i].Date]
	}
	if st.Recalls > 0 {
		st.HitRate = float64(st.Hits) / float64(st.Recalls)
		st.AvgTokens = float64(st.TokensSent) / float64(st.Recalls)
	}
	rank := func(score func(*Entry) int) []Ranked {
		var out []Ranked
		for _, e := range entries {
			if n := score(e); n > 0 {
				out = append(out, Ranked{ID: e.ID, Problem: e.Problem, Count: n})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
		if len(out) > 8 {
			out = out[:8]
		}
		return out
	}
	st.Recurring = rank(func(e *Entry) int { return e.Recalls })
	st.Useful = rank(func(e *Entry) int { return e.Successes - e.Failures })
	return st, nil
}

// Health lists what needs the person's attention.
type Health struct {
	Recheck    []*Entry    `json:"recheck"`
	Obsolete   []*Entry    `json:"obsolete"`
	Stale      []*Entry    `json:"stale"` // valid but not verified for 180 days
	Duplicates [][2]*Entry `json:"duplicates"`
}

// Health finds entries to recheck, obsolete or stale ones, and pairs with
// different signatures that describe the same problem.
func (s *Service) Health() (*Health, error) {
	entries, err := s.List(ListRequest{})
	if err != nil {
		return nil, err
	}
	h := &Health{Recheck: []*Entry{}, Obsolete: []*Entry{}, Stale: []*Entry{}, Duplicates: [][2]*Entry{}}
	byID := map[string]*Entry{}
	for _, e := range entries {
		byID[e.ID] = e
		switch {
		case e.Status == Recheck:
			h.Recheck = append(h.Recheck, e)
		case e.Status == Obsolete:
			h.Obsolete = append(h.Obsolete, e)
		case time.Since(e.Verified) > staleAfter:
			h.Stale = append(h.Stale, e)
		}
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if len(h.Duplicates) >= 20 {
			break
		}
		s.mu.Lock()
		vec := s.vecs[e.ID]
		s.mu.Unlock()
		if vec == nil {
			if vec, err = s.embed(e); err != nil {
				return nil, err
			}
			s.mu.Lock()
			s.vecs[e.ID] = vec
			s.mu.Unlock()
		}
		res, err := s.db.Search(s.entries, vtx.Query{Vector: vec, K: 3, NoPayload: true})
		if err != nil {
			return nil, err
		}
		for _, r := range res {
			o := byID[r.ID]
			if o == nil || o.Signature == e.Signature || 1-r.VectorDistance < 0.93 {
				continue
			}
			key := min(e.ID, o.ID) + "|" + max(e.ID, o.ID)
			if !seen[key] {
				seen[key] = true
				h.Duplicates = append(h.Duplicates, [2]*Entry{e, o})
			}
		}
	}
	return h, nil
}

// Export writes every entry as one JSON object per line.
func (s *Service) Export(w interface{ Write([]byte) (int, error) }) error {
	entries, err := s.List(ListRequest{})
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Created.Before(entries[j].Created) })
	enc := json.NewEncoder(w)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// ---- helpers -------------------------------------------------------------------

func clean(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if utf8.RuneCountInString(s) > max {
		r := []rune(s)
		s = string(r[:max-1]) + "…"
	}
	return s
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

func normVersions(vs []string) []string {
	var out []string
	for _, v := range vs {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		// Accept "expo 52" and "expo=52" as well as "expo@52".
		if !strings.Contains(v, "@") || strings.HasPrefix(v, "@") && strings.Count(v, "@") == 1 {
			if f := strings.FieldsFunc(v, func(r rune) bool { return r == ' ' || r == '=' }); len(f) == 2 {
				v = f[0] + "@" + f[1]
			}
		}
		out = append(out, v)
	}
	return union(nil, out)
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range append(append([]string{}, a...), b...) {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// overlap is the Jaccard index of the word sets of two solutions.
func overlap(a, b string) float64 {
	wa, wb := words(a), words(b)
	if len(wa) == 0 || len(wb) == 0 {
		return 0
	}
	inter := 0
	for w := range wa {
		if wb[w] {
			inter++
		}
	}
	return float64(inter) / float64(len(wa)+len(wb)-inter)
}

func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || r == '/' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r > 127)
	}) {
		if len(w) > 1 {
			out[w] = true
		}
	}
	return out
}
