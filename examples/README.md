# Lorag Examples

This directory contains example code and usage patterns for Lorag.

## Python MCP Client

The `mcp-client.py` script demonstrates how to interact with Lorag using the MCP protocol from Python.

### Usage

```bash
# Install dependencies
pip install requests

# Interactive mode
python mcp-client.py

# Command mode
python mcp-client.py search "machine learning"
python mcp-client.py index /path/to/documents --recursive
python mcp-client.py stats
```

## Shell Script Examples

The `basic-usage.sh` script demonstrates common operations using curl.

### Usage

```bash
# Make sure lorag server is running
lorag server &

# Run the examples
chmod +x basic-usage.sh
./basic-usage.sh
```

## API Examples

### Indexing

```bash
# Index a file
curl -X POST http://localhost:8080/api/v1/index/file \
    -H "Content-Type: application/json" \
    -d '{"path": "/path/to/file.pdf"}'

# Index a directory recursively
curl -X POST http://localhost:8080/api/v1/index/directory \
    -H "Content-Type: application/json" \
    -d '{"path": "/path/to/documents", "recursive": true}'
```

### Search

```bash
# Basic search
curl -X POST http://localhost:8080/api/v1/search \
    -H "Content-Type: application/json" \
    -d '{"query": "machine learning", "top_k": 5}'

# Search with minimum score
curl -X POST http://localhost:8080/api/v1/search \
    -H "Content-Type: application/json" \
    -d '{"query": "AI", "top_k": 10, "min_score": 0.7}'
```

### Document Management

```bash
# List documents
curl http://localhost:8080/api/v1/documents

# Get specific document
curl http://localhost:8080/api/v1/documents/DOC_ID

# Delete document
curl -X DELETE http://localhost:8080/api/v1/documents/DOC_ID
```

### Watch Management

```bash
# List watches
curl http://localhost:8080/api/v1/watches

# Add watch
curl -X POST http://localhost:8080/api/v1/watches \
    -H "Content-Type: application/json" \
    -d '{"path": "/path/to/watch", "recursive": true}'

# Remove watch
curl -X DELETE http://localhost:8080/api/v1/watches/WATCH_ID
```

## More Examples

See the [API documentation](../docs/API.md) and [MCP documentation](../docs/MCP.md) for more detailed examples.
