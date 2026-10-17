import { createContext, ReactNode, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import {
  Activity,
  Brain,
  Cable,
  CornerDownLeft,
  FolderInput,
  LayoutDashboard,
  Monitor,
  Moon,
  Search,
  Settings,
  Sun,
  Table2,
  TerminalSquare,
} from 'lucide-react';
import { api, Overview } from '../lib/api';
import { Resource, useHotkey, useResource, useStored } from '../lib/hooks';
import { useToast } from './toast';
import { Tooltip } from './tooltip';
import { cx, Dot, Kbd, modKey } from './ui';

// The overview is polled once and shared: sidebar status, Overview page,
// and every page that needs the MCP address or index state.
const OverviewCtx = createContext<Resource<Overview> | null>(null);
export function useOverview() {
  return useContext(OverviewCtx)!;
}

const nav = [
  { to: '/', label: 'Overview', icon: LayoutDashboard, end: true },
  { to: '/memory', label: 'Memory', icon: Brain },
  { to: '/explorer', label: 'Explorer', icon: Table2 },
  { to: '/playground', label: 'Playground', icon: TerminalSquare },
  { to: '/logs', label: 'Agent logs', icon: Activity },
  { to: '/ingestion', label: 'Ingestion', icon: FolderInput },
];

const navBottom = [
  { to: '/connect', label: 'Connect', icon: Cable },
  { to: '/settings', label: 'Settings', icon: Settings },
];

type Theme = 'system' | 'light' | 'dark';

function applyTheme(t: Theme) {
  const dark = t === 'dark' || (t === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
}

export function Shell({ children }: { children: ReactNode }) {
  const overview = useResource(api.overview, [], 5000);
  const embedder = useResource(api.embedder, [], 30000);
  const [theme, setTheme] = useStored<Theme>('lorag.theme', 'system');
  const [palette, setPalette] = useState(false);

  useEffect(() => {
    applyTheme(theme);
    if (theme !== 'system') return;
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const on = () => applyTheme('system');
    mq.addEventListener('change', on);
    return () => mq.removeEventListener('change', on);
  }, [theme]);

  useHotkey('mod+k', () => setPalette((p) => !p));

  const o = overview.data;
  const dark = document.documentElement.dataset.theme === 'dark';

  return (
    <OverviewCtx.Provider value={overview}>
      <div className="flex h-full">
        <aside className="flex w-[212px] shrink-0 flex-col border-r border-line bg-surface">
          <div className="flex h-[52px] items-center gap-2 border-b border-line px-4">
            <Logo />
            <span className="text-[13px] font-semibold tracking-tight">Lorag</span>
            <span className="ml-auto font-mono text-2xs text-faint">{o?.version ?? ''}</span>
          </div>

          <button
            onClick={() => setPalette(true)}
            className="mx-3 mt-3 flex h-8 items-center gap-2 rounded-md border border-line-strong bg-canvas px-2.5 text-xs text-faint hover:text-muted"
          >
            <Search size={13} />
            <span>Search or jump…</span>
            <span className="ml-auto flex gap-0.5">
              <Kbd>{modKey}</Kbd>
              <Kbd>K</Kbd>
            </span>
          </button>

          <nav className="mt-3 flex flex-col gap-px px-2">
            {nav.map((n) => (
              <NavItem key={n.to} {...n} />
            ))}
          </nav>
          <div className="flex-1" />
          <nav className="flex flex-col gap-px px-2 pb-2">
            {navBottom.map((n) => (
              <NavItem key={n.to} {...n} />
            ))}
          </nav>

          <div className="border-t border-line px-4 py-3 text-xs">
            <StatusRow
              label="Engine"
              tone={!o ? 'neutral' : o.index.rebuilding ? 'warn' : o.in_sync ? 'ok' : 'warn'}
              value={!o ? '…' : o.index.rebuilding ? 'rebuilding' : o.in_sync ? `VectixDB ${o.core_version}` : 'out of sync'}
            />
            <StatusRow
              label="Embedder"
              tone={!embedder.data ? 'neutral' : embedder.data.ok ? 'ok' : 'bad'}
              value={!embedder.data ? '…' : embedder.data.ok ? o?.embedder.model ?? 'ok' : 'unavailable'}
              title={embedder.data?.error}
            />
            <StatusRow label="Server" tone={overview.error ? 'bad' : o ? 'ok' : 'neutral'} value={overview.error ? 'unreachable' : o?.addr ?? '…'} />
            <div className="mt-2 flex items-center justify-between">
              <span className="text-faint">Theme</span>
              <div className="flex rounded-md border border-line p-0.5" role="radiogroup" aria-label="Theme">
                {(
                  [
                    ['system', Monitor, 'System'],
                    ['light', Sun, 'Light'],
                    ['dark', Moon, 'Dark'],
                  ] as const
                ).map(([t, Icon, label]) => (
                  <Tooltip key={t} label={label}>
                    <button
                      role="radio"
                      aria-checked={theme === t}
                      onClick={() => setTheme(t)}
                      className={cx('flex h-5 w-6 items-center justify-center rounded', theme === t ? 'bg-raised text-fg' : 'text-faint hover:text-fg')}
                    >
                      <Icon size={12} />
                    </button>
                  </Tooltip>
                ))}
              </div>
            </div>
          </div>
        </aside>

        <main className="flex min-w-0 flex-1 flex-col overflow-hidden">{children}</main>
      </div>
      {palette && <CommandPalette onClose={() => setPalette(false)} toggleTheme={() => setTheme(dark ? 'light' : 'dark')} />}
    </OverviewCtx.Provider>
  );
}

function Logo() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden className="text-accent">
      <rect x="3" y="3" width="18" height="18" rx="5" fill="currentColor" opacity="0.18" />
      <path d="M8 6.5v11h8" stroke="currentColor" strokeWidth="2.4" fill="none" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function NavItem({ to, label, icon: Icon, end }: { to: string; label: string; icon: typeof Search; end?: boolean }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        cx(
          'flex h-8 items-center gap-2.5 rounded-md px-2.5 text-[13px] transition-colors',
          isActive ? 'bg-raised font-medium text-fg' : 'text-muted hover:bg-raised/60 hover:text-fg',
        )
      }
    >
      <Icon size={15} strokeWidth={1.8} />
      {label}
    </NavLink>
  );
}

function StatusRow({ label, value, tone, title }: { label: string; value: string; tone: 'ok' | 'warn' | 'bad' | 'neutral'; title?: string }) {
  return (
    <div className="flex h-6 items-center gap-2" title={title}>
      <Dot tone={tone} />
      <span className="text-faint">{label}</span>
      <span className="ml-auto truncate font-mono text-2xs text-muted">
        {value}
      </span>
    </div>
  );
}

interface Command {
  id: string;
  group: string;
  label: string;
  hint?: string;
  run: () => void | Promise<unknown>;
}

function CommandPalette({ onClose, toggleTheme }: { onClose: () => void; toggleTheme: () => void }) {
  const navigate = useNavigate();
  const toast = useToast();
  const overview = useOverview();
  const [q, setQ] = useState('');
  const [sel, setSel] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  const commands = useMemo<Command[]>(() => {
    const go = (to: string) => () => navigate(to);
    const act = (f: () => Promise<unknown>, done: string) => async () => {
      try {
        await f();
        toast.ok(done);
        overview.reload();
      } catch (e) {
        toast.error(e);
      }
    };
    const mcpURL = `${window.location.origin}/mcp`;
    const list: Command[] = [
      ...[...nav, ...navBottom].map((n) => ({ id: `go:${n.to}`, group: 'Go to', label: n.label, run: go(n.to) })),
      { id: 'act:rebuild', group: 'Index', label: 'Rebuild index from the document store', run: act(api.rebuild, 'Rebuild started') },
      { id: 'act:compact', group: 'Index', label: 'Compact index (drop deleted chunks)', run: act(api.compact, 'Index compacted') },
      { id: 'act:checkpoint', group: 'Index', label: 'Checkpoint (trim the write-ahead log)', run: act(api.checkpoint, 'Checkpoint written') },
      { id: 'act:mcp', group: 'Connect', label: 'Copy MCP endpoint URL', hint: mcpURL, run: () => navigator.clipboard.writeText(mcpURL).then(() => toast.ok('MCP URL copied')) },
      { id: 'act:theme', group: 'Preferences', label: 'Toggle dark / light theme', run: toggleTheme },
    ];
    const needle = q.trim().toLowerCase();
    const matched = needle ? list.filter((c) => `${c.group} ${c.label}`.toLowerCase().includes(needle)) : list;
    if (needle) {
      matched.unshift({
        id: 'query',
        group: 'Search',
        label: `Search “${q.trim()}” in Playground`,
        run: () => navigate(`/playground?q=${encodeURIComponent(q.trim())}`),
      });
    }
    return matched;
  }, [q, navigate, toast, overview, toggleTheme]);

  useEffect(() => setSel(0), [q]);
  useEffect(() => {
    listRef.current?.querySelector(`[data-index="${sel}"]`)?.scrollIntoView({ block: 'nearest' });
  }, [sel]);

  const run = (c: Command | undefined) => {
    if (!c) return;
    onClose();
    c.run();
  };

  let lastGroup = '';
  return (
    <div className="fade-in fixed inset-0 z-40 flex items-start justify-center bg-black/40 pt-[14vh]" onMouseDown={onClose}>
      <div
        className="pop w-[560px] max-w-[calc(100vw-32px)] overflow-hidden rounded-xl border border-line-strong bg-surface shadow-2xl"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 border-b border-line px-3">
          <Search size={15} className="text-faint" />
          <input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') onClose();
              if (e.key === 'ArrowDown') {
                e.preventDefault();
                setSel((s) => Math.min(s + 1, commands.length - 1));
              }
              if (e.key === 'ArrowUp') {
                e.preventDefault();
                setSel((s) => Math.max(s - 1, 0));
              }
              if (e.key === 'Enter') {
                // Keep the key from reaching the field the command focuses.
                e.preventDefault();
                run(commands[sel]);
              }
            }}
            placeholder="Type a command, a page, or a query to search…"
            className="h-12 flex-1 bg-transparent text-sm outline-none placeholder:text-faint"
          />
          <Kbd>esc</Kbd>
        </div>
        <div ref={listRef} className="max-h-[50vh] overflow-y-auto p-1.5">
          {commands.length === 0 && <div className="px-3 py-6 text-center text-xs text-faint">No match</div>}
          {commands.map((c, i) => {
            const header = c.group !== lastGroup;
            lastGroup = c.group;
            return (
              <div key={c.id}>
                {header && <div className="px-2.5 pb-1 pt-2 text-2xs font-medium uppercase tracking-wide text-faint">{c.group}</div>}
                <button
                  data-index={i}
                  onMouseMove={() => setSel(i)}
                  onClick={() => run(c)}
                  className={cx('flex h-8 w-full items-center gap-2 rounded-md px-2.5 text-left text-[13px]', i === sel ? 'bg-raised text-fg' : 'text-muted')}
                >
                  <span className="truncate">{c.label}</span>
                  {c.hint && <span className="ml-auto truncate font-mono text-2xs text-faint">{c.hint}</span>}
                  {i === sel && !c.hint && <CornerDownLeft size={12} className="ml-auto text-faint" />}
                </button>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
