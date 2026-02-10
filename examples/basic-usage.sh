#!/bin/bash
#
# Basic usage examples for Lorag
#

set -e

echo "Lorag Basic Usage Examples"
echo "=========================="
echo ""

# Check if lorag server is running
if ! curl -s http://localhost:8080/health > /dev/null; then
    echo "Error: Lorag server is not running."
    echo "Start it with: lorag server"
    exit 1
fi

echo "1. Check server health"
curl -s http://localhost:8080/health | jq .
echo ""

# Create test documents directory
TEST_DIR="/tmp/lorag-test"
mkdir -p "$TEST_DIR"

echo "2. Create test documents"
echo "This is a document about machine learning and artificial intelligence." > "$TEST_DIR/ml.txt"
echo "Go is a programming language designed at Google." > "$TEST_DIR/go.txt"
echo "Python is a high-level programming language." > "$TEST_DIR/python.txt"
echo "Created 3 test documents in $TEST_DIR"
echo ""

echo "3. Index the test directory"
curl -s -X POST http://localhost:8080/api/v1/index/directory \
    -H "Content-Type: application/json" \
    -d "{\"path\": \"$TEST_DIR\", \"recursive\": true}" | jq .
echo ""

echo "4. Check statistics"
curl -s http://localhost:8080/api/v1/stats | jq .
echo ""

echo "5. Search for 'machine learning'"
curl -s -X POST http://localhost:8080/api/v1/search \
    -H "Content-Type: application/json" \
    -d '{"query": "machine learning", "top_k": 3}' | jq .
echo ""

echo "6. List indexed documents"
curl -s http://localhost:8080/api/v1/documents | jq '.documents[].filename'
echo ""

echo "7. Add a watch for the test directory"
curl -s -X POST http://localhost:8080/api/v1/watches \
    -H "Content-Type: application/json" \
    -d "{\"path\": \"$TEST_DIR\", \"recursive\": true}" | jq .
echo ""

echo "8. List active watches"
curl -s http://localhost:8080/api/v1/watches | jq .
echo ""

echo "=========================="
echo "Examples completed!"
echo ""
echo "Clean up test data with:"
echo "  rm -rf $TEST_DIR"
