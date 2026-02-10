# Lorag API Documentation

## Overview

Lorag provides a REST API for managing documents, performing searches, and configuring the system. The API is available at `/api/v1/`.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Currently, the API does not require authentication by default. To enable authentication, set the `LORAG_ENCRYPTION` environment variable and configure API keys in the settings.

## Endpoints

### Health Check

Check if the server is running.

```
GET /health
```

**Response:**
```json
{
  "status": "ok",
  "version": "0.1.0",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### Statistics

Get system statistics.

```
GET /api/v1/stats
```

**Response:**
```json
{
  "documents": 150,
  "chunks": 2500,
  "vector": {
    "elements": 2500,
    "dimension": 768
  },
  "maxChunks": 50000
}
```

### Search

Perform a semantic search across indexed documents.

```
POST /api/v1/search
```

**Request Body:**
```json
{
  "query": "machine learning algorithms",
  "top_k": 10,
  "min_score": 0.5,
  "context_chunks": 2
}
```

**Parameters:**
- `query` (string, required): The search query
- `top_k` (integer, optional): Number of results to return (default: 10)
- `min_score` (float, optional): Minimum relevance score (0-1)
- `context_chunks` (integer, optional): Number of chunks to include as context

**Response:**
```json
{
  "results": [
    {
      "chunk_id": "chunk_123",
      "document_id": "doc_456",
      "content": "Machine learning is a subset of artificial intelligence...",
      "score": 0.89,
      "source": {
        "path": "/docs/ml-guide.pdf",
        "filename": "ml-guide.pdf",
        "position": 5,
        "start_byte": 2048,
        "end_byte": 2560
      }
    }
  ],
  "total": 1,
  "time": "2024-01-01T00:00:00Z"
}
```

### Index File

Index a single file.

```
POST /api/v1/index/file
```

**Request Body:**
```json
{
  "path": "/path/to/document.pdf"
}
```

**Response:**
```json
{
  "success": true,
  "message": "indexed successfully",
  "documents": 1,
  "chunks": 12
}
```

### Index Directory

Index all files in a directory.

```
POST /api/v1/index/directory
```

**Request Body:**
```json
{
  "path": "/path/to/documents",
  "recursive": true
}
```

**Response:**
```json
{
  "success": true,
  "message": "Indexed: 10, Skipped: 2, Failed: 0, Total chunks: 150",
  "documents": 10,
  "chunks": 150
}
```

### List Documents

Get a list of indexed documents.

```
GET /api/v1/documents?offset=0&limit=20
```

**Query Parameters:**
- `offset` (integer, optional): Number of documents to skip
- `limit` (integer, optional): Maximum number of documents to return (max: 100)

**Response:**
```json
{
  "documents": [
    {
      "id": "doc_123",
      "path": "/docs/guide.pdf",
      "filename": "guide.pdf",
      "extension": ".pdf",
      "size": 1048576,
      "chunk_count": 15,
      "created_at": "2024-01-01T00:00:00Z",
      "indexed_at": "2024-01-01T00:05:00Z"
    }
  ],
  "offset": 0,
  "limit": 20
}
```

### Get Document

Get details of a specific document.

```
GET /api/v1/documents/{id}
```

**Response:**
```json
{
  "id": "doc_123",
  "path": "/docs/guide.pdf",
  "filename": "guide.pdf",
  "extension": ".pdf",
  "size": 1048576,
  "mime_type": "application/pdf",
  "checksum": "abc123...",
  "metadata": {
    "author": "John Doe",
    "title": "Guide"
  },
  "chunk_count": 15,
  "created_at": "2024-01-01T00:00:00Z",
  "indexed_at": "2024-01-01T00:05:00Z",
  "tags": ["guide", "docs"]
}
```

### Delete Document

Remove a document from the index.

```
DELETE /api/v1/documents/{id}
```

**Response:**
```json
{
  "status": "deleted",
  "id": "doc_123"
}
```

### Get Document Chunks

Get all chunks for a specific document.

```
GET /api/v1/documents/{id}/chunks
```

**Response:**
```json
{
  "chunks": [
    {
      "id": "chunk_1",
      "document_id": "doc_123",
      "content": "This is the content...",
      "position": 0,
      "start_byte": 0,
      "end_byte": 512,
      "tokens": 128,
      "created_at": "2024-01-01T00:05:00Z"
    }
  ],
  "total": 15
}
```

### List Watches

Get all active directory watches.

```
GET /api/v1/watches
```

**Response:**
```json
[
  {
    "id": "watch_123",
    "path": "/path/to/watch",
    "recursive": true,
    "patterns": ["*.pdf", "*.md"],
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

### Add Watch

Add a new directory watch.

```
POST /api/v1/watches
```

**Request Body:**
```json
{
  "path": "/path/to/directory",
  "recursive": true,
  "patterns": ["*.pdf", "*.md"]
}
```

**Response:**
```json
{
  "id": "watch_123",
  "path": "/path/to/directory",
  "recursive": true,
  "patterns": ["*.pdf", "*.md"],
  "created_at": "2024-01-01T00:00:00Z"
}
```

### Remove Watch

Remove a directory watch.

```
DELETE /api/v1/watches/{id}
```

**Response:**
```json
{
  "status": "removed",
  "id": "watch_123"
}
```

## Error Responses

All errors follow this format:

```json
{
  "error": "Error message description"
}
```

Common HTTP status codes:
- `200` - Success
- `201` - Created
- `400` - Bad Request
- `404` - Not Found
- `500` - Internal Server Error

## Rate Limiting

Rate limiting is not currently implemented. For production use, consider implementing rate limiting at the reverse proxy level (nginx, traefik, etc.).
