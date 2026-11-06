import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { AlertTriangle, Columns, CornerDownLeft, History, Play, X } from 'lucide-react';
import { api, Filter, Mode, SearchRequest, SearchResponse, SearchResult } from '../lib/api';
import { useHotkey, useStored } from '../lib/hooks';
import { dirname, location, ms, num, shortPath } from '../lib/format';
import { LocationBadge, OpenAt } from '../components/source';
import { Badge, bareInput, Button, CopyButton, cx, Drawer, Empty, InputShell, KV, modKey, NumberField, PageHeader, Segmented, Select, Toggle } from '../components/ui';

const RRF_K = 60;
const modes: { value: Mode; label: string; title: string }[] = [
  { value: 'hybrid', label: 'Hybrid', title: 'Vector and BM25, fused with reciprocal rank fusion' },
  { value: 'vector', label: 'Vector', title: 'Embedding similarity only' },
  { value: 'keyword', label: 'Keyword', title: 'BM25 full text only' },
  { value: 'exact', label: 'Exact', title: 'Chunks containing the text verbatim: exhaustive, for error messages and identifiers' },
];

interface Run {
  req: SearchRequest;
  res?: SearchResponse;
  error?: string;
}

export function PlaygroundPage() {
  const [params, setParams] = useSearchParams();
  const [query, setQuery] = useState(params.get('q') ?? '');
  const [mode, setMode] = useStored<Mode>('lorag.pg.mode', 'hybrid');
  const [topK, setTopK] = useStored('lorag.pg.topk', 10);
  const [context, setContext] = useState(0);
  const [ext, setExt] = useState('');
  const [path, setPath] = useState(params.get('path') ?? '');
  const [compare, setCompare] = useStored('lorag.pg.compare', false);
  const [history, setHistory] = useStored<string[]>('lorag.pg.history', []);
  const [runs, setRuns] = useState<Partial<Record<Mode, Run>>>({});
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<{ mode: Mode; r: SearchResult } | null>(null);
  const editor = useRef<HTMLTextAreaElement>(null);

  const filters = useMemo<Filter[]>(() => {
    const f: Filter[] = [];
    if (ext.trim()) f.push({ field: 'extension', operator: 'eq', value: ext.trim().startsWith('.') ? ext.trim() : `.${ext.trim()}` });
    if (path.trim()) f.push({ field: 'path', operator: 'eq', value: path.trim() });
    return f;
  }, [ext, path]);

  const run = async (text = query) => {
    const q = text.trim();
    if (!q) return;
    setBusy(true);
    setSelected(null);
    const targets: Mode[] = compare ? ['hybrid', 'vector', 'keyword'] : [mode];
    const out = await Promise.all(
      targets.map(async (m): Promise<[Mode, Run]> => {
        const req: SearchRequest = { query: q, top_k: topK, mode: m, context_chunks: context, filters };
        try {
          return [m, { req, res: await api.search(req) }];
        } catch (e) {
          return [m, { req, error: e instanceof Error ? e.message : String(e) }];
        }
      }),
    );
    setRuns(Object.fromEntries(out));
    setHistory([q, ...history.filter((h) => h !== q)].slice(0, 12));
    setParams((p) => {
      p.set('q', q);
      return p;
    }, { replace: true });
    setBusy(false);
  };

  // A query in the URL (from the command palette or the logs) runs at once.
  const first = useRef(true);
  useEffect(() => {
    if (first.current && params.get('q')) run(params.get('q')!);
    first.current = false;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useHotkey('mod+enter', () => run());
  useHotkey('/', () => editor.current?.focus());

  const single = compare ? undefined : runs[mode];
  const anyRun = compare ? runs.hybrid ?? runs.vector ?? runs.keyword : single;

  return (
    <div className="relative flex h-full">
      <div className="flex min-w-0 flex-1 flex-col">
        <PageHeader
          title="Playground"
          description="Run a query the way an agent would, and see why each chunk ranks where it does"
          actions={anyRun && <ExportButtons req={anyRun.req} />}
        />

        <div className="border-b border-line bg-surface px-5 py-3">
          <textarea
            ref={editor}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            rows={Math.min(8, Math.max(2, query.split('\n').length))}
            autoFocus
            spellCheck={false}
            placeholder="How do I fix ImagePullBackOff on the staging cluster?"
            className="block w-full resize-none rounded-md border border-line-strong bg-canvas px-3 py-2.5 font-mono text-[13px] leading-relaxed transition-colors placeholder:text-faint hover:border-faint/60"
          />
          <div className="mt-2.5 flex flex-wrap items-center gap-2">
            {compare ? (
              <Badge tone="accent" className="h-8 px-2.5 text-xs">
                <Columns size={13} /> All three routes
              </Badge>
            ) : (
              <Segmented value={mode} onChange={setMode} options={modes} />
            )}
            <NumberField prefix="top_k" className="w-[104px]" min={1} max={200} value={topK} onChange={setTopK} />
            <Select
              aria-label="Context chunks"
              prefix="context"
              className="w-[112px]"
              value={context}
              onChange={setContext}
              options={[0, 1, 2, 3].map((n) => ({ value: n, label: `±${n}`, text: String(n), hint: n === 0 ? 'chunk only' : `${n} each side` }))}
            />
            <InputShell prefix="type" className="w-[112px]">
              <input className={cx(bareInput, 'font-mono text-xs')} placeholder=".md" value={ext} onChange={(e) => setExt(e.target.value)} />
            </InputShell>
            <InputShell
              prefix="path"
              className="min-w-[180px] max-w-sm flex-1"
              suffix={
                path && (
                  <button className="text-faint hover:text-fg" onClick={() => setPath('')} aria-label="Clear path">
                    <X size={12} />
                  </button>
                )
              }
            >
              <input className={cx(bareInput, 'font-mono text-xs')} placeholder="exact file path" value={path} onChange={(e) => setPath(e.target.value)} />
            </InputShell>
            <Toggle checked={compare} onChange={setCompare} label="Compare routes" />
            <Button variant="primary" className="ml-auto pr-1.5" icon={<Play size={13} />} disabled={busy || !query.trim()} onClick={() => run()}>
              {busy ? 'Running…' : 'Run'}
              <span className="ml-1 flex items-center gap-0.5 rounded bg-black/15 px-1 py-0.5 font-mono text-[10px] leading-none">
                {modKey}
                <CornerDownLeft size={10} />
              </span>
            </Button>
          </div>
        </div>

        <div className="min-h-0 flex-1 overflow-auto">
          {!anyRun ? (
            <Recent history={history} onPick={(q) => { setQuery(q); run(q); }} onClear={() => setHistory([])} />
          ) : compare ? (
            <Compare runs={runs} onSelect={(m, r) => setSelected({ mode: m, r })} selected={selected?.r.chunk_id} />
          ) : (
            single && <Results run={single} onSelect={(r) => setSelected({ mode, r })} selected={selected?.r.chunk_id} />
          )}
        </div>
      </div>

      {selected && <ResultDrawer sel={selected} onClose={() => setSelected(null)} />}
    </div>
  );
}

function Recent({ history, onPick, onClear }: { history: string[]; onPick: (q: string) => void; onClear: () => void }) {
  if (!history.length)
    return (
      <Empty icon={<Play size={22} />} title="Run a query">
        Hybrid search embeds the query and matches its words with BM25, then fuses both rankings. Use Compare routes to see what each one finds on its own.
      </Empty>
    );
  return (
    <div className="p-5">
      <div className="mb-2 flex items-center gap-2 text-2xs font-medium uppercase tracking-wide text-faint">
        <History size={12} /> Recent queries
        <button className="ml-auto normal-case tracking-normal hover:text-fg" onClick={onClear}>
          Clear
        </button>
      </div>
      <div className="divide-y divide-line rounded-lg border border-line bg-surface">
        {history.map((q) => (
          <button key={q} onClick={() => onPick(q)} className="block w-full truncate px-3 py-2 text-left font-mono text-xs hover:bg-raised">
            {q}
          </button>
        ))}
      </div>
    </div>
  );
}

function RunMeta({ run }: { run: Run }) {
  if (run.error) return <div className="border-b border-bad/30 bg-bad/5 px-5 py-2 text-xs text-bad">{run.error}</div>;
  const r = run.res!;
  return (
    <>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-line px-5 py-2 font-mono text-2xs text-faint">
        <span>
          <span className="text-fg">{r.results.length}</span> results
        </span>
        {r.candidates !== r.results.length && <span>{num(r.candidates)} candidates before filters</span>}
        {r.mode !== 'keyword' && <span>embed {ms(r.timings.embed_ms)}</span>}
        <span>engine {ms(r.timings.search_ms)}</span>
        <span>total {ms(r.timings.total_ms)}</span>
        {r.mode !== run.req.mode && <Badge tone="warn">ran as {r.mode}</Badge>}
        <span className="ml-auto">{r.query_id}</span>
      </div>
      {r.warnings?.map((w) => (
        <div key={w} className="flex items-center gap-2 border-b border-warn/30 bg-warn/5 px-5 py-2 text-xs text-warn">
          <AlertTriangle size={13} /> {w}
        </div>
      ))}
    </>
  );
}

function Results({ run, onSelect, selected }: { run: Run; onSelect: (r: SearchResult) => void; selected?: string }) {
  const results = run.res?.results ?? [];
  const mode = run.res?.mode ?? run.req.mode;
  return (
    <>
      <RunMeta run={run} />
      {run.res && results.length === 0 ? (
        <Empty title="No results">Nothing matched. Try keyword mode for exact identifiers, or loosen the filters.</Empty>
      ) : (
        <table className="w-full table-fixed text-[13px]">
          <thead>
            <tr>
              <th className="th w-10 text-right">#</th>
              <th className="th w-32">Score</th>
              <th className="th w-28">
                <span className="text-vector">Vector</span>
              </th>
              <th className="th w-28">
                <span className="text-keyword">Keyword</span>
              </th>
              <th className="th">Chunk</th>
            </tr>
          </thead>
          <tbody>
            {results.map((r, i) => (
              <tr key={r.chunk_id} className={cx('row align-top', r.chunk_id === selected && 'row-active')} onClick={() => onSelect(r)}>
                <td className="td text-right font-mono text-xs text-faint">{i + 1}</td>
                <td className="td">
                  <ScoreCell r={r} mode={mode} />
                </td>
                <td className="td font-mono text-xs">
                  {r.vector ? (
                    <span className="text-vector">
                      #{r.vector.rank} <span className="text-faint">{r.vector.score.toFixed(3)}</span>
                    </span>
                  ) : (
                    <span className="text-faint">—</span>
                  )}
                </td>
                <td className="td font-mono text-xs">
                  {r.keyword ? (
                    <span className="text-keyword">
                      #{r.keyword.rank} <span className="text-faint">{r.keyword.score.toFixed(2)}</span>
                    </span>
                  ) : (
                    <span className="text-faint">—</span>
                  )}
                </td>
                <td className="td whitespace-normal py-2">
                  <div className="flex min-w-0 items-baseline gap-2 text-xs">
                    <span className="shrink-0 font-medium">{r.source.filename}</span>
                    <LocationBadge p={r.source} />
                    <span className="min-w-0 truncate font-mono text-2xs text-faint" title={r.source.path}>
                      {shortPath(dirname(r.source.path), 60)}
                    </span>
                  </div>
                  <div className="mt-0.5 line-clamp-2 text-xs text-muted">{r.content}</div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

// Parts of the normalized RRF score that come from each route.
function rrfParts(r: SearchResult) {
  const best = 2 / (RRF_K + 1);
  const v = r.vector ? 1 / (RRF_K + r.vector.rank) / best : 0;
  const k = r.keyword ? 1 / (RRF_K + r.keyword.rank) / best : 0;
  return { v, k };
}

function ScoreCell({ r, mode }: { r: SearchResult; mode: Mode }) {
  if (mode === 'exact') return <Badge tone="accent">verbatim</Badge>;
  if (mode !== 'hybrid') {
    const tone = mode === 'vector' ? 'bg-vector' : 'bg-keyword';
    const width = mode === 'vector' ? Math.max(0, Math.min(1, r.score)) : Math.min(1, r.score / 20);
    return (
      <div>
        <div className="font-mono text-xs tabular">{r.score.toFixed(3)}</div>
        <div className="mt-1 h-1 w-24 rounded-full bg-raised">
          <div className={cx('h-1 rounded-full', tone)} style={{ width: `${width * 100}%` }} />
        </div>
      </div>
    );
  }
  const { v, k } = rrfParts(r);
  return (
    <div title={`vector ${(v * 100).toFixed(1)}% + keyword ${(k * 100).toFixed(1)}% of the best possible score`}>
      <div className="font-mono text-xs tabular">{r.score.toFixed(3)}</div>
      <div className="mt-1 flex h-1 w-24 overflow-hidden rounded-full bg-raised">
        <div className="h-1 bg-vector" style={{ width: `${v * 100}%` }} />
        <div className="h-1 bg-keyword" style={{ width: `${k * 100}%` }} />
      </div>
    </div>
  );
}

function Compare({ runs, onSelect, selected }: { runs: Partial<Record<Mode, Run>>; onSelect: (m: Mode, r: SearchResult) => void; selected?: string }) {
  // Chunks found by only one route are the interesting ones: mark them.
  const seen = new Map<string, number>();
  for (const m of ['hybrid', 'vector', 'keyword'] as Mode[])
    for (const r of runs[m]?.res?.results ?? []) if (m !== 'hybrid') seen.set(r.chunk_id, (seen.get(r.chunk_id) ?? 0) + 1);
  return (
    <div className="grid min-w-[900px] grid-cols-3 divide-x divide-line">
      {(['hybrid', 'vector', 'keyword'] as Mode[]).map((m) => {
        const run = runs[m];
        return (
          <div key={m} className="min-w-0">
            <div className="sticky top-0 z-10 flex h-9 items-center gap-2 border-b border-line bg-surface px-4 text-xs font-semibold">
              <span className={m === 'vector' ? 'text-vector' : m === 'keyword' ? 'text-keyword' : 'text-fg'}>{m}</span>
              {run?.res && <span className="font-mono font-normal text-faint">{ms(run.res.timings.total_ms)}</span>}
            </div>
            {run?.error && <div className="px-4 py-3 text-xs text-bad">{run.error}</div>}
            {run?.res?.results.map((r, i) => (
              <button
                key={r.chunk_id}
                onClick={() => onSelect(m, r)}
                className={cx('block w-full border-b border-line px-4 py-2 text-left hover:bg-raised', r.chunk_id === selected && 'bg-accent/[0.07]')}
              >
                <div className="flex items-center gap-2 font-mono text-2xs">
                  <span className="w-5 text-right text-faint">{i + 1}</span>
                  <span className="truncate text-muted">{r.source.filename}</span>
                  <span className="text-faint">#{r.source.position}</span>
                  {m !== 'hybrid' && seen.get(r.chunk_id) === 1 && <Badge tone={m === 'vector' ? 'vector' : 'keyword'}>only here</Badge>}
                  <span className="ml-auto tabular text-faint">{r.score.toFixed(3)}</span>
                </div>
                <div className="mt-1 line-clamp-2 pl-7 text-xs text-muted">{r.content}</div>
              </button>
            ))}
          </div>
        );
      })}
    </div>
  );
}

function ResultDrawer({ sel, onClose }: { sel: { mode: Mode; r: SearchResult }; onClose: () => void }) {
  const { r, mode } = sel;
  const { v, k } = rrfParts(r);
  return (
    <Drawer
      open
      onClose={onClose}
      title={`${r.source.filename}${location(r.source) ? ` · ${location(r.source)}` : ` · chunk ${r.source.position}`}`}
      subtitle={r.source.section ? `${r.source.section} — ${r.source.path}` : r.source.path}
      actions={
        <Link to={`/explorer/${r.document_id}`} className="text-xs text-accent hover:underline">
          Open document
        </Link>
      }
    >
      <div className="border-b border-line px-4 py-3">
        <div className="mb-2 text-2xs font-medium uppercase tracking-wide text-faint">Why it ranks here</div>
        {mode === 'hybrid' ? (
          <div className="space-y-2 font-mono text-xs">
            <Contribution label="vector" tone="vector" rank={r.vector?.rank} extra={r.vector && `similarity ${r.vector.score.toFixed(4)}`} share={v} />
            <Contribution label="keyword" tone="keyword" rank={r.keyword?.rank} extra={r.keyword && `BM25 ${r.keyword.score.toFixed(3)}`} share={k} />
            <div className="border-t border-line pt-2 text-faint">
              score = (1/(60+r<sub>v</sub>) + 1/(60+r<sub>k</sub>)) ÷ (2/61) = <span className="text-fg">{r.score.toFixed(4)}</span>
            </div>
          </div>
        ) : mode === 'exact' ? (
          <div className="font-mono text-xs text-muted">Contains the query verbatim. The exact route is exhaustive: every matching chunk is listed, in index order.</div>
        ) : (
          <div className="font-mono text-xs text-muted">
            {mode === 'vector' ? `Cosine similarity ${r.score.toFixed(4)} (rank ${r.vector?.rank ?? '–'})` : `BM25 ${r.score.toFixed(3)} (rank ${r.keyword?.rank ?? '–'})`}
          </div>
        )}
      </div>
      <KV
        rows={[
          ['Chunk ID', <span className="flex items-center justify-between gap-2">{r.chunk_id}<CopyButton text={r.chunk_id} /></span>],
          ['Document ID', r.document_id],
          ...(location(r.source) ? ([['Position', location(r.source)]] as [string, string][]) : []),
          ...(r.source.start_byte >= 0 && r.source.end_byte > 0 ? ([['Bytes', `${num(r.source.start_byte)}–${num(r.source.end_byte)}`]] as [string, string][]) : []),
        ]}
      />
      <OpenAt documentId={r.document_id} p={r.source} className="block px-4 pt-3" />
      <div className="flex items-center justify-between px-4 pb-1 pt-3">
        <span className="text-2xs font-medium uppercase tracking-wide text-faint">Content returned to the agent</span>
        <CopyButton text={r.content} />
      </div>
      <pre className="whitespace-pre-wrap break-words px-4 pb-6 font-mono text-xs leading-relaxed">{r.content}</pre>
    </Drawer>
  );
}

function Contribution({ label, tone, rank, extra, share }: { label: string; tone: 'vector' | 'keyword'; rank?: number; extra?: string | false; share: number }) {
  return (
    <div className="flex items-center gap-3">
      <span className={cx('w-16', tone === 'vector' ? 'text-vector' : 'text-keyword')}>{label}</span>
      <span className="w-14 text-muted">{rank ? `#${rank}` : 'missed'}</span>
      <div className="h-1.5 flex-1 rounded-full bg-raised">
        <div className={cx('h-1.5 rounded-full', tone === 'vector' ? 'bg-vector' : 'bg-keyword')} style={{ width: `${share * 200}%` }} />
      </div>
      <span className="w-40 truncate text-right text-faint">{extra || '—'}</span>
    </div>
  );
}

function ExportButtons({ req }: { req: SearchRequest }) {
  const origin = window.location.origin;
  const body = JSON.stringify({ query: req.query, top_k: req.top_k, mode: req.mode, ...(req.filters?.length ? { filters: req.filters } : {}) });
  const curl = `curl -s ${origin}/api/v1/search -H 'Content-Type: application/json' -d '${body.replace(/'/g, "'\\''")}'`;
  const args: Record<string, unknown> = { query: req.query, top_k: Math.min(req.top_k, 50), mode: req.mode };
  for (const f of req.filters ?? []) args[f.field] = f.value;
  if (req.context_chunks) args.context = req.context_chunks;
  const mcpCall = JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/call', params: { name: 'search', arguments: args } }, null, 2);
  return (
    <div className="flex items-center gap-1 text-xs text-faint">
      Copy as
      <CopyButton text={curl} label="curl" />
      <CopyButton text={mcpCall} label="MCP" />
    </div>
  );
}
