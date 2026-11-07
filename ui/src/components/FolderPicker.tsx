import { useEffect, useState } from 'react';
import { ArrowUp, ChevronRight, Folder, FolderOpen, X } from 'lucide-react';
import { api } from '../lib/api';
import { useResource } from '../lib/hooks';
import { Button, ErrorLine, Toggle, cx } from './ui';

// FolderPicker browses the folders of the machine running Lorag. A browser
// cannot give a page the path of a folder, so the server lists them.
export function FolderPicker({ initial, onPick, onClose }: { initial?: string; onPick: (path: string) => void; onClose: () => void }) {
  const [path, setPath] = useState(initial?.trim() || '');
  const [hidden, setHidden] = useState(false);
  const listing = useResource(() => api.dirs(path, hidden), [path, hidden]);
  const d = listing.data;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose();
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  // An initial path that does not exist (a typo, a file): start from home.
  useEffect(() => {
    if (listing.error && path === initial?.trim()) setPath('');
  }, [listing.error, path, initial]);

  const crumbs = d ? breadcrumbs(d.path) : [];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div role="dialog" aria-modal="true" aria-label="Choose a folder" className="flex max-h-[80vh] w-full max-w-xl flex-col overflow-hidden rounded-lg border border-line-strong bg-surface shadow-2xl">
        <header className="flex items-center gap-2 border-b border-line px-4 py-3">
          <FolderOpen size={15} className="text-accent" />
          <div className="flex-1 text-[13px] font-semibold">Choose a folder</div>
          <Button variant="ghost" size="sm" onClick={onClose} icon={<X size={14} />} tip="Close" />
        </header>

        {d && d.shortcuts.length > 0 && (
          <div className="flex flex-wrap gap-1.5 border-b border-line px-4 py-2">
            {d.shortcuts.map((s) => (
              <button
                key={s.path}
                onClick={() => setPath(s.path)}
                className={cx(
                  'rounded-md border px-2 py-0.5 text-xs transition-colors',
                  s.path === d.path ? 'border-accent/50 bg-accent/10 text-fg' : 'border-line text-muted hover:bg-raised hover:text-fg',
                )}
              >
                {s.name}
              </button>
            ))}
          </div>
        )}

        <div className="flex items-center gap-1 border-b border-line px-2 py-1.5">
          <Button variant="ghost" size="sm" disabled={!d?.parent} onClick={() => d?.parent && setPath(d.parent)} icon={<ArrowUp size={14} />} tip="Parent folder" />
          <div className="flex min-w-0 flex-1 items-center overflow-x-auto font-mono text-xs text-muted">
            {crumbs.map((c, i) => (
              <span key={c.path} className="flex shrink-0 items-center">
                {i > 0 && <ChevronRight size={12} className="mx-0.5 text-faint" />}
                <button onClick={() => setPath(c.path)} className={cx('rounded px-1 py-0.5 hover:bg-raised', i === crumbs.length - 1 && 'text-fg')}>
                  {c.name}
                </button>
              </span>
            ))}
          </div>
        </div>

        <ErrorLine error={listing.error} />
        <ul className="min-h-[200px] flex-1 overflow-y-auto py-1">
          {d?.dirs.map((dir) => (
            <li key={dir.path}>
              <button
                onClick={() => setPath(dir.path)}
                onDoubleClick={() => onPick(dir.path)}
                className="flex w-full items-center gap-2 px-4 py-1.5 text-left text-[13px] hover:bg-raised"
              >
                <Folder size={14} className="shrink-0 text-faint" />
                <span className="truncate">{dir.name}</span>
              </button>
            </li>
          ))}
          {d && d.dirs.length === 0 && <li className="px-4 py-6 text-center text-xs text-faint">No subfolder</li>}
        </ul>

        <footer className="flex flex-wrap items-center gap-3 border-t border-line px-4 py-3">
          <Toggle checked={hidden} onChange={setHidden} label="Hidden folders" />
          <div className="flex-1" />
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" disabled={!d} onClick={() => d && onPick(d.path)}>
            Choose this folder
          </Button>
        </footer>
      </div>
    </div>
  );
}

function breadcrumbs(path: string): { name: string; path: string }[] {
  const parts = path.split('/').filter(Boolean);
  const out = [{ name: '/', path: '/' }];
  let acc = '';
  for (const p of parts) {
    acc += '/' + p;
    out.push({ name: p, path: acc });
  }
  return out;
}
