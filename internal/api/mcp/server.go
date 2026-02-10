package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/lorag/lorag/internal/core/indexer"
	"github.com/lorag/lorag/internal/core/query"
	"github.com/lorag/lorag/internal/core/store"
)

// Server implements MCP (Model Context Protocol) server
type Server struct {
	store         *store.BoltStore
	settingsStore *store.SettingsStore
	indexer       *indexer.Indexer
	query         *query.Engine
}

// NewServer creates a new MCP server
func NewServer(
	st *store.BoltStore,
	settings *store.SettingsStore,
	idx *indexer.Indexer,
	qe *query.Engine,
) *Server {
	return &Server{
		store:         st,
		settingsStore: settings,
		indexer:       idx,
		query:         qe,
	}
}

// MCPRequest represents an MCP request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
	ID      interface{} `json:"id,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPInfo contains server capabilities
type MCPInfo struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Capabilities []string          `json:"capabilities"`
	Tools       []MCPTool         `json:"tools,omitempty"`
}

// MCPTool represents an available tool
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Handle handles MCP requests (both GET for SSE and POST for JSON-RPC)
func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	// Handle GET request for SSE (Streamable HTTP)
	if r.Method == "GET" {
		s.handleSSE(w, r)
		return
	}
	
	// Handle POST request for JSON-RPC
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, nil, -32700, "Parse error")
		return
	}
	
	if req.JSONRPC != "2.0" {
		s.respondError(w, req.ID, -32600, "Invalid Request: must be jsonrpc 2.0")
		return
	}
	
	var result interface{}
	var mcpErr *MCPError
	
	switch req.Method {
	case "initialize":
		result, mcpErr = s.handleInitialize(r.Context(), req.Params)
	case "tools/list":
		result, mcpErr = s.handleToolsList(r.Context())
	case "tools/call":
		result, mcpErr = s.handleToolsCall(r.Context(), req.Params)
	case "resources/search":
		result, mcpErr = s.handleResourcesSearch(r.Context(), req.Params)
	case "resources/get":
		result, mcpErr = s.handleResourcesGet(r.Context(), req.Params)
	default:
		s.respondError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
		return
	}
	
	if mcpErr != nil {
		s.respondError(w, req.ID, mcpErr.Code, mcpErr.Message)
		return
	}
	
	s.respond(w, req.ID, result)
}

// handleSSE handles Server-Sent Events for Streamable HTTP transport
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// For now, just return a simple SSE stream that stays open
	// In a full implementation, this would maintain a connection and send notifications
	w.WriteHeader(http.StatusOK)
	
	// Send a ping comment to establish connection
	fmt.Fprint(w, ": ping\n\n")
	
	// Keep connection open but don't block - client will reconnect if needed
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// InitializeResult represents the response to initialize
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

// handleInitialize handles the initialize method
func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (interface{}, *MCPError) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{},
		},
	}
	result.ServerInfo.Name = "lorag"
	result.ServerInfo.Version = "0.1.0"
	
	return result, nil
}

// handleToolsList returns available tools
func (s *Server) handleToolsList(ctx context.Context) (interface{}, *MCPError) {
	return map[string]interface{}{
		"tools": []MCPTool{
			{
				Name:        "search",
				Description: "Search through indexed documents using semantic search",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"top_k":{"type":"integer","default":5}},"required":["query"]}`),
			},
			{
				Name:        "index_file",
				Description: "Index a file or directory",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"recursive":{"type":"boolean","default":false}},"required":["path"]}`),
			},
			{
				Name:        "get_stats",
				Description: "Get indexing statistics",
				InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
	}, nil
}

// handleToolsCall handles tool invocations
func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (interface{}, *MCPError) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &MCPError{Code: -32602, Message: "Invalid params"}
	}
	
	switch call.Name {
	case "search":
		return s.handleToolSearch(ctx, call.Arguments)
	case "get_document":
		return s.handleToolGetDocument(ctx, call.Arguments)
	case "index_file":
		return s.handleToolIndexFile(ctx, call.Arguments)
	case "get_stats":
		return s.handleToolGetStats(ctx)
	default:
		return nil, &MCPError{Code: -32601, Message: fmt.Sprintf("Unknown tool: %s", call.Name)}
	}
}

// defaultTemplates contains built-in prompt templates
var defaultTemplates = map[string]string{
	"default": `Found {{count}} results for: {{query}}

{{results}}

Based on these documents, please answer the question: {{query}}`,

	"concise": `Results for: {{query}}

{{results}}

Provide a brief 2-3 sentence answer.`,

	"detailed": `Search Results for: {{query}}

{{results}}

Please provide a comprehensive answer citing the sources [1], [2], etc.`,

	"academic": `Sources found for: {{query}}

{{results}}

Provide a formal academic response with proper structure and citations.`,

	"bullet": `Results for: {{query}}

{{results}}

Answer using bullet points for clarity.`,
}

// handleToolSearch handles the search tool
func (s *Server) handleToolSearch(ctx context.Context, args json.RawMessage) (interface{}, *MCPError) {
	var params struct {
		Query    string `json:"query"`
		TopK     int    `json:"top_k"`
		Template string `json:"template,omitempty"`
	}
	
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, &MCPError{Code: -32602, Message: "Invalid arguments"}
	}
	
	if params.TopK <= 0 {
		params.TopK = 5
	}
	
	// Use user's default template if none specified
	if params.Template == "" {
		settings, _ := s.settingsStore.GetAppSettings()
		if settings != nil && settings.DefaultTemplate != "" {
			params.Template = settings.DefaultTemplate
		} else {
			params.Template = "default"
		}
	}
	
	q := query.Query{
		Text: params.Query,
		TopK: params.TopK,
	}
	
	results, err := s.query.Search(ctx, q)
	if err != nil {
		return nil, &MCPError{Code: -32000, Message: err.Error()}
	}
	
	if len(results) == 0 {
		return map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "No results found for query: " + params.Query},
			},
		}, nil
	}
	
	// Format results list
	var resultsList strings.Builder
	for i, r := range results {
		resultsList.WriteString(fmt.Sprintf("[%d] Source: %s (relevance: %.2f)\n%s\n\n", 
			i+1, r.Source.Filename, r.Score, r.Content))
	}
	
	// Apply template
	template, ok := defaultTemplates[params.Template]
	if !ok {
		template = defaultTemplates["default"]
	}
	
	output := strings.ReplaceAll(template, "{{query}}", params.Query)
	output = strings.ReplaceAll(output, "{{count}}", fmt.Sprintf("%d", len(results)))
	output = strings.ReplaceAll(output, "{{results}}", resultsList.String())
	
	return map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": output},
		},
	}, nil
}

// handleToolGetDocument handles the get_document tool
func (s *Server) handleToolGetDocument(ctx context.Context, args json.RawMessage) (interface{}, *MCPError) {
	var params struct {
		DocumentID string `json:"document_id"`
	}
	
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, &MCPError{Code: -32602, Message: "Invalid arguments"}
	}
	
	// Get document
	doc, err := s.store.GetDocument(params.DocumentID)
	if err != nil {
		return nil, &MCPError{Code: -32000, Message: fmt.Sprintf("Document not found: %s", params.DocumentID)}
	}
	
	// Get chunks
	chunks, err := s.store.GetChunksByDocument(params.DocumentID)
	if err != nil {
		return nil, &MCPError{Code: -32000, Message: "Failed to get chunks"}
	}
	
	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Document: %s\n", doc.Filename))
	output.WriteString(fmt.Sprintf("Path: %s\n", doc.Path))
	output.WriteString(fmt.Sprintf("Indexed: %s\n", doc.IndexedAt.Format("2006-01-02 15:04")))
	output.WriteString(fmt.Sprintf("Chunks: %d\n\n", len(chunks)))
	
	output.WriteString("=== CONTENT CHUNKS ===\n\n")
	for i, chunk := range chunks {
		output.WriteString(fmt.Sprintf("--- Chunk %d (position %d) ---\n", i+1, chunk.Position))
		output.WriteString(chunk.Content)
		output.WriteString("\n\n")
	}
	
	return map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": output.String()},
		},
	}, nil
}

// handleToolIndexFile handles the index_file tool
func (s *Server) handleToolIndexFile(ctx context.Context, args json.RawMessage) (interface{}, *MCPError) {
	var params struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, &MCPError{Code: -32602, Message: "Invalid arguments"}
	}
	
	result, err := s.indexer.IndexDirectory(ctx, params.Path, params.Recursive)
	if err != nil {
		return nil, &MCPError{Code: -32000, Message: err.Error()}
	}
	
	output := result.String()
	
	return map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": output},
		},
	}, nil
}

// handleToolGetStats handles the get_stats tool
func (s *Server) handleToolGetStats(ctx context.Context) (interface{}, *MCPError) {
	stats := s.indexer.GetStats()
	
	output := fmt.Sprintf("Indexing Statistics:\n\n")
	for k, v := range stats {
		output += fmt.Sprintf("%s: %v\n", k, v)
	}
	
	return map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": output},
		},
	}, nil
}

// handleResourcesSearch handles resource search
func (s *Server) handleResourcesSearch(ctx context.Context, params json.RawMessage) (interface{}, *MCPError) {
	var searchParams struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	
	if err := json.Unmarshal(params, &searchParams); err != nil {
		return nil, &MCPError{Code: -32602, Message: "Invalid params"}
	}
	
	if searchParams.Limit <= 0 {
		searchParams.Limit = 10
	}
	
	q := query.Query{
		Text: searchParams.Query,
		TopK: searchParams.Limit,
	}
	
	results, err := s.query.Search(ctx, q)
	if err != nil {
		return nil, &MCPError{Code: -32000, Message: err.Error()}
	}
	
	resources := make([]map[string]interface{}, len(results))
	for i, r := range results {
		resources[i] = map[string]interface{}{
			"uri":         fmt.Sprintf("file://%s#chunk=%s", r.Source.Path, r.ChunkID),
			"name":        r.Source.Filename,
			"description": fmt.Sprintf("Chunk %d from %s (score: %.3f)", r.Source.Position, r.Source.Filename, r.Score),
			"mimeType":    "text/plain",
		}
	}
	
	return map[string]interface{}{
		"resources": resources,
	}, nil
}

// handleResourcesGet handles resource retrieval
func (s *Server) handleResourcesGet(ctx context.Context, params json.RawMessage) (interface{}, *MCPError) {
	var getParams struct {
		URI string `json:"uri"`
	}
	
	if err := json.Unmarshal(params, &getParams); err != nil {
		return nil, &MCPError{Code: -32602, Message: "Invalid params"}
	}
	
	// Parse chunk ID from URI
	// Format: file://path#chunk=chunkID
	// This is a simplified implementation
	
	return map[string]interface{}{
		"contents": []map[string]string{
			{"uri": getParams.URI, "mimeType": "text/plain", "text": "Resource content"},
		},
	}, nil
}

// respond sends a successful response
func (s *Server) respond(w http.ResponseWriter, id interface{}, result interface{}) {
	response := MCPResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// respondError sends an error response
func (s *Server) respondError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := MCPResponse{
		JSONRPC: "2.0",
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
