// Typed client for the Lorag REST API (/api/v1) and the MCP endpoint.

export type Mode = 'hybrid' | 'vector' | 'keyword' | 'exact';
export type Source = 'mcp' | 'api' | 'studio';

export interface IndexStats {
  core_version: string;
  collection: string;
  chunks: number;
  dim: number;
  metric: string;
  quantized: boolean;
  disk_bytes: number;
  wal_bytes: number;
  last_seq: number;
  checkpoint_seq: number;
  rebuilding: boolean;
}

export interface QueueStats {
  enabled: boolean;
  workers?: number;
  running?: number;
  completed?: number;
  queue_depth?: number;
  max_queue?: number;
}

export interface ActivitySummary {
  total: number;
  by_source: Partial<Record<Source, number>>;
  p50_ms: number;
  p95_ms: number;
  errors: number;
  empty: number;
  clients: Record<string, number>;
}

export interface Overview {
  version: string;
  core_version: string;
  uptime_s: number;
  data_dir: string;
  addr: string;
  documents: number;
  chunks: number;
  max_chunks: number;
  db_bytes: number;
  index: IndexStats;
  in_sync: boolean;
  embedder: { provider: string; model: string; url: string; dim: number };
  queue: QueueStats;
  watches: number;
  activity: ActivitySummary;
}

export interface EmbedderStatus {
  ok: boolean;
  latency_ms?: number;
  expected?: number;
  dim?: number;
  error?: string;
}

export interface Signal {
  rank: number;
  score: number;
}

export interface SearchResult {
  chunk_id: string;
  document_id: string;
  content: string;
  score: number;
  vector?: Signal;
  keyword?: Signal;
  source: SourceRef;
}

// Where a chunk comes from; the position fields are set when the source has
// pages, lines or a timeline.
export interface SourceRef {
  path: string;
  filename: string;
  position: number;
  start_byte: number;
  end_byte: number;
  kind?: string;
  page?: number;
  line_start?: number;
  line_end?: number;
  start_ms?: number;
  end_ms?: number;
  section?: string;
}

export interface Filter {
  field: string;
  operator: string;
  value: string;
}

export interface SearchRequest {
  query: string;
  top_k: number;
  mode: Mode;
  context_chunks?: number;
  filters?: Filter[];
}

export interface SearchResponse {
  query_id: string;
  mode: Mode;
  results: SearchResult[];
  candidates: number;
  timings: { embed_ms: number; search_ms: number; total_ms: number };
  warnings?: string[];
  total: number;
}

export interface ActivityHit {
  chunk_id: string;
  document_id: string;
  filename: string;
  path: string;
  position: number;
  score: number;
}

export interface ActivityQuery {
  id: string;
  time: string;
  source: Source;
  client?: string;
  tool?: string;
  query: string;
  mode: Mode;
  top_k: number;
  duration_ms: number;
  error?: string;
  hits: ActivityHit[] | null;
}

export interface Document {
  id: string;
  path: string;
  filename: string;
  extension: string;
  size: number;
  mime_type: string;
  checksum: string;
  metadata: Record<string, string> | null;
  created_at: string;
  updated_at: string;
  indexed_at?: string;
  chunk_count: number;
  kind?: string;
}

export interface Chunk {
  id: string;
  document_id: string;
  content: string;
  position: number;
  start_byte: number;
  end_byte: number;
  tokens: number;
  created_at: string;
  dim: number;
  indexed: boolean;
  page?: number;
  line_start?: number;
  line_end?: number;
  start_ms?: number;
  end_ms?: number;
  section?: string;
}

export type MemoryStatus = 'valid' | 'recheck' | 'obsolete';

export type MemoryKind = 'fix' | 'knowledge' | 'fact';

export interface MemoryEntry {
  id: string;
  signature: string;
  kind: MemoryKind;
  source?: string;
  variant: number;
  revision: number;
  problem: string;
  error?: string;
  solution: string;
  space: string;
  project?: string;
  platform?: string;
  versions?: string[] | null;
  tags?: string[] | null;
  status: MemoryStatus;
  successes: number;
  failures: number;
  recalls: number;
  author: 'agent' | 'person';
  origin?: string;
  solve_tokens: number;
  verified_at: string;
  created_at: string;
  updated_at: string;
}

export interface MemoryEvent {
  id: string;
  at: string;
  type: 'create' | 'variant' | 'confirm' | 'recall' | 'miss' | 'feedback' | 'edit' | 'status' | 'delete';
  entry?: string;
  actor?: string;
  summary?: string;
  tokens?: number;
  worked?: boolean;
  query?: string;
  after?: MemoryEntry;
}

export interface MemoryDay {
  date: string;
  saves: number;
  hits: number;
  misses: number;
  worked: number;
  failed: number;
}

export interface MemoryStats {
  entries: number;
  by_status: Partial<Record<MemoryStatus, number>>;
  by_space: Record<string, number>;
  by_kind: Partial<Record<MemoryKind, number>>;
  recalls: number;
  hits: number;
  misses: number;
  hit_rate: number;
  tokens_returned: number;
  avg_recall_tokens: number;
  tokens_saved_estimate: number;
  saves: number;
  feedback: { worked: number; failed: number };
  daily: MemoryDay[];
  recurring: { id: string; problem: string; count: number }[] | null;
  useful: { id: string; problem: string; count: number }[] | null;
  thresholds?: Thresholds;
}

export interface MemoryHealth {
  recheck: MemoryEntry[];
  obsolete: MemoryEntry[];
  stale: MemoryEntry[];
  duplicates: [MemoryEntry, MemoryEntry][];
}

export interface RecallMatch {
  entry: MemoryEntry;
  by: 'signature' | 'similarity';
  similarity?: number;
  status: MemoryStatus;
  reason?: string;
}

export interface Thresholds {
  similarity: number;
  similarity_text: number;
  calibrated: boolean;
}

export interface RecallResult {
  text: string;
  tokens: number;
  matches: RecallMatch[] | null;
  nearest?: RecallMatch;
  thresholds: Thresholds;
}

export interface SaveMemoryRequest {
  kind: MemoryKind;
  source?: string;
  problem: string;
  solution: string;
  error?: string;
  project?: string;
  space?: string;
  platform?: string;
  versions?: string[];
  tags?: string[];
}

export interface IngestEvent {
  time: string;
  path: string;
  document_id?: string;
  status: 'indexed' | 'unchanged' | 'failed';
  chunks: number;
  duration_ms: number;
  error?: string;
}

export interface Ingestion {
  queue: QueueStats;
  running: { id: string; path: string; started_at: string }[] | null;
  events: IngestEvent[];
}

export interface Watch {
  id: string;
  path: string;
  recursive: boolean;
  patterns: string[] | null;
  created_at: string;
}

export interface Settings {
  host: string;
  port: number;
  embedding_provider: string;
  embedding_model: string;
  embedding_url: string;
  embedding_dim: number;
  vector_metric: 'cosine' | 'l2' | 'dot';
  vector_quantized: boolean;
  vector_m: number;
  vector_ef: number;
  vector_ef_search: number;
  bm25_k1: number;
  bm25_b: number;
  max_chunks: number;
  chunk_size: number;
  chunk_overlap: number;
  index_workers: number;
  default_template: string;
  memory_min_similarity: number;
  memory_min_similarity_text: number;
  ocr_enabled: boolean;
  ocr_model: string;
  transcription_enabled: boolean;
  whisper_model: string;
}

export type WhisperStatus = 'not_downloaded' | 'downloading' | 'ready' | 'error';

export interface DownloadProgress {
  downloaded: number;
  total: number; // 0 while unknown
  error?: string;
}

export interface ModelStatus {
  ocr: { enabled: boolean; model: string };
  transcription: {
    enabled: boolean;
    model: string;
    model_status: Record<string, WhisperStatus>;
    progress?: Record<string, DownloadProgress>;
  };
}

export interface McpTool {
  name: string;
  title?: string;
  description: string;
  inputSchema: { properties?: Record<string, { type?: string; description?: string; default?: unknown; enum?: string[] }>; required?: string[] };
  annotations?: Record<string, boolean>;
}

export class ApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json', 'X-Lorag-Client': 'studio' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  let data: unknown = undefined;
  try {
    data = text ? JSON.parse(text) : undefined;
  } catch {
    data = text;
  }
  if (!res.ok) {
    const msg = (data as { error?: string } | undefined)?.error ?? `${res.status} ${res.statusText}`;
    throw new ApiError(msg, res.status);
  }
  return data as T;
}

const v1 = (p: string) => `/api/v1${p}`;
const qs = (params: Record<string, string | number | undefined>) => {
  const s = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== '') s.set(k, String(v));
  const out = s.toString();
  return out ? `?${out}` : '';
};

export const api = {
  overview: () => request<Overview>('GET', v1('/overview')),
  embedder: () => request<EmbedderStatus>('GET', v1('/embedder')),

  search: (r: SearchRequest) => request<SearchResponse>('POST', v1('/search'), r),

  activity: (source?: Source, limit = 200) =>
    request<{ queries: ActivityQuery[]; summary: ActivitySummary }>('GET', v1(`/activity/${qs({ source, limit })}`)),

  documents: (p: { q?: string; ext?: string; sort?: string; offset?: number; limit?: number }) =>
    request<{ documents: Document[]; total: number; offset: number; limit: number }>('GET', v1(`/documents/${qs(p)}`)),
  document: (id: string) => request<Document>('GET', v1(`/documents/${id}`)),
  chunks: (id: string) => request<{ chunks: Chunk[]; total: number }>('GET', v1(`/documents/${id}/chunks`)),
  deleteDocument: (id: string) => request<unknown>('DELETE', v1(`/documents/${id}`)),
  reindexDocument: (id: string) => request<Document>('POST', v1(`/documents/${id}/reindex`)),

  ingestion: (limit = 100) => request<Ingestion>('GET', v1(`/ingestion${qs({ limit })}`)),
  indexFile: (path: string) =>
    request<{ message: string; chunks: number; task_id?: string }>('POST', v1('/index/file'), { path, async: true }),
  indexDirectory: (path: string, recursive: boolean) =>
    request<{ message: string }>('POST', v1('/index/directory'), { path, recursive, async: true }),

  watches: () => request<{ watches: Watch[] }>('GET', v1('/watches/')),
  addWatch: (path: string, recursive: boolean) => request<Watch>('POST', v1('/watches/'), { path, recursive }),
  removeWatch: (id: string) => request<unknown>('DELETE', v1(`/watches/${id}`)),

  rebuild: () => request<unknown>('POST', v1('/index/rebuild')),
  compact: () => request<IndexStats>('POST', v1('/index/compact')),
  checkpoint: () => request<IndexStats>('POST', v1('/index/save')),

  memory: (p: { kind?: string; status?: string; space?: string; q?: string; limit?: number }) =>
    request<{ entries: MemoryEntry[]; total: number }>('GET', v1(`/memory/${qs(p)}`)),
  memoryEntry: (id: string) => request<{ entry: MemoryEntry | null; history: MemoryEvent[] }>('GET', v1(`/memory/${id}`)),
  saveMemory: (r: SaveMemoryRequest) => request<{ entry: MemoryEntry; action: string }>('POST', v1('/memory/'), r),
  recall: (r: { query?: string; error?: string; project?: string; k?: number; budget_tokens?: number; context?: Record<string, string> }) =>
    request<RecallResult>('POST', v1('/memory/recall'), r),
  updateMemory: (id: string, e: { kind?: MemoryKind; source?: string; problem?: string; solution?: string; status?: MemoryStatus; tags?: string[] }) =>
    request<MemoryEntry>('PATCH', v1(`/memory/${id}`), e),
  deleteMemory: (id: string) => request<unknown>('DELETE', v1(`/memory/${id}`)),
  memoryFeedback: (id: string, worked: boolean) => request<MemoryEntry>('POST', v1(`/memory/${id}/feedback`), { worked }),
  memoryStats: () => request<MemoryStats>('GET', v1('/memory/stats')),
  memoryEvents: (limit = 300) => request<{ events: MemoryEvent[] }>('GET', v1(`/memory/events${qs({ limit })}`)),
  memoryHealth: () => request<MemoryHealth>('GET', v1('/memory/health')),
  memoryExportURL: v1('/memory/export'),

  fileURL: (documentId: string) => v1(`/documents/${documentId}/file`),

  settings: () => request<Settings>('GET', v1('/settings')),
  saveSettings: (s: Partial<Settings>) =>
    request<{ restart_required: boolean; settings: Settings }>('PUT', v1('/settings'), s),
  models: () => request<ModelStatus>('GET', v1('/models/status')),
  downloadWhisper: (model: string) => request<unknown>('POST', v1(`/models/whisper/${model}/download`)),
};

// MCP over HTTP, used by the Connect page to list tools and test the server.
let mcpId = 0;
export async function mcp<T>(method: string, params?: unknown): Promise<T> {
  const res = await fetch('/mcp', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: ++mcpId, method, params }),
  });
  if (!res.ok) throw new ApiError(`${res.status} ${res.statusText}`, res.status);
  const data = (await res.json()) as { result?: T; error?: { message: string } };
  if (data.error) throw new ApiError(data.error.message, 400);
  return data.result as T;
}

export interface UploadResult {
  name: string;
  path?: string;
  size?: number;
  task_id?: string;
  error?: string;
}

// upload sends files with progress (fetch cannot report upload progress).
export function upload(files: File[], onProgress: (fraction: number) => void): Promise<UploadResult[]> {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    for (const f of files) form.append('files', f, f.name);
    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/v1/upload');
    xhr.upload.onprogress = (e) => e.lengthComputable && onProgress(e.loaded / e.total);
    xhr.onload = () => {
      try {
        const data = JSON.parse(xhr.responseText);
        if (xhr.status >= 400) reject(new ApiError(data.error ?? xhr.statusText, xhr.status));
        else resolve(data.files ?? []);
      } catch {
        reject(new ApiError(xhr.statusText || 'upload failed', xhr.status));
      }
    };
    xhr.onerror = () => reject(new ApiError('network error', 0));
    xhr.send(form);
  });
}
