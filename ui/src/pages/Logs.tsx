import { useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { Activity, Play, Search } from 'lucide-react';
import { ActivityQuery, api, Source } from '../lib/api';
import { useResource } from '../lib/hooks';
import { clock, dateTime, dirname, ms, num, shortPath } from '../lib/format';
import { Badge, Button, cx, Drawer, Empty, ErrorLine, Input, KV, PageHeader, Segmented, SkeletonRows, Stat } from '../components/ui';

export function SourceBadge({ q }: { q: Pick<ActivityQuery, 'source' | 'client' | 'tool'> }) {
  const tone = q.source === 'mcp' ? 'accent' : q.source === 'studio' ? 'neutral' : 'keyword';
  return (
    <span className="flex min-w-0 items-center gap-1.5">
      <Badge tone={tone}>{q.source}</Badge>
      {q.source === 'mcp' && q.client && <span className="truncate font-mono text-2xs text-muted">{q.client}</span>}
    </span>
  );
}

type SourceFilter = '' | Source;

export function LogsPage() {
  const [params, setParams] = useSearchParams();
  const [source, setSource] = useState<SourceFilter>('');
  const [text, setText] = useState('');
  const activity = useResource(() => api.activity(source || undefined, 1000), [source], 3000);
  const selectedId = params.get('id');

  const queries = useMemo(() => {
    const needle = text.trim().toLowerCase();
    const qs = activity.data?.queries ?? [];
    return needle ? qs.filter((q) => q.query.toLowerCase().includes(needle) || (q.client ?? '').toLowerCase().includes(needle)) : qs;
  }, [activity.data, text]);
  const selected = activity.data?.queries.find((q) => q.id === selectedId);
  const s = activity.data?.summary;

  const select = (id: string | null) =>
    setParams((p) => {
      if (id) p.set('id', id);
      else p.delete('id');
      return p;
    });

  return (
    <div className="flex h-full">
      <div className="flex min-w-0 flex-1 flex-col">
        <PageHeader title="Agent logs" description="Every search served since the server started: who asked, what came back, how fast (last 1,000 kept in memory)" />
        {s && (
          <div className="grid grid-cols-2 divide-x divide-line border-b border-line bg-surface md:grid-cols-5">
            <Stat
              label="Queries"
              value={num(s.total)}
              hint={Object.entries(s.by_source)
                .filter(([, n]) => n)
                .map(([k, n]) => `${n} ${k}`)
                .join(' · ')}
            />
            <Stat label="Latency p50" value={ms(s.p50_ms)} />
            <Stat label="Latency p95" value={ms(s.p95_ms)} />
            <Stat label="Empty results" value={num(s.empty)} hint={s.total ? `${Math.round((s.empty / s.total) * 100)}% of queries` : undefined} tone={s.empty > 0 ? 'warn' : undefined} />
            <Stat label="Errors" value={num(s.errors)} tone={s.errors > 0 ? 'bad' : undefined} />
          </div>
        )}
        <div className="flex items-center gap-2 border-b border-line px-5 py-2.5">
          <Segmented<SourceFilter>
            value={source}
            onChange={setSource}
            options={[
              { value: '', label: 'All' },
              { value: 'mcp', label: 'MCP' },
              { value: 'api', label: 'API' },
              { value: 'studio', label: 'Studio' },
            ]}
          />
          <div className="relative w-72">
            <Search size={13} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-faint" />
            <Input className="w-full pl-8" placeholder="Filter by query or client" value={text} onChange={(e) => setText(e.target.value)} />
          </div>
          <span className="ml-auto flex items-center gap-1.5 text-2xs text-faint">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-ok" /> live
          </span>
        </div>
        <ErrorLine error={activity.error} />
        <div className="min-h-0 flex-1 overflow-auto">
          {activity.loading && !activity.data ? (
            <SkeletonRows />
          ) : queries.length === 0 ? (
            <Empty icon={<Activity size={22} />} title={text ? 'No query matches' : 'No queries yet'}>
              {text ? 'Try another filter.' : <>Connect an agent from <Link to="/connect" className="text-accent hover:underline">Connect</Link>, or run a query in the playground.</>}
            </Empty>
          ) : (
            <table className="w-full table-fixed text-[13px]">
              <thead>
                <tr>
                  <th className="th w-24">Time</th>
                  <th className="th w-40">Source</th>
                  <th className="th">Query</th>
                  <th className="th w-20">Mode</th>
                  <th className="th w-16 text-right">Hits</th>
                  <th className="th w-24 text-right">Latency</th>
                </tr>
              </thead>
              <tbody>
                {queries.map((q) => (
                  <tr key={q.id} className={cx('row', q.id === selectedId && 'row-active')} onClick={() => select(q.id === selectedId ? null : q.id)}>
                    <td className="td font-mono text-xs text-faint" title={dateTime(q.time)}>
                      {clock(q.time)}
                    </td>
                    <td className="td">
                      <SourceBadge q={q} />
                    </td>
                    <td className="td truncate font-mono text-xs">{q.query}</td>
                    <td className="td text-xs text-muted">{q.mode}</td>
                    <td className="td text-right font-mono text-xs tabular">
                      {q.error ? <Badge tone="bad">error</Badge> : (q.hits?.length ?? 0) === 0 ? <Badge tone="warn">0</Badge> : q.hits!.length}
                    </td>
                    <td className="td text-right font-mono text-xs tabular text-muted">{ms(q.duration_ms)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
      {selected && <QueryDrawer q={selected} onClose={() => select(null)} />}
    </div>
  );
}

function QueryDrawer({ q, onClose }: { q: ActivityQuery; onClose: () => void }) {
  const navigate = useNavigate();
  return (
    <Drawer
      open
      onClose={onClose}
      title={q.query}
      subtitle={`${q.id} · ${dateTime(q.time)}`}
      actions={
        <Button size="sm" icon={<Play size={12} />} onClick={() => navigate(`/playground?q=${encodeURIComponent(q.query)}`)}>
          Replay
        </Button>
      }
    >
      <KV
        rows={[
          ['Source', <SourceBadge q={q} />],
          ['Client', q.client || '—'],
          ['Tool', q.tool || '—'],
          ['Mode / top_k', `${q.mode} / ${q.top_k}`],
          ['Latency', ms(q.duration_ms)],
          ...(q.error ? ([['Error', <span className="text-bad">{q.error}</span>]] as [string, JSX.Element][]) : []),
        ]}
      />
      <div className="px-4 pb-2 pt-4 text-2xs font-medium uppercase tracking-wide text-faint">Returned to the client</div>
      {q.hits?.length ? (
        <table className="w-full table-fixed text-xs">
          <tbody>
            {q.hits.map((h, i) => (
              <tr
                key={h.chunk_id}
                className="row"
                onClick={() => navigate(h.path.startsWith('memory/') ? `/memory?tab=entries&id=${h.chunk_id}` : `/explorer/${h.document_id}`)}
              >
                <td className="td w-10 text-right font-mono text-faint">{i + 1}</td>
                <td className="td" title={h.path}>
                  <div className="flex min-w-0 items-baseline gap-2">
                    <span className="shrink-0 font-medium">{h.filename}</span>
                    <span className="shrink-0 font-mono text-2xs text-faint">#{h.position}</span>
                    <span className="min-w-0 truncate font-mono text-2xs text-faint">{shortPath(dirname(h.path), 50)}</span>
                  </div>
                </td>
                <td className="td w-20 text-right font-mono tabular text-muted">{h.score.toFixed(3)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <div className="px-4 text-xs text-faint">Nothing. An empty answer usually means the content is not indexed, or the query needs exact terms (keyword mode).</div>
      )}
    </Drawer>
  );
}
