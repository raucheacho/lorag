# MCP (Model Context Protocol)

Lorag serves MCP over the Streamable HTTP transport, so agents can search the indexed local files and add new ones.

## Endpoint

```
POST   http://localhost:8080/mcp     JSON-RPC 2.0 request, JSON response
DELETE http://localhost:8080/mcp     ends the session named by Mcp-Session-Id
```

- Protocol revisions: `2025-06-18`, `2025-03-26`, `2024-11-05`. The server answers `initialize` with the client's revision when it supports it, the newest one otherwise.
- `initialize` returns an `Mcp-Session-Id` header. Clients send it back on each request; Lorag uses it only to attribute searches to the client (`clientInfo.name`) in the activity log.
- Notifications (no `id`, e.g. `notifications/initialized`) get `202 Accepted` with no body.
- `GET` returns `405`: the server never pushes messages.
- Requests carrying an `Origin` header other than `localhost`, `127.0.0.1` or `::1` are refused (`403`), so a web page cannot drive the server from the browser.

Methods: `initialize`, `ping`, `tools/list`, `tools/call`.

## Connecting a client

Clients that support HTTP servers take the URL directly:

```json
{
  "mcpServers": {
    "lorag": { "type": "http", "url": "http://localhost:8080/mcp" }
  }
}
```

Clients limited to stdio servers can go through a bridge such as `npx mcp-remote http://localhost:8080/mcp`.

## Tools

### `search`

Hybrid search by default: the query is embedded and matched against the chunk vectors, and its words against the chunk text (BM25); both rankings are fused with reciprocal rank fusion inside VectixDB.

| Argument | Type | Default | |
| --- | --- | --- | --- |
| `query` | string | required | natural language or exact terms |
| `top_k` | integer | 5 | 1 to 50 |
| `mode` | string | `hybrid` | `hybrid`, `vector` or `keyword` |
| `extension` | string | | only files with this extension (`md` or `.md`) |
| `path` | string | | only the file with this exact path |
| `context` | integer | 0 | neighbor chunks added on each side, 0 to 3 |

`keyword` mode finds exact identifiers (error codes, function names) that embeddings blur. If the embedding service is down, `hybrid` falls back to `keyword` and says so in a note.

The result is text, one block per chunk:

```
Found 2 results for: registry secret

[1] /home/me/notes/deploy.md (chunk 3, score 0.984, document_id 26c5…)
Kubernetes deployment rollout fails with ImagePullBackOff when …
```

Hybrid scores are normalized to [0, 1]: 1 means ranked first by both routes. The framing text follows the template chosen in the settings (`default`, `concise`, `detailed`, `academic`, `bullet`).

### `get_document`

Full indexed text of a document, chunk by chunk. Arguments: `document_id` (from a search result) or `path`.

### `index_file`

Indexes a file, or every supported file of a directory. Arguments: `path`, `recursive` (default `false`). An unchanged file is skipped; a changed file has its chunks replaced in one atomic write, so a search never sees half of it.

### `get_stats`

Document and chunk counts and the state of the search index, as JSON.

## Errors

- A tool that fails (missing argument, unreadable file, search error) returns a normal result with `"isError": true` and the message as text, so the agent can read it and retry.
- Protocol errors use JSON-RPC codes: `-32700` parse error, `-32600` invalid request, `-32601` unknown method, `-32602` invalid params or unknown tool.

## Example

```sh
curl -s localhost:8080/mcp -H 'Content-Type: application/json' -d '{
  "jsonrpc": "2.0", "id": 1, "method": "tools/call",
  "params": {"name": "search", "arguments": {"query": "ImagePullBackOff", "mode": "keyword"}}
}'
```
