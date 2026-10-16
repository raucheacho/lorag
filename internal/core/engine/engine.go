// Package engine keeps the searchable copy of the chunks in a VectixDB
// database: vectors, full text and the fields used for filtering, with
// crash-safe storage. The document store stays the source of truth; the
// index can always be rebuilt from it.
//
// Chunks live in a collection named chunks_<generation>. A rebuild fills a
// new generation while searches keep using the current one, then switches
// the pointer kept in the meta collection in one atomic write.
package engine

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	vtx "github.com/raucheacho/vectixdb/bindings/go"
)

const (
	metaCollection = "lorag_meta"
	metaID         = "index"
	chunkPrefix    = "chunks_"

	// Chunks per write during a rebuild.
	rebuildBatch = 512

	// schemaVersion changes when the chunk fields change: a new version
	// starts a new generation, rebuilt from the document store.
	schemaVersion = 2
)

// Config describes the index to open or create.
type Config struct {
	Dir            string
	Dim            int
	Metric         string // cosine, l2 or dot
	Quantized      bool
	M              int
	EfConstruction int
	EfSearch       int
	BM25K1         float32
	BM25B          float32
}

// fingerprint lists every parameter baked into a collection; a change
// requires a new generation.
func (c Config) fingerprint() string {
	return fmt.Sprintf("schema=%d dim=%d metric=%s int8=%t m=%d efc=%d k1=%g b=%g",
		schemaVersion, c.Dim, c.Metric, c.Quantized, c.M, c.EfConstruction, c.BM25K1, c.BM25B)
}

func parseMetric(s string) vtx.Metric {
	switch strings.ToLower(s) {
	case "l2", "euclidean":
		return vtx.L2
	case "dot", "ip", "inner_product":
		return vtx.Dot
	default:
		return vtx.Cosine
	}
}

func metricName(m vtx.Metric) string {
	switch m {
	case vtx.L2:
		return "l2"
	case vtx.Dot:
		return "dot"
	default:
		return "cosine"
	}
}

// Chunk is one indexed passage with the fields needed to display and
// filter it without going back to the document store.
type Chunk struct {
	ID         string
	DocumentID string
	Content    string
	Path       string
	Filename   string
	Extension  string
	Position   int
	StartByte  int
	EndByte    int
	Kind       string
	Page       int
	LineStart  int
	LineEnd    int
	StartMs    int64
	EndMs      int64
	Section    string
	Vector     []float32
}

// Field names of the chunk collection.
const (
	FieldContent    = "content"
	FieldDocumentID = "document_id"
	FieldPath       = "path"
	FieldFilename   = "filename"
	FieldExtension  = "extension"
	FieldPosition   = "position"
	FieldStartByte  = "start_byte"
	FieldEndByte    = "end_byte"
	FieldKind       = "kind"
	FieldPage       = "page"
	FieldLineStart  = "line_start"
	FieldLineEnd    = "line_end"
	FieldStartMs    = "start_ms"
	FieldEndMs      = "end_ms"
	FieldSection    = "section"
)

func (c Config) schema() vtx.Schema {
	return vtx.Schema{
		Vector: &vtx.VectorSpec{
			Dim:            uint32(c.Dim),
			Metric:         parseMetric(c.Metric),
			Quantized:      c.Quantized,
			M:              uint32(c.M),
			EfConstruction: uint32(c.EfConstruction),
		},
		Fields: []vtx.Field{
			{Name: FieldContent, Type: vtx.TypeText},
			{Name: FieldDocumentID, Type: vtx.TypeKeyword},
			{Name: FieldPath, Type: vtx.TypeKeyword},
			{Name: FieldFilename, Type: vtx.TypeKeyword},
			{Name: FieldExtension, Type: vtx.TypeKeyword},
			{Name: FieldPosition, Type: vtx.TypeInt},
			{Name: FieldStartByte, Type: vtx.TypeInt},
			{Name: FieldEndByte, Type: vtx.TypeInt},
			{Name: FieldKind, Type: vtx.TypeKeyword},
			{Name: FieldPage, Type: vtx.TypeInt},
			{Name: FieldLineStart, Type: vtx.TypeInt},
			{Name: FieldLineEnd, Type: vtx.TypeInt},
			{Name: FieldStartMs, Type: vtx.TypeInt},
			{Name: FieldEndMs, Type: vtx.TypeInt},
			{Name: FieldSection, Type: vtx.TypeKeyword},
		},
		BM25K1: c.BM25K1,
		BM25B:  c.BM25B,
	}
}

func (c Chunk) doc() vtx.Doc {
	return vtx.Doc{
		ID:     c.ID,
		Vector: c.Vector,
		Fields: map[string]vtx.Value{
			FieldContent:    vtx.Text(c.Content),
			FieldDocumentID: vtx.Keyword(c.DocumentID),
			FieldPath:       vtx.Keyword(c.Path),
			FieldFilename:   vtx.Keyword(c.Filename),
			FieldExtension:  vtx.Keyword(c.Extension),
			FieldPosition:   vtx.Int(c.Position),
			FieldStartByte:  vtx.Int(c.StartByte),
			FieldEndByte:    vtx.Int(c.EndByte),
			FieldKind:       vtx.Keyword(c.Kind),
			FieldPage:       vtx.Int(c.Page),
			FieldLineStart:  vtx.Int(c.LineStart),
			FieldLineEnd:    vtx.Int(c.LineEnd),
			FieldStartMs:    vtx.Int(c.StartMs),
			FieldEndMs:      vtx.Int(c.EndMs),
			FieldSection:    vtx.Keyword(truncate(c.Section, 200)),
		},
	}
}

// truncate cuts s to at most n bytes without splitting a UTF-8 sequence.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func chunkOf(r *vtx.Result) Chunk {
	str := func(name string) string {
		switch v := r.Fields[name].(type) {
		case vtx.Text:
			return string(v)
		case vtx.Keyword:
			return string(v)
		}
		return ""
	}
	num := func(name string) int {
		if v, ok := r.Fields[name].(vtx.Int); ok {
			return int(v)
		}
		return 0
	}
	return Chunk{
		ID:         r.ID,
		DocumentID: str(FieldDocumentID),
		Content:    str(FieldContent),
		Path:       str(FieldPath),
		Filename:   str(FieldFilename),
		Extension:  str(FieldExtension),
		Position:   num(FieldPosition),
		StartByte:  num(FieldStartByte),
		EndByte:    num(FieldEndByte),
		Kind:       str(FieldKind),
		Page:       num(FieldPage),
		LineStart:  num(FieldLineStart),
		LineEnd:    num(FieldLineEnd),
		StartMs:    int64(num(FieldStartMs)),
		EndMs:      int64(num(FieldEndMs)),
		Section:    str(FieldSection),
	}
}

var returnFields = []string{FieldContent, FieldDocumentID, FieldPath, FieldFilename,
	FieldExtension, FieldPosition, FieldStartByte, FieldEndByte, FieldKind, FieldPage,
	FieldLineStart, FieldLineEnd, FieldStartMs, FieldEndMs, FieldSection}

// Stats describes the index.
type Stats struct {
	CoreVersion string `json:"core_version"`
	Collection  string `json:"collection"`
	Chunks      int    `json:"chunks"`
	Dim         int    `json:"dim"`
	Metric      string `json:"metric"`
	Quantized   bool   `json:"quantized"`
	DiskBytes   int64  `json:"disk_bytes"`
	WALBytes    uint64 `json:"wal_bytes"`
	LastSeq     uint64 `json:"last_seq"`
	Checkpoint  uint64 `json:"checkpoint_seq"`
	Rebuilding  bool   `json:"rebuilding"`
}

// Engine is safe for concurrent use.
type Engine struct {
	cfg Config
	db  *vtx.DB

	mu       sync.RWMutex // guards active and building
	active   string
	building string // collection being filled by Rebuild, or ""
	gen      int

	rebuildMu sync.Mutex
}

// Open opens the database in cfg.Dir, or creates it. When the stored index
// was built with other parameters (dimension, metric…), a new empty
// generation is started; call Rebuild to fill it.
func Open(cfg Config) (*Engine, error) {
	if cfg.Dim <= 0 {
		return nil, fmt.Errorf("engine: invalid dimension %d", cfg.Dim)
	}
	if cfg.EfSearch == 0 {
		cfg.EfSearch = 64
	}
	cfg.Metric = metricName(parseMetric(cfg.Metric))
	if cfg.Quantized && cfg.Metric != "cosine" {
		log.Printf("engine: int8 quantization requires cosine, using f32 vectors for %s", cfg.Metric)
		cfg.Quantized = false
	}
	db, err := vtx.Open(cfg.Dir, nil)
	if err != nil {
		return nil, fmt.Errorf("engine: open %s: %w", cfg.Dir, err)
	}
	e := &Engine{cfg: cfg, db: db}
	if err := e.init(); err != nil {
		db.Close()
		return nil, err
	}
	return e, nil
}

func (e *Engine) init() error {
	names, err := e.db.Collections()
	if err != nil {
		return err
	}
	if !contains(names, metaCollection) {
		if err := e.db.CreateCollection(metaCollection, vtx.Schema{Fields: []vtx.Field{
			{Name: "active", Type: vtx.TypeKeyword},
			{Name: "config", Type: vtx.TypeKeyword},
		}}); err != nil {
			return fmt.Errorf("engine: create meta: %w", err)
		}
	}
	for _, n := range names {
		if g, ok := generation(n); ok && g > e.gen {
			e.gen = g
		}
	}

	meta, err := e.db.Get(metaCollection, metaID)
	if err != nil {
		return err
	}
	want := e.cfg.fingerprint()
	if meta != nil {
		active, _ := meta.Fields["active"].(vtx.Keyword)
		config, _ := meta.Fields["config"].(vtx.Keyword)
		switch {
		case !contains(names, string(active)):
			log.Printf("engine: active collection %q is missing, starting a new one", active)
		case string(config) != want:
			log.Printf("engine: index was built with %s, configuration asks %s; starting a new one", config, want)
		default:
			e.active = string(active)
		}
	}
	if e.active == "" {
		name, err := e.newGeneration()
		if err != nil {
			return err
		}
		if err := e.setActive(name); err != nil {
			return err
		}
		e.active = name
	}
	// Generations left by an interrupted rebuild or a configuration change.
	for _, n := range names {
		if _, ok := generation(n); ok && n != e.active {
			if err := e.db.DropCollection(n); err != nil {
				log.Printf("engine: drop %s: %v", n, err)
			}
		}
	}
	return nil
}

func generation(name string) (int, bool) {
	if !strings.HasPrefix(name, chunkPrefix) {
		return 0, false
	}
	g, err := strconv.Atoi(name[len(chunkPrefix):])
	return g, err == nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// newGeneration creates the next chunk collection.
func (e *Engine) newGeneration() (string, error) {
	e.gen++
	name := chunkPrefix + strconv.Itoa(e.gen)
	if err := e.db.CreateCollection(name, e.cfg.schema()); err != nil {
		return "", fmt.Errorf("engine: create %s: %w", name, err)
	}
	return name, nil
}

func (e *Engine) setActive(name string) error {
	_, err := e.db.PutDocs(metaCollection, vtx.Doc{ID: metaID, Fields: map[string]vtx.Value{
		"active": vtx.Keyword(name),
		"config": vtx.Keyword(e.cfg.fingerprint()),
	}})
	return err
}

// targets returns the collections a write must reach: the active one and,
// during a rebuild, the one being built. The caller holds e.mu (read), and
// keeps it until the write is done so no collection is dropped under it.
func (e *Engine) targets() []string {
	if e.building != "" {
		return []string{e.active, e.building}
	}
	return []string{e.active}
}

// Dim returns the configured vector dimension.
func (e *Engine) Dim() int { return e.cfg.Dim }

// Similarity turns a vector distance into a score where higher is better.
func (e *Engine) Similarity(dist float32) float32 {
	if e.cfg.Metric == "l2" {
		return 1 / (1 + dist)
	}
	return 1 - dist
}

func (e *Engine) checkDim(c *Chunk) error {
	if len(c.Vector) != e.cfg.Dim {
		return fmt.Errorf("chunk %s: embedding has %d dimensions, index expects %d", c.ID, len(c.Vector), e.cfg.Dim)
	}
	return nil
}

// Put indexes chunks atomically, replacing previous versions with the same id.
func (e *Engine) Put(chunks ...Chunk) error { return e.Replace(nil, chunks) }

// Remove drops chunks atomically; unknown ids are ignored.
func (e *Engine) Remove(ids ...string) error { return e.Replace(ids, nil) }

// Replace removes chunks and puts others in one atomic write, so a search
// sees either the old version of a document or the new one.
func (e *Engine) Replace(remove []string, put []Chunk) error {
	if len(remove) == 0 && len(put) == 0 {
		return nil
	}
	for i := range put {
		if err := e.checkDim(&put[i]); err != nil {
			return err
		}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	targets := e.targets()
	ops := make([]vtx.Op, 0, (len(remove)+len(put))*len(targets))
	for _, t := range targets {
		for _, id := range remove {
			ops = append(ops, vtx.Delete(t, id))
		}
		for i := range put {
			ops = append(ops, vtx.Put(t, put[i].doc()))
		}
	}
	_, err := e.db.Write(ops...)
	return err
}

// Request is a search on the chunks. With both Vector and Text the routes
// are fused with reciprocal rank fusion (k = 60).
type Request struct {
	Vector []float32
	Text   string
	// Exact keeps only chunks whose text contains this literal string
	// (case-sensitive); alone, it lists them in index order.
	Exact  string
	Filter *vtx.Filter
	K      int
	// Candidates taken from each route before fusion (0: engine default).
	Candidates int
}

// Hit is one result with the contribution of each route.
type Hit struct {
	Chunk
	VectorRank     int     // 1-based, 0 when the vector route missed it
	VectorDistance float32 // valid when VectorRank > 0
	TextRank       int     // 1-based, 0 when the text route missed it
	TextScore      float32 // BM25, valid when TextRank > 0
}

// Search runs a vector, text or hybrid search, filter applied inside the
// engine.
func (e *Engine) Search(r Request) ([]Hit, error) {
	if r.K <= 0 {
		r.K = 10
	}
	q := vtx.Query{
		Vector:       r.Vector,
		Text:         r.Text,
		Exact:        r.Exact,
		TextFields:   []string{FieldContent},
		Filter:       r.Filter,
		K:            uint32(r.K),
		Ef:           uint32(max(e.cfg.EfSearch, r.K)),
		Candidates:   uint32(r.Candidates),
		Fusion:       vtx.Fusion{RRFK: 60},
		ReturnFields: returnFields,
		NoPayload:    true,
	}
	if len(r.Vector) > 0 && len(r.Vector) != e.cfg.Dim {
		return nil, fmt.Errorf("query embedding has %d dimensions, index expects %d", len(r.Vector), e.cfg.Dim)
	}
	e.mu.RLock()
	res, err := e.db.Search(e.active, q)
	e.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	hits := make([]Hit, len(res))
	for i := range res {
		hits[i] = Hit{
			Chunk:          chunkOf(&res[i]),
			VectorRank:     int(res[i].VectorRank),
			VectorDistance: res[i].VectorDistance,
			TextRank:       int(res[i].TextRank),
			TextScore:      res[i].TextScore,
		}
	}
	return hits, nil
}

// Get returns one chunk, or nil.
func (e *Engine) Get(id string) (*Chunk, error) {
	e.mu.RLock()
	r, err := e.db.Get(e.active, id)
	e.mu.RUnlock()
	if err != nil || r == nil {
		return nil, err
	}
	c := chunkOf(r)
	return &c, nil
}

// Len returns the number of chunks in the active collection.
func (e *Engine) Len() int {
	e.mu.RLock()
	n, err := e.db.Count(e.active)
	e.mu.RUnlock()
	if err != nil {
		return 0
	}
	return int(n)
}

// Stats describes the index.
func (e *Engine) Stats() Stats {
	e.mu.RLock()
	s := Stats{
		CoreVersion: vtx.Version(),
		Collection:  e.active,
		Dim:         e.cfg.Dim,
		Metric:      e.cfg.Metric,
		Quantized:   e.cfg.Quantized,
		Rebuilding:  e.building != "",
	}
	e.mu.RUnlock()
	s.Chunks = e.Len()
	if ds, err := e.db.Stats(); err == nil {
		s.WALBytes, s.LastSeq, s.Checkpoint = ds.WALBytes, ds.LastSeq, ds.CheckpointSeq
	}
	filepath.WalkDir(e.cfg.Dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				s.DiskBytes += fi.Size()
			}
		}
		return nil
	})
	return s
}

// Compact rewrites the active collection without deleted chunks, then
// checkpoints so the log is trimmed.
func (e *Engine) Compact() error {
	e.mu.RLock()
	err := e.db.Compact(e.active)
	e.mu.RUnlock()
	if err != nil {
		return err
	}
	return e.db.Checkpoint()
}

// Checkpoint writes snapshots and trims the log. Writes are already durable
// when they return; this only shortens the next open.
func (e *Engine) Checkpoint() error { return e.db.Checkpoint() }

// ErrRebuilding is returned when a rebuild is already running.
var ErrRebuilding = errors.New("engine: rebuild already running")

// Rebuild fills a new generation from source, then makes it the active one.
// Searches keep using the current generation until the switch; writes made
// meanwhile reach both.
func (e *Engine) Rebuild(source func(yield func(Chunk) error) error) error {
	if !e.rebuildMu.TryLock() {
		return ErrRebuilding
	}
	defer e.rebuildMu.Unlock()

	e.mu.Lock()
	name, err := e.newGeneration()
	if err != nil {
		e.mu.Unlock()
		return err
	}
	e.building = name
	e.mu.Unlock()

	abort := func(err error) error {
		e.mu.Lock()
		e.building = ""
		e.mu.Unlock()
		if derr := e.db.DropCollection(name); derr != nil {
			log.Printf("engine: drop %s: %v", name, derr)
		}
		return err
	}

	batch := make([]vtx.Doc, 0, rebuildBatch)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		_, err := e.db.PutDocs(name, batch...)
		batch = batch[:0]
		return err
	}
	err = source(func(c Chunk) error {
		if err := e.checkDim(&c); err != nil {
			return err
		}
		batch = append(batch, c.doc())
		if len(batch) == rebuildBatch {
			return flush()
		}
		return nil
	})
	if err == nil {
		err = flush()
	}
	if err != nil {
		return abort(err)
	}

	e.mu.Lock()
	if err := e.setActive(name); err != nil {
		e.mu.Unlock()
		return abort(err)
	}
	old := e.active
	e.active, e.building = name, ""
	e.mu.Unlock()
	if err := e.db.DropCollection(old); err != nil {
		log.Printf("engine: drop %s: %v", old, err)
	}
	return e.db.Checkpoint()
}

// DB returns the underlying database, for other collections that live next
// to the chunks (the problem-solution memory). Collections named chunks_*
// and lorag_meta belong to the engine.
func (e *Engine) DB() *vtx.DB { return e.db }

// MetaCollection holds small pointer documents (active generation and the
// configuration it was built with), keyed by the subsystem name.
const MetaCollection = metaCollection

// Close makes every write durable and releases the database.
func (e *Engine) Close() error { return e.db.Close() }
