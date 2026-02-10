# MCP (Model Context Protocol) Documentation

## Overview

Lorag implements the Model Context Protocol (MCP) to allow AI agents to interact with the local RAG engine. MCP is a standard protocol for AI agents to discover and use tools.

## Endpoint

```
POST /mcp
Content-Type: application/json
```

## Protocol Format

All MCP requests and responses use JSON-RPC 2.0 format.

### Request Format

```json
{
  "jsonrpc": "2.0",
  "method": "method.name",
  "params": { ... },
  "id": 1
}
```

### Response Format

```json
{
  "jsonrpc": "2.0",
  "result": { ... },
  "id": 1
}
```

### Error Format

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32600,
    "message": "Invalid Request"
  },
  "id": 1
}
```

## Methods

### Initialize

Get server information and capabilities.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "name": "lorag",
    "version": "0.1.0",
    "capabilities": ["tools", "resources", "search"],
    "tools": [
      {
        "name": "search",
        "description": "Search through indexed documents",
        "inputSchema": {
          "type": "object",
          "properties": {
            "query": {"type": "string"},
            "top_k": {"type": "integer", "default": 5}
          },
          "required": ["query"]
        }
      }
    ]
  },
  "id": 1
}
```

### Tools List

List all available tools.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "tools": [
      {
        "name": "search",
        "description": "Search through indexed documents using semantic search",
        "inputSchema": {
          "type": "object",
          "properties": {
            "query": {"type": "string"},
            "top_k": {"type": "integer", "default": 5}
          },
          "required": ["query"]
        }
      },
      {
        "name": "index_file",
        "description": "Index a file or directory",
        "inputSchema": {
          "type": "object",
          "properties": {
            "path": {"type": "string"},
            "recursive": {"type": "boolean", "default": false}
          },
          "required": ["path"]
        }
      },
      {
        "name": "get_stats",
        "description": "Get indexing statistics",
        "inputSchema": {
          "type": "object",
          "properties": {}
        }
      }
    ]
  },
  "id": 1
}
```

### Tools Call

Call a tool.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "search",
    "arguments": {
      "query": "machine learning",
      "top_k": 5
    }
  },
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Found 5 results for: machine learning\n\n[1] ml-guide.pdf (score: 0.892)\nMachine learning is a subset of artificial intelligence...\n\n[2] ai-intro.pdf (score: 0.845)\n..."
      }
    ]
  },
  "id": 1
}
```

#### Available Tools

##### search

Search through indexed documents using semantic search.

**Parameters:**
- `query` (string, required): The search query
- `top_k` (integer, optional): Number of results to return (default: 5)

**Returns:** Text content with search results.

##### index_file

Index a file or directory.

**Parameters:**
- `path` (string, required): Path to file or directory
- `recursive` (boolean, optional): Index recursively for directories (default: false)

**Returns:** Text content with indexing results.

##### get_stats

Get indexing statistics.

**Parameters:** None

**Returns:** Text content with system statistics.

### Resources Search

Search for resources.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "resources/search",
  "params": {
    "query": "machine learning",
    "limit": 10
  },
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "resources": [
      {
        "uri": "file:///docs/ml-guide.pdf#chunk=chunk_123",
        "name": "ml-guide.pdf",
        "description": "Chunk 5 from ml-guide.pdf (score: 0.892)",
        "mimeType": "text/plain"
      }
    ]
  },
  "id": 1
}
```

### Resources Get

Get a specific resource.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "resources/get",
  "params": {
    "uri": "file:///docs/ml-guide.pdf#chunk=chunk_123"
  },
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "contents": [
      {
        "uri": "file:///docs/ml-guide.pdf#chunk=chunk_123",
        "mimeType": "text/plain",
        "text": "Machine learning is a subset of artificial intelligence..."
      }
    ]
  },
  "id": 1
}
```

## Example Usage

### Python Client

```python
import requests

MCP_URL = "http://localhost:8080/mcp"

def mcp_request(method, params=None, request_id=1):
    payload = {
        "jsonrpc": "2.0",
        "method": method,
        "id": request_id
    }
    if params:
        payload["params"] = params
    
    response = requests.post(MCP_URL, json=payload)
    return response.json()

# Initialize
result = mcp_request("initialize")
print(result["result"]["name"])  # "lorag"

# Search
result = mcp_request("tools/call", {
    "name": "search",
    "arguments": {"query": "machine learning", "top_k": 5}
})
print(result["result"]["content"][0]["text"])

# Get stats
result = mcp_request("tools/call", {
    "name": "get_stats",
    "arguments": {}
})
print(result["result"]["content"][0]["text"])
```

### JavaScript Client

```javascript
async function mcpRequest(method, params, id = 1) {
  const response = await fetch('http://localhost:8080/mcp', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      method,
      params,
      id
    })
  });
  return response.json();
}

// Search
const result = await mcpRequest('tools/call', {
  name: 'search',
  arguments: { query: 'machine learning', top_k: 5 }
});
console.log(result.result.content[0].text);
```

## Error Codes

- `-32700`: Parse error
- `-32600`: Invalid Request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error
- `-32000`: Server error

## Integration Examples

### Claude Desktop

To use Lorag with Claude Desktop, add the following to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "lorag": {
      "command": "http://localhost:8080/mcp"
    }
  }
}
```

### VSCode

For VSCode extensions, use the MCP endpoint to provide context-aware suggestions:

```typescript
const mcpClient = new MCPClient('http://localhost:8080/mcp');
const results = await mcpClient.call('search', {
  query: 'current file context',
  top_k: 3
});
```
