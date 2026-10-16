// Typed client for the Lorag REST API (/api/v1) and the MCP endpoint.

export type Mode = 'hybrid' | 'vector' | 'keyword';
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
  source: { path: string; filename: string; position: number; start_byte: number; end_byte: number };
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
  ocr_enabled: boolean;
  ocr_model: string;
  transcription_enabled: boolean;
  whisper_model: string;
}

export interface ModelStatus {
  ocr: { enabled: boolean; model: string };
  transcription: { enabled: boolean; model: string; model_status: Record<string, string> };
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

  settings: () => request<Settings>('GET', v1('/settings')),
  saveSettings: (s: Partial<Settings>) =>
    request<{ restart_required: boolean; settings: Settings }>('PUT', v1('/settings'), s),
  models: () => request<ModelStatus>('GET', v1('/models/status')),
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
