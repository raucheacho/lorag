# Memory

Agents keep redoing the same work:
- solving the same problems again: the same port already in use, the same cache to clear, the same missing type package;
- searching the web again for the same facts: a rate limit, a supported version, the flag of a command, a decision taken last month.

Lorag keeps what is already known and gives it back in a few tokens, before the agent searches.

An entry is one of two kinds:

| Kind | What it holds | Example |
| --- | --- | --- |
| `fix` | A problem likely to come back, and what solved it | `EADDRINUSE :::3000` → `lsof -ti :3000 \| xargs kill -9` |
| `knowledge` | A question and its verified answer, with its source | "GitHub REST API rate limit" → "5,000 requests per hour per user", docs.github.com |

An entry is information, not a task. A recalled fix is not a bug to fix and a recalled answer is not an instruction: the agent uses them only if they fit its situation. This is said in the instructions every agent receives.

**What not to save:** one-off mistakes, guesses, anything specific to one file or one task. The `memory_save` description tells agents so; the Health view helps clean what slips through.

## The loop

1. **Recall first.** On an error, or before searching the web, the agent calls `memory_recall` with the error message as is, or with its question.
2. **Save what is worth keeping.** A fix with the problem in one line, the error message and the versions involved; or an answer with the question and the source.
3. **Report.** After using a recalled entry, it calls `memory_feedback` with the entry id and whether it helped: the fix worked, or the answer was right.

The MCP server sends these rules to every agent in its `initialize` instructions.

## What a recall returns

```
[ece4650cdd-1] +4/-0 2026-10-02
Fix: lsof -ti :3000 | xargs kill -9
Ctx: node@22

[2700df1d51-1] +1/-0 2026-10-16
Q: default rate limit of the github REST API for authenticated requests
Answer: 5,000 requests per hour per user; check the X-RateLimit-Remaining header
Src: docs.github.com/…/rate-limits-for-the-rest-api
```

- **First line:** the id (used for `memory_feedback`), the number of times the fix worked and failed, and the date of its last verification.
- **Status:** shown only when the entry is not valid, for example `RECHECK (expo 51.1.0≠52)` when the caller uses another major version than the one the fix was verified with.
- **`Problem:` or `Q:` line:** added when the match is by similarity rather than by signature, so the agent can judge whether it really is its problem or question.
- **`Src:` line:** where an answer comes from; dropped first when the budget is short, so the answer is never cut for it.
- **Size:** the answer fits the token budget (`budget_tokens`, 60 by default), and the fix is cut when needed. The format itself costs about 15 tokens around the fix.
- **No match:** the answer is `No memory for this problem.` (8 tokens).

## How problems are matched

1. **Signature.** The error message is normalized:
   - the line that states the error is kept, stack frames are dropped;
   - paths, `file:line:col`, versions, addresses, hashes, ids, URLs and numbers are replaced by placeholders;
   - the result is lowercased and hashed into 10 hex digits.

   The same error on another machine, in another directory, on another port or line has the same signature.
2. **Similarity.** Otherwise, entries are searched by meaning (embeddings) and by words (BM25, over the problem, the error and the answer).
   - **What the vector covers.** For a fix, the problem and the error line; for knowledge, the question **and** the answer, since the meaning is often in the answer ("Personal reminder" → "Eat before noon").
   - **When a match is kept.** Its cosine similarity is at least the model's threshold, or a lower one when the words match too.
   - **Why strict.** A wrong recall costs more tokens than it saves.
   - **Misses.** The best rejected candidate is returned to the Studio and the API (`nearest`), never to agents, to understand a miss.

### Thresholds and calibration

Similarities depend on the embedding model, so the thresholds do too. They are picked from the model name, and can be overridden in Settings → Memory (0 = automatic).

| Model | Alone | With matching words | Calibrated on |
| --- | --- | --- | --- |
| `bge-m3` (default, 1,024 dimensions) | 0.55 | 0.50 | French, English and across languages |
| `nomic-embed-text` (768) | 0.80 | 0.705 | English only |
| any other model | 0.80 | 0.705 | not calibrated: measure before relying on it |

Measured with `bge-m3` on eight entries (fixes and knowledge, French and English) against 22 queries:

| Kind of query | Cosine similarity | Result |
| --- | --- | --- |
| Paraphrases, in French, in English, and across languages ("when do I usually eat?" against a French entry) | 0.59 to 0.77, each ranked first | all recalled |
| Unrelated questions | 0.27 to 0.49 | none recalled |
| A neighbor problem ("mysql refuses connections" against a Postgres-in-Docker entry) | 0.575 | recalled, and arguably useful: the same fix applies |

With `nomic-embed-text` the same set fails: French similarities are compressed (true 0.49 to 0.79, unrelated up to 0.63), four true matches are not ranked first, and English queries do not find French entries (0.37). Use `bge-m3` for anything not in English.

To calibrate another model:
1. Save a dozen entries like yours.
2. Use Memory → Test a recall with paraphrases and with unrelated questions.
3. Set the thresholds between the weakest true match and the strongest unrelated one.

## Entries

| Field | Meaning |
| --- | --- |
| `id` | `<signature>-<variant>` |
| `kind` | `fix` or `knowledge`. Without it: `fix` when an error message is given, else `knowledge` |
| `problem`, `error`, `solution` | Problem or question, error message, fix or answer. Secrets are masked before writing: API keys (`sk-…`, `ghp_…`, `AKIA…`), JWTs, bearer tokens, `password=`, `*_SECRET_KEY=`, private keys, credentials in URLs |
| `space` | `global`, or a project name. A recall with `project` searches that project and `global` |
| `versions`, `platform` | What the fix was verified with: `expo@52`, `node@22` |
| `status` | `valid`, `recheck` or `obsolete` |
| `successes`, `failures`, `recalls` | Updated by saves, feedback and recalls |
| `author`, `origin` | `agent` (with the MCP client name) or `person` |
| `source` | Where an answer comes from: URL, document, person |
| `solve_tokens` | What the entry cost to find (solving or searching), given by the agent; 1,500 when unknown |

### Saving

- **A new problem** creates variant 1.
- **The same problem, in the same space, with the same fix** (word overlap of at least 60 %) confirms the entry: one more success, verified today.
- **The same problem with a different fix** becomes another variant. Variants are ranked by status, then by successes minus twice the failures.

### Feedback and aging

- **A failure** puts the entry in `recheck`.
- **Two failures or more, outnumbering the successes,** make it `obsolete`. Obsolete entries are never recalled.
- **A success** makes the entry `valid` again and resets its verification date.
- **Entries not verified for 180 days** are recalled with a `RECHECK` mark.

### History

Nothing is overwritten. Every save, confirmation, variant, recall, miss, feedback, edit, status change and deletion is an event in the `memory_events` collection, with a snapshot of the entry after the change. Deleting an entry removes it from recall; its history stays.

## Studio

Memory has four tabs:

- **Trends:** recalls per day (hits and misses), the most recalled problems and the most useful fixes.
- **Entries:** filter by status and space, search; the detail panel shows the fix, the error, the counts and the history, and lets you edit (as a new revision), change the status or delete.
- **Timeline:** what was learned, used, failed or forgotten, day by day.
- **Health:** entries to recheck, obsolete ones, entries not verified for six months, likely duplicates (different signatures, near-identical problems).

**Test a recall** shows exactly what an agent would receive, with its size in tokens.

## Tokens saved

For each recall that returns an entry, Lorag counts `solve_tokens` minus the tokens of the answer. Recalls of entries later reported as wrong are not counted. It is an estimate of avoided work, useful to follow over time. The roadmap criterion (tokens saved on at least 20 repeated problems) is read from these numbers.

## Command line

```sh
npm run dev 2>&1 | loragctl mem recall - --project shop --ctx node@22
loragctl mem save --error "$ERR" --fix "lsof -ti :3000 | xargs kill -9" --project shop --versions node@22
loragctl mem feedback ece4650cdd-1 --worked
loragctl mem list --status recheck
loragctl mem stats
loragctl mem export > memory.jsonl
```

## HTTP

| Method and path | |
| --- | --- |
| `POST /api/v1/memory/recall` | `{error, query, project, context, k, budget_tokens}` |
| `POST /api/v1/memory/` | save |
| `GET /api/v1/memory/?status=&space=&q=` | list |
| `GET /api/v1/memory/{id}` | entry and history |
| `PATCH /api/v1/memory/{id}` | `{problem, solution, status, tags}`, as the person |
| `DELETE /api/v1/memory/{id}` | |
| `POST /api/v1/memory/{id}/feedback` | `{worked, note}` |
| `GET /api/v1/memory/stats` | |
| `GET /api/v1/memory/events` | |
| `GET /api/v1/memory/health` | |
| `GET /api/v1/memory/export` | JSON Lines |

## Storage

Entries are documents of a VectixDB collection, `memory_<n>`, next to the chunks:
- the vector is the embedding of the problem and the error line;
- the full entry is the payload.

When the embedding dimension changes, the entries are re-embedded into the next generation before it becomes active.
