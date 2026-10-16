// Package mcp serves the Model Context Protocol over Streamable HTTP
// (POST /mcp, JSON responses), so agents can search and feed the index.
package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/lorag/lorag/internal/activity"
	"github.com/lorag/lorag/internal/core/indexer"
	"github.com/lorag/lorag/internal/core/query"
	"github.com/lorag/lorag/internal/core/store"
)

// Protocol revisions this server speaks, newest first.
var protocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const maxSessions = 1024

// Server implements the MCP tools on top of the indexer and the query engine.
type Server struct {
	store    *store.BoltStore
	settings *store.SettingsStore
	indexer  *indexer.Indexer
	query    *query.Engine
	version  string

	mu       sync.Mutex
	sessions map[string]string // session id → client name
}

// NewServer creates the MCP server.
func NewServer(st *store.BoltStore, settings *store.SettingsStore, idx *indexer.Indexer, qe *query.Engine, version string) *Server {
	return &Server{store: st, settings: settings, indexer: idx, query: qe, version: version, sessions: map[string]string{}}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"` // absent for notifications
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// Handle serves /mcp.
func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	if !allowedOrigin(r.Header.Get("Origin")) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodPost:
	case http.MethodDelete:
		s.mu.Lock()
		delete(s.sessions, r.Header.Get("Mcp-Session-Id"))
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		// No server-initiated stream: GET is not offered.
		w.Header().Set("Allow", "POST, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body := http.MaxBytesReader(w, r.Body, 4<<20)
	var req request
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		writeJSON(w, response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, "parse error"}})
		return
	}
	if len(req.ID) == 0 {
		// Notification (initialized, cancelled…) or a response: nothing to return.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{codeInvalidRequest, "invalid request"}})
		return
	}

	var result any
	var rerr *rpcError
	switch req.Method {
	case "initialize":
		result, rerr = s.initialize(w, req.Params)
	case "ping":
		result = struct{}{}
	case "tools/list":
		result = map[string]any{"tools": tools}
	case "tools/call":
		result, rerr = s.callTool(r.Context(), s.client(r), req.Params)
	default:
		rerr = &rpcError{codeMethodNotFound, "method not found: " + req.Method}
	}
	if rerr != nil {
		writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Error: rerr})
		return
	}
	writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Result: result})
}

// allowedOrigin rejects browser requests from other sites (DNS rebinding);
// requests without an Origin come from local tools.
func allowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (s *Server) initialize(w http.ResponseWriter, params json.RawMessage) (any, *rpcError) {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
		ClientInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{codeInvalidParams, "invalid params"}
		}
	}
	version := protocolVersions[0]
	for _, v := range protocolVersions {
		if v == p.ProtocolVersion {
			version = v
		}
	}

	var b [16]byte
	rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	s.mu.Lock()
	if len(s.sessions) >= maxSessions {
		clear(s.sessions)
	}
	s.sessions[id] = p.ClientInfo.Name
	s.mu.Unlock()
	w.Header().Set("Mcp-Session-Id", id)

	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]string{"name": "lorag", "version": s.version},
		"instructions": "Lorag searches the user's local files. Use search first; " +
			"use get_document to read a whole file from a result's document_id.",
	}, nil
}

// client names the agent behind a request, for the activity log.
func (s *Server) client(r *http.Request) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[r.Header.Get("Mcp-Session-Id")]
}

type tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Annotations map[string]bool `json:"annotations,omitempty"`
}

var tools = []tool{
	{
		Name:  "search",
		Title: "Search local files",
		Description: "Search the indexed local files. Hybrid by default: meaning (embeddings) and exact words (BM25) fused. " +
			"Each result gives its source path, document_id and chunk text.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` +
			`"query":{"type":"string","description":"What to look for, in natural language or exact terms"},` +
			`"top_k":{"type":"integer","minimum":1,"maximum":50,"default":5},` +
			`"mode":{"type":"string","enum":["hybrid","vector","keyword"],"default":"hybrid"},` +
			`"extension":{"type":"string","description":"Only files with this extension, e.g. .md"},` +
			`"path":{"type":"string","description":"Only the file with this exact path"},` +
			`"context":{"type":"integer","minimum":0,"maximum":3,"default":0,"description":"Neighbor chunks to include on each side"}` +
			`},"required":["query"]}`),
		Annotations: map[string]bool{"readOnlyHint": true},
	},
	{
		Name:        "get_document",
		Title:       "Read an indexed document",
		Description: "Return the full indexed text of a document, by document_id (from search) or path.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` +
			`"document_id":{"type":"string"},"path":{"type":"string"}}}`),
		Annotations: map[string]bool{"readOnlyHint": true},
	},
	{
		Name:        "index_file",
		Title:       "Index a file or directory",
		Description: "Index a file, or every supported file of a directory, so it becomes searchable.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string"},"recursive":{"type":"boolean","default":false}},"required":["path"]}`),
		Annotations: map[string]bool{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true},
	},
	{
		Name:        "get_stats",
		Title:       "Index statistics",
		Description: "Number of documents and chunks indexed, and the state of the search index.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		Annotations: map[string]bool{"readOnlyHint": true},
	},
}

// toolResult is the content of a tools/call result. Tool failures are
// reported here (isError) so the agent can read them, not as protocol errors.
func toolResult(text string, isError bool) map[string]any {
	r := map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}
	if isError {
		r["isError"] = true
	}
	return r
}

var errArgs = errors.New("invalid arguments")

func (s *Server) callTool(ctx context.Context, client string, params json.RawMessage) (any, *rpcError) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &rpcError{codeInvalidParams, "invalid params"}
	}
	if len(call.Arguments) == 0 {
		call.Arguments = json.RawMessage("{}")
	}
	var text string
	var err error
	switch call.Name {
	case "search":
		text, err = s.search(ctx, client, call.Arguments)
	case "get_document":
		text, err = s.getDocument(call.Arguments)
	case "index_file":
		text, err = s.index(ctx, call.Arguments)
	case "get_stats":
		text, err = s.stats()
	default:
		return nil, &rpcError{codeInvalidParams, "unknown tool: " + call.Name}
	}
	if err != nil {
		return toolResult(err.Error(), true), nil
	}
	return toolResult(text, false), nil
}

func decode(args json.RawMessage, v any) error {
	d := json.NewDecoder(bytes.NewReader(args))
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("%w: %v", errArgs, err)
	}
	return nil
}

// templates frame the results; the default one is chosen in the settings.
var templates = map[string]string{
	"default":  "Found {{count}} results for: {{query}}\n\n{{results}}",
	"concise":  "Results for: {{query}}\n\n{{results}}\nProvide a brief 2-3 sentence answer.",
	"detailed": "Search results for: {{query}}\n\n{{results}}\nGive a comprehensive answer citing the sources [1], [2], etc.",
	"academic": "Sources found for: {{query}}\n\n{{results}}\nGive a formal answer with proper structure and citations.",
	"bullet":   "Results for: {{query}}\n\n{{results}}\nAnswer using bullet points.",
}

func (s *Server) search(ctx context.Context, client string, args json.RawMessage) (string, error) {
	var p struct {
		Query     string `json:"query"`
		TopK      int    `json:"top_k"`
		Mode      string `json:"mode"`
		Extension string `json:"extension"`
		Path      string `json:"path"`
		Context   int    `json:"context"`
		Template  string `json:"template"`
	}
	if err := decode(args, &p); err != nil {
		return "", err
	}
	if strings.TrimSpace(p.Query) == "" {
		return "", errors.New("query is required")
	}
	p.TopK = min(max(p.TopK, 0), 50)
	if p.TopK == 0 {
		p.TopK = 5
	}
	q := query.Query{
		Text: p.Query, TopK: p.TopK, Mode: query.Mode(p.Mode),
		Source: activity.SourceMCP, Client: client, Tool: "search",
	}
	if p.Extension != "" {
		ext := strings.ToLower(p.Extension)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		q.Filters = append(q.Filters, query.Filter{Field: "extension", Operator: "eq", Value: ext})
	}
	if p.Path != "" {
		q.Filters = append(q.Filters, query.Filter{Field: "path", Operator: "eq", Value: p.Path})
	}
	resp, err := s.query.Run(ctx, q)
	if err != nil {
		return "", err
	}
	results := s.query.WithContext(resp.Results, min(max(p.Context, 0), 3))
	if len(results) == 0 {
		return "No results for: " + p.Query, nil
	}

	var b strings.Builder
	for _, w := range resp.Warnings {
		fmt.Fprintf(&b, "Note: %s\n\n", w)
	}
	for i, r := range results {
		fmt.Fprintf(&b, "[%d] %s (chunk %d, score %.3f, document_id %s)\n%s\n\n",
			i+1, r.Source.Path, r.Source.Position, r.Score, r.DocumentID, r.Content)
	}

	name := p.Template
	if name == "" {
		if st, err := s.settings.GetAppSettings(); err == nil && st.DefaultTemplate != "" {
			name = st.DefaultTemplate
		}
	}
	tmpl, ok := templates[name]
	if !ok {
		tmpl = templates["default"]
	}
	out := strings.ReplaceAll(tmpl, "{{query}}", p.Query)
	out = strings.ReplaceAll(out, "{{count}}", fmt.Sprint(len(results)))
	return strings.ReplaceAll(out, "{{results}}", b.String()), nil
}

func (s *Server) getDocument(args json.RawMessage) (string, error) {
	var p struct {
		DocumentID string `json:"document_id"`
		Path       string `json:"path"`
	}
	if err := decode(args, &p); err != nil {
		return "", err
	}
	var doc *store.Document
	var err error
	switch {
	case p.DocumentID != "":
		doc, err = s.store.GetDocument(p.DocumentID)
	case p.Path != "":
		doc, err = s.store.GetDocumentByPath(p.Path)
	default:
		return "", errors.New("document_id or path is required")
	}
	if err != nil || doc == nil {
		return "", errors.New("document not found")
	}
	chunks, err := s.store.GetChunksByDocument(doc.ID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Document: %s\nPath: %s\nChunks: %d\n", doc.Filename, doc.Path, len(chunks))
	if doc.IndexedAt != nil {
		fmt.Fprintf(&b, "Indexed: %s\n", doc.IndexedAt.Format("2006-01-02 15:04"))
	}
	for _, c := range chunks {
		fmt.Fprintf(&b, "\n--- chunk %d ---\n%s\n", c.Position, c.Content)
	}
	return b.String(), nil
}

func (s *Server) index(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := decode(args, &p); err != nil {
		return "", err
	}
	info, err := os.Stat(p.Path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		doc, err := s.indexer.IndexFile(ctx, p.Path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Indexed %s: %d chunks (document_id %s)", doc.Path, doc.ChunkCount, doc.ID), nil
	}
	result, err := s.indexer.IndexDirectory(ctx, p.Path, p.Recursive)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func (s *Server) stats() (string, error) {
	st := s.indexer.GetStats()
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
