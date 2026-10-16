package mcp

import (
	"bytes"
	"encoding/json"
	"hash/fnv"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorag/lorag/internal/activity"
	"github.com/lorag/lorag/internal/core/engine"
	"github.com/lorag/lorag/internal/core/indexer"
	"github.com/lorag/lorag/internal/core/query"
	"github.com/lorag/lorag/internal/core/store"
)

const dim = 32

// wordEmbedder hashes words into a fixed vector: texts sharing words are
// close, which is all the tests need.
type wordEmbedder struct{}

func (wordEmbedder) Embed(text string) ([]float32, error) {
	v := make([]float32, dim)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		h := fnv.New32a()
		h.Write([]byte(strings.Trim(w, ".,;:!?")))
		v[h.Sum32()%dim]++
	}
	var n float64
	for _, x := range v {
		n += float64(x * x)
	}
	if n > 0 {
		for i := range v {
			v[i] /= float32(math.Sqrt(n))
		}
	}
	return v, nil
}

func (e wordEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i], _ = e.Embed(t)
	}
	return out, nil
}

func (wordEmbedder) Dimension() int { return dim }

type fixture struct {
	t     *testing.T
	srv   *httptest.Server
	dir   string
	log   *activity.Log
	sess  string
	reqID int
}

func newFixture(t *testing.T) *fixture {
	data := t.TempDir()
	st, err := store.NewBoltStore(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	settings := store.NewSettingsStore(st)
	eng, err := engine.Open(engine.Config{Dir: filepath.Join(data, "vectix"), Dim: dim})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Close() })
	emb := wordEmbedder{}
	idx := indexer.NewIndexer(st, settings, eng, emb, indexer.Config{MaxChunks: 1000, ChunkSize: 400, ChunkOverlap: 0})
	log := activity.NewLog(100)
	qe := query.NewEngine(st, eng, emb, log)
	srv := httptest.NewServer(http.HandlerFunc(NewServer(st, settings, idx, qe, "test").Handle))
	t.Cleanup(srv.Close)
	return &fixture{t: t, srv: srv, dir: t.TempDir(), log: log}
}

func (f *fixture) post(body any, header map[string]string) *http.Response {
	f.t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, f.srv.URL, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if f.sess != "" {
		req.Header.Set("Mcp-Session-Id", f.sess)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp
}

// rpc sends a request and returns its result, failing on a protocol error.
func (f *fixture) rpc(method string, params any) map[string]any {
	f.t.Helper()
	f.reqID++
	resp := f.post(map[string]any{"jsonrpc": "2.0", "id": f.reqID, "method": method, "params": params}, nil)
	defer resp.Body.Close()
	var out struct {
		Result map[string]any `json:"result"`
		Error  *rpcError      `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		f.t.Fatal(err)
	}
	if out.Error != nil {
		f.t.Fatalf("%s: error %d %s", method, out.Error.Code, out.Error.Message)
	}
	if s := resp.Header.Get("Mcp-Session-Id"); s != "" {
		f.sess = s
	}
	return out.Result
}

// call runs a tool and returns its text and isError.
func (f *fixture) call(name string, args map[string]any) (string, bool) {
	f.t.Helper()
	r := f.rpc("tools/call", map[string]any{"name": name, "arguments": args})
	content := r["content"].([]any)
	isErr, _ := r["isError"].(bool)
	return content[0].(map[string]any)["text"].(string), isErr
}

func (f *fixture) write(name, text string) string {
	p := filepath.Join(f.dir, name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func TestProtocol(t *testing.T) {
	f := newFixture(t)
	init := f.rpc("initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"clientInfo":      map[string]string{"name": "test-agent", "version": "1"},
	})
	if init["protocolVersion"] != "2025-03-26" || f.sess == "" {
		t.Fatalf("initialize: %v, session %q", init, f.sess)
	}
	if v := f.rpc("initialize", map[string]any{"protocolVersion": "1999-01-01"})["protocolVersion"]; v != protocolVersions[0] {
		t.Fatalf("unknown version answered with %v", v)
	}

	resp := f.post(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("notification: status %d", resp.StatusCode)
	}

	tools := f.rpc("tools/list", nil)["tools"].([]any)
	if len(tools) != 4 {
		t.Fatalf("tools: %v", tools)
	}

	resp = f.post(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"}, map[string]string{"Origin": "http://evil.example"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin: status %d", resp.StatusCode)
	}
	resp = f.post(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"}, map[string]string{"Origin": "http://localhost:5173"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("local origin: status %d", resp.StatusCode)
	}

	get, _ := http.Get(f.srv.URL)
	if get.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET: status %d", get.StatusCode)
	}

	if text, isErr := f.call("search", map[string]any{"query": "  "}); !isErr || !strings.Contains(text, "required") {
		t.Fatalf("empty query: %q %v", text, isErr)
	}
}

func TestIndexSearchReindex(t *testing.T) {
	f := newFixture(t)
	f.rpc("initialize", map[string]any{"clientInfo": map[string]string{"name": "test-agent"}})
	a := f.write("deploy.md", "Kubernetes deployment rollout fails with ImagePullBackOff when the registry secret is missing.")
	f.write("cooking.md", "Bake the bread at high temperature until the crust is golden.")
	f.write("notes.txt", "Meeting notes: the registry migration happens next week.")

	if text, isErr := f.call("index_file", map[string]any{"path": f.dir}); isErr {
		t.Fatalf("index: %s", text)
	}

	text, isErr := f.call("search", map[string]any{"query": "kubernetes rollout ImagePullBackOff", "top_k": 2})
	if isErr || !strings.HasPrefix(strings.SplitN(text, "\n\n", 2)[1], "[1] "+a) {
		t.Fatalf("search: %q", text)
	}

	// Exact identifiers are found by the keyword route.
	text, _ = f.call("search", map[string]any{"query": "ImagePullBackOff", "mode": "keyword"})
	if !strings.Contains(text, "deploy.md") || strings.Contains(text, "cooking.md") {
		t.Fatalf("keyword search: %q", text)
	}

	// The extension filter runs inside the engine.
	text, _ = f.call("search", map[string]any{"query": "registry", "extension": "txt"})
	if !strings.Contains(text, "notes.txt") || strings.Contains(text, "deploy.md") {
		t.Fatalf("filtered search: %q", text)
	}

	// The search is attributed to the MCP client.
	q := f.log.List(activity.Filter{})
	if len(q) == 0 || q[0].Source != activity.SourceMCP || q[0].Client != "test-agent" {
		t.Fatalf("activity: %+v", q)
	}

	// get_document by path.
	text, isErr = f.call("get_document", map[string]any{"path": a})
	if isErr || !strings.Contains(text, "registry secret") {
		t.Fatalf("get_document: %q", text)
	}

	// Reindexing a changed file replaces its chunks.
	f.write("deploy.md", "Terraform plan shows drift on the load balancer.")
	if text, isErr := f.call("index_file", map[string]any{"path": a}); isErr {
		t.Fatalf("reindex: %s", text)
	}
	text, _ = f.call("search", map[string]any{"query": "ImagePullBackOff", "mode": "keyword"})
	if strings.Contains(text, "deploy.md") {
		t.Fatalf("stale chunk after reindex: %q", text)
	}
	text, _ = f.call("search", map[string]any{"query": "terraform drift", "mode": "keyword"})
	if !strings.Contains(text, "deploy.md") {
		t.Fatalf("new chunk missing: %q", text)
	}

	stats, _ := f.call("get_stats", nil)
	if !strings.Contains(stats, `"documents": 3`) {
		t.Fatalf("stats: %s", stats)
	}
}
