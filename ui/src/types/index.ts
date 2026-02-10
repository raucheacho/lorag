export interface Document {
  id: string;
  path: string;
  filename: string;
  extension: string;
  size: number;
  mime_type: string;
  checksum: string;
  content?: string;
  chunk_count: number;
  created_at: string;
  updated_at: string;
  indexed_at?: string;
  is_deleted: boolean;
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
}

export interface Stats {
  documents: number;
  chunks: number;
  maxChunks: number;
  vector: {
    dimension: number;
    elements: number;
    layer0_nodes: number;
    max_layer: number;
  };
  watches: number;
}

export interface QueueStats {
  enabled: boolean;
  workers: number;
  running: number;
  completed: number;
  queue_depth: number;
  max_queue: number;
}

export interface Task {
  id: string;
  path: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  progress?: number;
  chunks?: number;
  error?: string;
}

export interface SearchResult {
  chunk_id: string;
  document_id: string;
  content: string;
  score: number;
  source: {
    path: string;
    filename: string;
    position: number;
    start_byte: number;
    end_byte: number;
  };
}

export interface Watch {
  id: string;
  path: string;
  recursive: boolean;
  patterns: string[];
  created_at: string;
}
