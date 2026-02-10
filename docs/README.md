# Lorag Documentation

## Local RAG + MCP Engine

Lorag is a local-first, privacy-focused RAG (Retrieval-Augmented Generation) engine with MCP (Model Context Protocol) support for AI agents.

## Table of Contents

- [Getting Started](#getting-started)
- [Installation](#installation)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [MCP Protocol](#mcp-protocol)
- [Architecture](#architecture)

## Getting Started

### Prerequisites

- Go 1.22 or later
- Node.js 18+ (for UI development)
- Ollama (for local embeddings) or OpenAI API key

### Quick Start

```bash
# Clone the repository
git clone https://github.com/lorag/lorag.git
cd lorag

# Install dependencies
make install

# Build the project
make build

# Run the server
make run
```

The server will start on `http://localhost:8080` and the web UI will be available at the same address.

## Installation

### From Source

```bash
go install github.com/lorag/lorag/cmd/lorag@latest
go install github.com/lorag/lorag/cmd/loragctl@latest
```

### Using Docker

```bash
docker build -t lorag .
docker run -p 8080:8080 -v ./data:/data lorag
```

## Configuration

Configuration can be done via environment variables or a `.env` file:

| Variable | Description | Default |
|----------|-------------|---------|
| `LORAG_HOST` | Server host | `localhost` |
| `LORAG_PORT` | Server port | `8080` |
| `LORAG_DATA_DIR` | Data directory | `./data` |
| `LORAG_EMBEDDING_PROVIDER` | Embedding provider | `ollama` |
| `LORAG_EMBEDDING_MODEL` | Embedding model | `nomic-embed-text` |
| `LORAG_EMBEDDING_URL` | Embedding service URL | `http://localhost:11434` |
| `LORAG_EMBEDDING_DIM` | Embedding dimension | `768` |
| `LORAG_CHUNK_SIZE` | Text chunk size | `512` |
| `LORAG_CHUNK_OVERLAP` | Chunk overlap | `50` |

## API Reference

### REST API

#### Health Check
```
GET /health
```

#### Search
```
POST /api/v1/search
{
  "query": "your search query",
  "top_k": 10,
  "min_score": 0.5
}
```

#### Index File
```
POST /api/v1/index/file
{
  "path": "/path/to/file.pdf"
}
```

#### Index Directory
```
POST /api/v1/index/directory
{
  "path": "/path/to/directory",
  "recursive": true
}
```

#### List Documents
```
GET /api/v1/documents?offset=0&limit=20
```

#### Get Document
```
GET /api/v1/documents/{id}
```

#### Delete Document
```
DELETE /api/v1/documents/{id}
```

#### List Watches
```
GET /api/v1/watches
```

#### Add Watch
```
POST /api/v1/watches
{
  "path": "/path/to/watch",
  "recursive": true,
  "patterns": ["*.pdf", "*.md"]
}
```

#### Remove Watch
```
DELETE /api/v1/watches/{id}
```

## MCP Protocol

Lorag implements the Model Context Protocol (MCP) for integration with AI agents.

### Endpoint
```
POST /mcp
```

### Supported Methods

- `initialize` - Get server capabilities
- `tools/list` - List available tools
- `tools/call` - Call a tool
- `resources/search` - Search resources
- `resources/get` - Get a resource

### Example Usage

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

## Architecture

Lorag is built with a modular architecture:

- **Core Engine** - RAG indexing and querying
- **Vector Store** - HNSW-based vector index
- **Document Store** - BoltDB for metadata
- **File Watcher** - Real-time file monitoring
- **API Layer** - REST and MCP protocols
- **Web UI** - React-based interface

### Data Flow

1. Files are watched or manually indexed
2. Documents are parsed and chunked
3. Chunks are embedded using the configured provider
4. Embeddings are stored in the vector index
5. Queries are embedded and searched against the index
6. Results are returned with source references

## License

MIT License - See [LICENSE](LICENSE) for details.
