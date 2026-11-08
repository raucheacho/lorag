# Developing Lorag

Read [ARCHITECTURE.md](ARCHITECTURE.md) first, and [STATUS.md](STATUS.md) for what is done, what is pending and the known issues.

## Setup

| Tool | Version | For |
| --- | --- | --- |
| Go | 1.24+ | the daemon and the CLI |
| Zig | 0.16.0 | only for the Linux release archives (`zig cc`) |
| Node.js | 20 | the Studio |
| Ollama | any | embeddings (`ollama pull bge-m3`) and image description |
| poppler (`pdftotext`), ffmpeg, whisper.cpp, tesseract | optional | PDF pages, audio and video, OCR ([MEDIA_INDEXING.md](MEDIA_INDEXING.md)) |

```sh
(cd ui && npm ci && npm run build)
make build-server                # bin/lorag
./bin/lorag server -d ./data     # Studio on http://localhost:8080
```

`go.mod` requires a released VectixDB binding (`v0.1.0`). Its per-platform modules (`bindings/go/lib/<platform>`) carry the prebuilt static libraries, so building needs no Zig and no VectixDB checkout. To upgrade: `go get github.com/raucheacho/vectixdb/bindings/go@v<version>`.

To try an unreleased VectixDB, build its library and use a `go.work` (ignored by git):

```sh
../vectixdb/scripts/go-libs.sh
go work init . ../vectixdb/bindings/go ../vectixdb/bindings/go/lib/darwin_arm64
```

## Everyday commands

| Command | |
| --- | --- |
| `make dev-server` | `go run ./cmd/lorag server` (needs `ui/dist`, even empty) |
| `make dev-ui` | Vite on http://localhost:5173 with hot reload, proxying `/api` to :8080 |
| `make build-server`, `make build-cli` | `bin/lorag`, `bin/loragctl` |
| `make test` | `go test -v ./...` |
| `go test -race ./...` | What CI runs |
| `(cd ui && npx tsc --noEmit)` | Typecheck the Studio |
| `make docker-build` | Docker image |
| `make dist` | Release archive for the host in `dist/`; `PLATFORMS="darwin_arm64 linux_amd64 ..."` for others |

### Configuration for development

| Variable | Default | |
| --- | --- | --- |
| `LORAG_DATA_DIR` | `~/.lorag/data` | use a scratch directory to test; one server per directory |
| `LORAG_HOST`, `LORAG_PORT` | `localhost`, `8080` | |
| `LORAG_EMBEDDING_PROVIDER`, `_MODEL`, `_URL`, `_DIM`, `_API_KEY` | from the settings | override the persisted settings at start |
| `LORAG_MAX_CHUNKS` | from the settings (`max_chunks`, 0: no limit) | caps the index size; parallel workers reserve their chunks, so the cap holds |
| `LORAG_MAX_QUEUE_SIZE` | 100 | |

Every other setting (embedding model, index parameters, chunking, media) lives in BoltDB and is edited in Studio → Settings. Most take effect on restart.

## Tests

| Package | What the tests cover |
| --- | --- |
| `internal/core/engine` | put/search/reopen, hybrid with filters, atomic replace, generations on config change, rebuild (with writes during it, failure, concurrent searches) |
| `internal/core/indexer` | chunk positions: exact lines and bytes for Markdown, pages and media time ranges |
| `internal/parsers` | Markdown and code segments, PDF pages (generated PDF, real `pdftotext`), xlsx sheets, pptx slides, whisper JSON, kinds |
| `internal/memory` | signatures across machines, secret masking, save/confirm/variant, recall by signature and similarity, context RECHECK, spaces, history, migration, knowledge entries, budget |
| `internal/api/mcp` | the protocol (versions, notifications, Origin), indexing and search end to end with a fake embedder, the memory workflow, exact-first search with positions |
| `internal/core/store`, `internal/activity` | storage and the activity ring |

Tests that need a binary (`pdftotext`) skip when it is missing.

### Checking with real models and files

The unit tests use deterministic fake embedders. Before trusting a change to ranking or memory thresholds, check with Ollama and real files:

1. Start a server on a scratch directory: `./bin/lorag server -p 18080 -d /tmp/lorag-check`.
2. In Settings, turn on transcription for audio and video, then restart.
3. Index a folder with a multi-page PDF, Markdown, code, an audio file and a video.
4. Search through MCP (`curl` examples in [MCP.md](MCP.md)) and check that each result points at the right page, lines or minute.
5. For the memory, save a few entries and recall paraphrases and unrelated questions: Studio → Memory → Test a recall shows what the agent receives.

### Studio checks

There are no automated UI tests in the repository yet. The checks used so far ran Playwright (`playwright-core` with a local Chromium) against a live server:
- command palette, Playground runs and route comparison;
- drawers and dropdown keyboard navigation;
- Settings save and discard, MCP test;
- the memory views;
- the video player range, PDF page links, upload;
- no console errors, in both themes.

Adding them under `ui/e2e/` with a seed script is in [STATUS.md](STATUS.md).

## Recipes

### Adding an MCP tool

1. Add its definition to `tools` in `internal/api/mcp/server.go`: name, description written for an agent, JSON schema, annotations.
2. Add its case in `callTool`. Return errors as tool results (`isError`), not protocol errors.
3. Log searches and recalls in the activity log with the client name.
4. Test it in `internal/api/mcp/server_test.go` through the fixture (real VectixDB, fake embedder).
5. Document it in [MCP.md](MCP.md). The Connect page lists tools from the server, so it needs no change.

### Adding a REST endpoint and a Studio view

1. Write the handler in `internal/api/rest/` and register its route in `router.go`.
2. Add the types and the call in `ui/src/lib/api.ts`.
3. Add the page in `ui/src/pages/`, its route in `App.tsx` and a nav entry in `components/Shell.tsx`. Use the primitives in `components/ui.tsx` (Button with `tip`, Select, NumberField, InputShell, Drawer, Section, Stat, SkeletonRows) rather than native controls.
4. Document the endpoint in [API.md](API.md).

### Supporting a new file type

1. Map the extension to a kind in `parsers.KindOf`.
2. Produce segments with positions in `parsers.Extract`:
   - verbatim slices with `Line` and `Offset` for text;
   - `Page` for paged formats;
   - `StartMs`/`EndMs` for media.
3. Test the segments in `internal/parsers/extract_test.go`. Chunking and display follow automatically.

### Changing the chunk fields

1. Change `engine.Chunk`, `schema()`, `doc()`, `chunkOf()` and `returnFields`.
2. **Bump `schemaVersion` in `engine.go`.** Existing installations then start a new generation and rebuild it from BoltDB. Without the bump, old collections lack the field and searches fail.
3. Carry the field through `indexer.indexEntry` and `query.Source`.

### Changing memory entries

1. Fields that are only in the JSON payload need nothing.
2. A new filterable field: add it to `memory.schema()` and `doc()`, then **bump `schemaVersion` in `memory.go`**, so entries are migrated (re-embedded) into a new generation.
3. Old payloads decode with zero values; give them a sensible default in `decode` (see `kindOf`).

## Conventions

- **Formatting.** `gofmt` the files you touch. Several older files are not formatted; do not reformat them in the same change as a fix.
- **Comments.** Explain why, not what. Exported identifiers have doc comments.
- **Errors.** Errors from VectixDB carry their code: compare with `errors.Is(err, vectixdb.ErrLocked)`.
- **Studio text.** It is in English; user documentation is in French.
- **Commits.** Short French messages in lowercase, without a conventional prefix.

## CI and Docker

- **Tests.** `.github/workflows/ci.yml` runs `go vet` and the Go tests with `-race`, the UI build and the Docker build.
- **Image.** The `Dockerfile` is Debian-based (the VectixDB library needs glibc).
- **Releases.** A tag `v<version>` runs `.github/workflows/release.yml`:
  - `scripts/dist.sh` builds the archives: macOS on a macOS runner (Apple clang, the binary links Apple frameworks), Linux with `zig cc` against glibc 2.28;
  - each runner checks its native binary;
  - a draft GitHub release gets the archives, `SHA256SUMS` and the Homebrew formula from `scripts/homebrew.sh`;
  - the Docker image goes to `ghcr.io` (amd64 only for now).

  Publish the draft by hand: `.github/workflows/tap.yml` then pushes `lorag.rb` to `raucheacho/homebrew-tap` with the secret `HOMEBREW_TAP_TOKEN` (skipped for prereleases). Details in [BUILD_INSTRUCTIONS.md](../BUILD_INSTRUCTIONS.md).
- **Installation.** `scripts/install.sh` installs a release in `~/.local/bin` after checking its checksum, and with `--service` adds a systemd user unit. `LORAG_DOWNLOAD_URL` points it at a local mirror for testing.
