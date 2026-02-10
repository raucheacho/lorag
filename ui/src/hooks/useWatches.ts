import { useState, useEffect, useCallback } from 'react';
import type { Watch } from '../types';

const API_BASE = '/api/v1';

export function useWatches() {
  const [watches, setWatches] = useState<Watch[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchWatches = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/watches`);
      if (!res.ok) throw new Error('Failed to fetch watches');
      const data = await res.json();
      setWatches(data.watches || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchWatches();
  }, [fetchWatches]);

  const addWatch = async (path: string, recursive: boolean): Promise<boolean> => {
    try {
      const res = await fetch(`${API_BASE}/watches`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path, recursive, patterns: [] }),
      });
      
      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.error || 'Failed to add watch');
      }
      
      await fetchWatches();
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add watch');
      return false;
    }
  };

  const removeWatch = async (id: string): Promise<boolean> => {
    try {
      const res = await fetch(`${API_BASE}/watches/${id}`, {
        method: 'DELETE',
      });
      
      if (!res.ok) throw new Error('Failed to remove watch');
      
      await fetchWatches();
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove watch');
      return false;
    }
  };

  return { watches, loading, error, addWatch, removeWatch, refetch: fetchWatches };
}
