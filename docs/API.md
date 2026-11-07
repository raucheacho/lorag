# REST API

Base URL: `http://localhost:8080/api/v1`. JSON in and out. Errors are `{"error": "message"}` with a 4xx or 5xx status.

There is no authentication: the server listens on `localhost` by default and must not be exposed. CORS accepts `localhost` and `127.0.0.1` origins.

Header `X-Lorag-Client: studio` marks requests from the Studio: searches are logged as Studio activity, and memory saves, edits and feedback as made by the person.

## Health and overview

| Method | Path | |
| --- | --- | --- |
| GET | `/health` (outside `/api/v1`) | `{status, version, timestamp}` |
| GET | `/overview` | Everything the Overview page shows: version, data dir, document and chunk counts, index stats, embedder settings, queue, activity summary |
| GET | `/stats` | Document and chunk counts, index stats, chunk limit, watches |
| GET | `/embedder` | Embeds a test text: `{ok, latency_ms, dim, expected, error}` (10 s timeout) |

`index` in `/overview` is the chunk index:

```json
{"core_version": "0.1.0", "collection": "chunks_1", "chunks": 79, "dim": 768, "metric": "cosine",
 "quantized": false, "disk_bytes": 303000, "wal_bytes": 22358, "last_seq": 6, "checkpoint_seq": 6, "rebuilding": false}
```

## Search

`POST /search`

```json
{
  "query": "how do we roll back the payments service",
  "top_k": 10,
  "mode": "hybrid",
  "context_chunks": 0,
  "min_score": 0,
  "filters": [{"field": "kind", "operator": "eq", "value": "pdf"}],
  "format": "json",
  "template": "default"
}
```

- **`mode`.**

  | Mode | Searches by |
  | --- | --- |
  | `hybrid` (default) | meaning and words |
  | `vector` | meaning |
  | `keyword` | BM25 |
  | `exact` | the literal text |

- **`filters`.**
  - Fields: `path`, `filename`, `extension`, `document_id` and `kind` run in the engine with `eq` or `neq`. `kind` is one of `text`, `markdown`, `code`, `pdf`, `office`, `sheet`, `slides`, `audio`, `video`, `image`.
  - Operators `contains`, `startswith`, `endswith` and other document metadata fields are applied afterwards.
- **`context_chunks`.** Neighbor chunks, 0 to 3, replace each result's content with the chunk and its neighbors.
- **`format: "text"`.** Returns plain text framed by `template`.

Response:

```json
{
  "query_id": "q_17",
  "mode": "hybrid",
  "candidates": 7,
  "timings": {"embed_ms": 29.9, "search_ms": 0.1, "total_ms": 30.0},
  "warnings": [],
  "total": 7,
  "results": [{
    "chunk_id": "7461…", "document_id": "09b9…", "content": "Welcome to the quarterly incident review…",
    "score": 1.0,
    "vector": {"rank": 1, "score": 0.669},
    "keyword": {"rank": 1, "score": 8.93},
    "source": {
      "path": "/talks/incident-review.mp4", "filename": "incident-review.mp4", "position": 0,
      "kind": "video", "start_ms": 0, "end_ms": 12620,
      "page": 0, "line_start": 0, "line_end": 0, "section": "",
      "start_byte": -1, "end_byte": -1
    }
  }]
}
```

`vector` and `keyword` are present only when that route returned the chunk. Hybrid scores are RRF normalized to [0, 1].

## Indexing

| Method | Path | Body | |
| --- | --- | --- | --- |
| POST | `/index/file` | `{path, async}` | Indexes one file; `async` queues it and returns `task_id` |
| POST | `/index/directory` | `{path, recursive, async}` | Indexes every supported file; `async` returns at once |
| POST | `/upload` | `multipart/form-data`, field `files` (repeatable) | Stores files under `<data>/imports/<date>/` and queues them; returns `{files: [{name, path, size, task_id, error}]}`. 4 GiB per file |
| GET | `/ingestion?limit=` | | `{queue, running, events}`: queue stats, files in progress, last events (`indexed`, `unchanged`, `failed` with `error`) |
| GET | `/queue/stats` | | |
| GET | `/queue/tasks/{id}` | | `{status: running, completed, failed, not_found, …}` |
| POST | `/documents/reindex-media` | `{types: ["audio","video","image"]}` | Reindexes media files in the background |
| POST | `/index/rebuild` | | Rebuilds the search index from the store, in the background |
| POST | `/index/compact` | | Drops deleted chunks from the index |
| POST | `/index/save` | | Checkpoints the index (trims its log) |

## Documents

| Method | Path | |
| --- | --- | --- |
| GET | `/documents/?q=&ext=&sort=&offset=&limit=` | `q` matches path and name; `sort`: `updated` (default), `name`, `size`, `chunks`. Returns `{documents, total, offset, limit}` |
| GET | `/documents/{id}` | One document: `path`, `filename`, `extension`, `kind`, `size`, `checksum`, `indexed_at`, `chunk_count`, `metadata` |
| GET | `/documents/{id}/chunks` | Chunks in order, without embeddings: `content`, `position`, `tokens`, `dim`, `indexed` (present in the search index), `page`, `line_start`, `line_end`, `start_ms`, `end_ms`, `section`, `start_byte`, `end_byte` |
| GET | `/documents/{id}/file` | The original file, with range requests. Code and HTML are served as `text/plain` |
| POST | `/documents/{id}/reindex` | Reindexes the file; returns the new document (its id changes) |
| DELETE | `/documents/{id}` | Removes the document from the store and the index |

To open a source at a chunk's position: `/documents/{id}/file#page=3` for a PDF, `#t=12.0,45.5` for audio and video.

## Watched folders

| Method | Path | |
| --- | --- | --- |
| GET | `/fs/dirs?path=&hidden=1` | `{path, parent, dirs: [{name, path}], shortcuts}`: subfolders of a folder on the server (home by default), for the Studio's folder picker |
| GET | `/watches/` | `{watches: [{id, path, recursive, patterns, created_at}]}` |
| POST | `/watches/` | `{path, recursive, patterns}`; persisted in the settings |
| DELETE | `/watches/{id}` | |

## Activity

| Method | Path | |
| --- | --- | --- |
| GET | `/activity/?source=&limit=` | Searches and memory recalls, newest first. `source`: `mcp`, `api`, `studio`. Returns `{queries, summary}`; `summary` has counts, p50/p95 latency, errors, empty results, clients |
| GET | `/activity/{id}` | One query with its hits |

The log holds the last 1,000 queries in memory.

## Memory

Described in [MEMORY.md](MEMORY.md).

| Method | Path | Body / query | |
| --- | --- | --- | --- |
| POST | `/memory/recall` | `{error, query, project, context, k, budget_tokens}` | `{text, tokens, matches: [{entry, by, similarity, status, reason}], nearest, thresholds}`; `nearest` is the best rejected candidate on a miss |
| POST | `/memory/` | `{kind, problem, solution, error, source, project, space, versions, platform, tags, solve_tokens}` | `{entry, action: created \| variant \| confirmed}` |
| GET | `/memory/?kind=&status=&space=&q=&limit=` | | `{entries, total}` |
| GET | `/memory/{id}` | | `{entry, history}`; `entry` is `null` after a delete |
| PATCH | `/memory/{id}` | `{kind, source, problem, solution, status, tags}` | A new revision, authored by the person |
| DELETE | `/memory/{id}` | | Removed from recall; the history stays |
| POST | `/memory/{id}/feedback` | `{worked, note}` | |
| GET | `/memory/stats` | | Entries by kind, status and space; recalls, hits, misses, tokens returned and saved (estimate); 30 days of daily counts; most recalled and most useful |
| GET | `/memory/events?limit=` | | The journal, newest first |
| GET | `/memory/health` | | `{recheck, obsolete, stale, duplicates}` |
| GET | `/memory/export` | | JSON Lines, one entry per line |

## Settings

| Method | Path | |
| --- | --- | --- |
| GET | `/settings` | All settings |
| PUT | `/settings` | Partial update, validated; returns `{status, restart_required, settings}` |
| GET | `/models/status` | OCR and transcription state; per Whisper model, `model_status` (`not_downloaded`, `downloading`, `ready`, `error`) and `progress` (`downloaded`, `total` bytes, `error`) |
| POST | `/models/whisper/{tiny\|base\|small}/download` | Starts downloading the model to `~/.lorag/models` in the background (202); follow it with `/models/status` |

| Setting | Default | Takes effect |
| --- | --- | --- |
| `embedding_provider`, `embedding_model`, `embedding_url`, `embedding_dim` | `ollama`, `bge-m3`, `http://localhost:11434`, 1024 | restart; a new model or dimension rebuilds the index and re-embeds the memory |
| `memory_min_similarity`, `memory_min_similarity_text` | 0 (calibrated for the model) | restart |
| `vector_metric`, `vector_quantized`, `vector_m`, `vector_ef`, `bm25_k1`, `bm25_b` | `cosine`, false, 16, 200, 1.2, 0.75 | restart, then a rebuild of the index |
| `vector_ef_search` | 64 | restart |
| `chunk_size` (characters), `chunk_overlap`, `index_workers`, `max_chunks` | 512, 50, 3, 50000 | restart; files indexed afterwards |
| `default_template` | `default` | immediately (MCP search answers) |
| `ocr_enabled`, `ocr_model` | false, `llava:7b` | restart (image description through Ollama) |
| `transcription_enabled`, `whisper_model` | true, `tiny` | restart (audio and video) |

## MCP

`POST /mcp`, outside `/api/v1`: see [MCP.md](MCP.md).
