import { useState, useEffect, useCallback } from 'react';
import type { Stats, QueueStats, Document, SearchResult } from '../types';

const API_BASE = '/api/v1';

export function useStats(refreshInterval = 5000) {
  const [stats, setStats] = useState<Stats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchStats = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/stats`);
      if (!res.ok) throw new Error('Failed to fetch stats');
      const data = await res.json();
      setStats(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStats();
    const interval = setInterval(fetchStats, refreshInterval);
    return () => clearInterval(interval);
  }, [fetchStats, refreshInterval]);

  return { stats, loading, error, refetch: fetchStats };
}

export function useQueueStats(refreshInterval = 1000) {
  const [stats, setStats] = useState<QueueStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const res = await fetch(`${API_BASE}/queue/stats`);
        if (res.ok) {
          const data = await res.json();
          setStats(data);
        }
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
    const interval = setInterval(fetchStats, refreshInterval);
    return () => clearInterval(interval);
  }, [refreshInterval]);

  return { stats, loading };
}

export function useDocuments(limit = 50) {
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchDocuments = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/documents?limit=${limit}`);
      if (res.ok) {
        const data = await res.json();
        setDocuments(data.documents || []);
      }
    } finally {
      setLoading(false);
    }
  }, [limit]);

  useEffect(() => {
    fetchDocuments();
  }, [fetchDocuments]);

  return { documents, loading, refetch: fetchDocuments };
}

export async function searchDocuments(query: string, top_k = 10, min_score = 0): Promise<SearchResult[]> {
  const res = await fetch(`${API_BASE}/search`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query, top_k, min_score }),
  });
  
  if (!res.ok) throw new Error('Search failed');
  const data = await res.json();
  return data.results || [];
}

export async function getSettings(): Promise<Record<string, any>> {
  const res = await fetch(`${API_BASE}/settings`);
  if (!res.ok) throw new Error('Failed to get settings');
  return res.json();
}

export async function updateSettings(settings: Record<string, any>): Promise<void> {
  const res = await fetch(`${API_BASE}/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  });
  if (!res.ok) throw new Error('Failed to update settings');
}

export async function indexFile(path: string): Promise<void> {
  const res = await fetch(`${API_BASE}/index/file`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  });
  
  if (!res.ok) throw new Error('Indexing failed');
}

export async function indexDirectory(path: string, recursive = true): Promise<void> {
  const res = await fetch(`${API_BASE}/index/directory`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, recursive }),
  });
  
  if (!res.ok) throw new Error('Indexing failed');
}

export async function deleteDocument(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/documents/${id}`, {
    method: 'DELETE',
  });
  
  if (!res.ok) throw new Error('Delete failed');
}

export async function reindexDocument(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/documents/${id}/reindex`, {
    method: 'POST',
  });
  
  if (!res.ok) throw new Error('Reindex failed');
}

export async function reindexMedia(types: string[] = ['audio', 'video', 'image']): Promise<{count: number, types: string[]}> {
  const res = await fetch(`${API_BASE}/documents/reindex-media`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ types }),
  });
  
  if (!res.ok) throw new Error('Reindex media failed');
  return res.json();
}

export async function getModelStatus(): Promise<{
  ocr: { enabled: boolean; model: string };
  transcription: { enabled: boolean; model: string; model_status: Record<string, string> };
}> {
  const res = await fetch(`${API_BASE}/models/status`);
  if (!res.ok) throw new Error('Failed to get model status');
  return res.json();
}
