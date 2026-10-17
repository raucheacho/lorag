# Architecture

Lorag is one Go process (the daemon). It indexes local files, keeps a memory of fixes and answers for AI agents, and serves three interfaces: MCP, a REST API and the Studio, a web UI embedded in the binary. Search runs in [VectixDB](https://github.com/raucheacho/vectixdb), a Zig engine linked into the process through cgo.

```
 agents (MCP clients)      scripts, loragctl         browser
        │ POST /mcp               │ /api/v1               │ /  (Studio, embedded)
        ▼                         ▼                       ▼
 ┌──────────────────────────── lorag server (Go) ───────────────────────────┐
 │ internal/api/mcp      internal/api/rest            ui/ (React, go:embed) │
 │        │                     │                                            │
 │        ├──── internal/core/query ─────┐        internal/memory            │
 │        │      (search, filters)        │   (fixes and answers, journal)   │
 │        │                               ▼               │                  │
 │ internal/core/indexer ───▶ internal/core/engine ◀──────┘                  │
 │  (queue, chunker)            (chunk generations)                          │
 │        │                               │ cgo                              │
 │ internal/parsers                       ▼                                  │
 │  (positioned extraction)   VectixDB: libvectixdb.a                        │
 │        │                               │                                  │
 │ internal/watcher           internal/core/store (BoltDB)                   │
 │ internal/embeddings (Ollama, OpenAI-compatible)                           │
 └──────────────────────────────────────────────────────────────────────────┘
              │                                │
       <data>/lorag.db                   <data>/vectix/
```

## Data on disk

| Path | Content | Source of truth? |
| --- | --- | --- |
| `<data>/lorag.db` | BoltDB: documents, chunks with their embeddings and positions, settings, watched folders | yes, for documents |
| `<data>/vectix/` | VectixDB: the search index (`chunks_<n>`), the memory (`memory_<n>`, `memory_events`), pointers (`lorag_meta`) | yes for the memory; the chunk index is derived and can be rebuilt from BoltDB |
| `<data>/imports/<date>/` | Files dropped in the Studio | yes |
| `~/.lorag/models/` | Whisper models, downloaded on first use | cache |

`<data>` is `lorag server --data-dir`, else `LORAG_DATA_DIR`, else `~/.lorag/data` (`config.ResolveDataDir`). A `./data` left by an earlier version, which defaulted to it, is still used while `~/.lorag/data` holds no database.

The activity log (searches and recalls shown in Agent logs) is an in-memory ring of 1,000 entries; it is lost on restart.

## Packages

| Package | Role |
| --- | --- |
| `cmd/lorag` | `lorag server`, `lorag version` |
| `cmd/loragctl` | CLI client of the REST API: `index`, `search`, `status`, `watch`, `doc`, `mem` |
| `internal/server` | Wiring: opens the store, the engine and the memory, starts the watcher and the HTTP server, rebuilds the index when it is behind the store |
| `internal/config` | Environment variables (`LORAG_*`) |
| `internal/core/store` | BoltDB: documents, chunks, settings (`AppSettings`), watches |
| `internal/core/engine` | The chunk index on VectixDB, its generations and rebuilds |
| `internal/core/indexer` | File → positioned segments → chunks → embeddings → store + index; async queue |
| `internal/core/query` | Search modes (hybrid, vector, keyword, exact), filters, scores, neighbor context |
| `internal/parsers` | Extraction with positions: text, Markdown, code, PDF, Office, slides, sheets, audio, video, images |
| `internal/embeddings` | Embedding providers (Ollama, OpenAI-compatible, custom) |
| `internal/memory` | Fixes and answers: signatures, variants, recall, feedback, events, statistics |
| `internal/activity` | In-memory log of searches and recalls, with client attribution |
| `internal/watcher` | fsnotify watches that reindex changed files |
| `internal/api/rest` | REST handlers ([API.md](API.md)), file serving, uploads |
| `internal/api/mcp` | MCP server over Streamable HTTP ([MCP.md](MCP.md)) |
| `ui` | Studio (React, Vite, Tailwind), embedded with `go:embed all:dist` |

## Indexing a file

```
watcher / REST / MCP ingest / upload
        │
        ▼
indexer.IndexFile(path)
  1. checksum; unchanged file → skipped
  2. parsers.Extract(path) → []Segment{Text, Page, Line, Offset, StartMs, EndMs, Section}
  3. chunker.ChunkSegments → []Piece (pages never merged, media ≤ 60 s per piece)
  4. embed each piece (section heading prepended for context)
  5. store: delete the previous version, save document + chunks
  6. engine.Replace(old chunk ids, new chunks)   ← one atomic VectixDB write
```

- **Embed before writing.** If the embedding service is down, the previous version of the file stays intact.
- **One atomic swap.** Old and new chunks are swapped in a single write, so a search never sees half a document.
- **Positions.** The extractor of each file type produces them; see [MEDIA_INDEXING.md](MEDIA_INDEXING.md).
- **Queue.** With `index_workers > 0`, files are indexed in parallel through `indexer.Queue`. Events (indexed, unchanged, failed) feed the Ingestion page.

## The chunk index (`internal/core/engine`)

Chunks live in a VectixDB collection named `chunks_<generation>`. The document `lorag_meta/index` holds `{active, config}`, where `config` is a fingerprint of everything baked into the collection:

```
schema=2 model=bge-m3 dim=1024 metric=cosine int8=false m=16 efc=200 k1=1.2 b=0.75
```

- **At startup**, if the fingerprint differs from the settings, a new empty generation is created. `server.rebuildIfOutOfSync` then fills it from BoltDB, in the background.
- **A rebuild** fills generation n+1 while searches use n. Writes during the rebuild go to both in the same batch, then one write switches the pointer.
- **Leftovers** of an interrupted rebuild are dropped at the next start.

Each chunk stores its text and the fields needed to display and filter it without BoltDB: path, filename, extension, kind, position, page, lines, times, section.


## Search (`internal/core/query`)

`query.Engine.Run` turns a `Query` into one `engine.Request`:

| Mode | Engine request | Score shown |
| --- | --- | --- |
| hybrid (default) | vector + BM25, RRF k=60 | RRF normalized to [0, 1] (1 = first in both routes) |
| vector | vector | similarity |
| keyword | BM25 | BM25 |
| exact | literal substring | 1 |

- **Filters.** Filters on `path`, `filename`, `extension`, `document_id` and `kind` (eq, neq) run inside the engine. Others (`contains`, metadata) are applied afterwards on the document, with over-fetching.
- **Error messages over MCP.** The MCP `search` tool tries the exact route first, with and without the `Error:` prefix, then falls back to hybrid.
- **Logging.** Every search goes to the activity log, with its source (mcp, api, studio) and client name.

## The memory (`internal/memory`)

Fixes and answers for agents; the full description is in [MEMORY.md](MEMORY.md).

- **Storage.** Entries are documents of `memory_<generation>`:
  - the vector is the embedding of the problem and the error line, plus the answer for knowledge entries;
  - the payload is the full entry as JSON;
  - `signature`, `kind`, `status`, `space`, `tags`, `updated` are fields for filtering.
- **Journal.** Every change is also an event in `memory_events`, in the same atomic batch, so history is never lost.
- **Migration.** A configuration change (model, dimension, schema version) re-embeds the entries into the next generation before switching.
- **Thresholds.** Similarity thresholds for recall depend on the model (`memory.calibrated`), overridable in the settings.
- **Vector cache.** Stored vectors are kept in memory (`Service.vecs`), so counter updates do not re-embed.

## Interfaces

- **MCP.**
  - Endpoints: `POST /mcp` (JSON-RPC, JSON responses), `DELETE` ends a session.
  - Sessions name the client, used in the activity log.
  - Requests with a foreign `Origin` are refused.
  - Tools: `memory_recall`, `memory_save`, `memory_feedback`, `search`, `get_document`, `ingest`, `get_stats`.
- **REST.** Under `/api/v1`; the Studio and `loragctl` use it. Requests sent with `X-Lorag-Client: studio` are logged as Studio activity and memory edits as the person.
- **Studio.**
  - A single-page app in `ui/`, built to `ui/dist` and embedded.
  - The server serves `index.html` for unknown paths (client-side routes).
  - In development, Vite on port 5173 proxies `/api` to port 8080.

## Concurrency

- **VectixDB** is the only writer of `<data>/vectix` (exclusive lock): run one `lorag server` per data directory.
- **`engine.Engine`.**
  - A read-write mutex guards the active collection name.
  - Searches and writes hold the read side for their whole call, so a rebuild cannot drop a collection under them.
- **`memory.Service`** serializes its writes (dedupe and counters) with a mutex.
- **BoltDB** serializes its own writes.
