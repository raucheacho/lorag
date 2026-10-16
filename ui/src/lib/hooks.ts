import { useCallback, useEffect, useRef, useState } from 'react';

export interface Resource<T> {
  data: T | undefined;
  error: string | undefined;
  loading: boolean;
  reload: () => void;
}

// useResource loads fn() when deps change and, with pollMs, refreshes it in
// the background while the tab is visible. Stale responses are dropped.
export function useResource<T>(fn: () => Promise<T>, deps: unknown[], pollMs?: number): Resource<T> {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);
  const seq = useRef(0);
  const fnRef = useRef(fn);
  fnRef.current = fn;

  const load = useCallback((background: boolean) => {
    const id = ++seq.current;
    if (!background) setLoading(true);
    fnRef
      .current()
      .then((d) => {
        if (id !== seq.current) return;
        setData(d);
        setError(undefined);
      })
      .catch((e: unknown) => {
        if (id !== seq.current) return;
        setError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (id === seq.current) setLoading(false);
      });
  }, []);

  useEffect(() => {
    load(false);
    if (!pollMs) return;
    const t = setInterval(() => {
      if (document.visibilityState === 'visible') load(true);
    }, pollMs);
    return () => clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, pollMs]);

  return { data, error, loading, reload: () => load(true) };
}

// useStored keeps a value in localStorage; storage failures are ignored.
export function useStored<T>(key: string, initial: T): [T, (v: T) => void] {
  const [value, setValue] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key);
      return raw === null ? initial : (JSON.parse(raw) as T);
    } catch {
      return initial;
    }
  });
  const set = useCallback(
    (v: T) => {
      setValue(v);
      try {
        localStorage.setItem(key, JSON.stringify(v));
      } catch {
        /* private mode */
      }
    },
    [key],
  );
  return [value, set];
}

export const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);

// useHotkey binds a shortcut such as "mod+k" or "/" (ignored while typing,
// except for shortcuts using mod).
export function useHotkey(combo: string, handler: (e: KeyboardEvent) => void) {
  const ref = useRef(handler);
  ref.current = handler;
  useEffect(() => {
    const parts = combo.toLowerCase().split('+');
    const key = parts[parts.length - 1];
    const mod = parts.includes('mod');
    const onKey = (e: KeyboardEvent) => {
      if (e.key.toLowerCase() !== key) return;
      const hasMod = isMac ? e.metaKey : e.ctrlKey;
      if (mod !== hasMod) return;
      const t = e.target as HTMLElement | null;
      const typing = t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT' || t.isContentEditable);
      if (typing && !mod) return;
      e.preventDefault();
      ref.current(e);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [combo]);
}
