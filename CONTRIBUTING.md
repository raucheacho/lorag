# Contributing to Lorag

Thank you for your interest in contributing to Lorag! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions.

## How to Contribute

### Reporting Bugs

- Check if the bug has already been reported in the [issues](https://github.com/lorag/lorag/issues)
- If not, create a new issue with:
  - A clear description of the bug
  - Steps to reproduce
  - Expected behavior
  - Actual behavior
  - Environment details (OS, Go version, etc.)

### Suggesting Features

- Check if the feature has already been requested
- Create a new issue with the "feature request" label
- Describe the feature and its use case
- Explain why it would be useful

### Pull Requests

1. Fork the repository
2. Create a new branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add or update tests as needed
5. Ensure tests pass (`make test`)
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.22+
- Node.js 18+ (for UI development)
- Make

### Setup

```bash
# Clone your fork
git clone https://github.com/yourusername/lorag.git
cd lorag

# Install dependencies
make install

# Run tests
make test

# Build
make build

# Run server
make run
```

### Project Structure

```
lorag/
├── cmd/              # Entry points
│   ├── lorag/        # Main server
│   └── loragctl/     # CLI tool
├── internal/         # Internal packages
│   ├── api/          # REST and MCP APIs
│   ├── core/         # Core RAG functionality
│   ├── embeddings/   # Embedding providers
│   ├── parsers/      # Document parsers
│   └── watcher/      # File system watcher
├── web/              # React web UI
├── docs/             # Documentation
└── examples/         # Example code
```

## Coding Standards

### Go Code

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` to format code
- Use meaningful variable names
- Add comments for exported functions
- Keep functions focused and small
- Write tests for new functionality

### TypeScript/React Code

- Follow the existing code style
- Use TypeScript for type safety
- Use functional components with hooks
- Keep components small and focused
- Add PropTypes or TypeScript interfaces

### Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

Example:

```
Add fuzzy search to document listing

- Implement Levenshtein distance algorithm
- Add fuzzy matching to search API
- Update UI to show fuzzy match scores

Fixes #123
```

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific test
go test ./internal/core/... -v
```

### Writing Tests

- Add tests for new functionality
- Use table-driven tests where appropriate
- Mock external dependencies
- Test both success and error cases
- Aim for good coverage, but focus on critical paths

## Documentation

- Update README.md if you change functionality
- Add documentation for new features
- Update API.md for API changes
- Update MCP.md for protocol changes

## Release Process

1. Update version in relevant files
2. Update CHANGELOG.md
3. Create a new release on GitHub
4. CI will build and publish artifacts

## Questions?

Feel free to open an issue for questions or join our Discord community.

Thank you for contributing to Lorag!
je fais montest
