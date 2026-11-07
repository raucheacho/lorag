import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { ArrowRight, FolderOpen, FolderPlus, TerminalSquare } from 'lucide-react';
import { FolderPicker } from '../components/FolderPicker';
import { api } from '../lib/api';
import { useResource } from '../lib/hooks';
import { ago, bytes, duration, ms, num, shortPath } from '../lib/format';
import { useOverview } from '../components/Shell';
import { useToast } from '../components/toast';
import { Badge, Button, Dot, ErrorLine, Input, KV, PageHeader, Section, Stat, Toggle } from '../components/ui';
import { SourceBadge } from './Logs';
import { StatusBadge } from './Ingestion';

export function OverviewPage() {
  const { data: o, error } = useOverview();
  const activity = useResource(() => api.activity(undefined, 8), [], 5000);
  const ingestion = useResource(() => api.ingestion(8), [], 5000);
  const navigate = useNavigate();

  return (
    <div className="flex h-full flex-col overflow-y-auto">
      <PageHeader
        title="Overview"
        description={o ? `${o.data_dir} · up ${duration(o.uptime_s)}` : 'Loading…'}
        actions={
          <Button variant="primary" icon={<TerminalSquare size={14} />} onClick={() => navigate('/playground')}>
            Open playground
          </Button>
        }
      />
      <ErrorLine error={error} />
      {!o && !error && (
        <div className="space-y-4 p-5">
          <div className="skeleton h-[74px] rounded-lg" />
          <div className="grid gap-4 xl:grid-cols-[1fr_340px]">
            <div className="skeleton h-72 rounded-lg" />
            <div className="skeleton h-72 rounded-lg" />
          </div>
        </div>
      )}

      {o && (
        <div className="space-y-4 p-5">
          {o.documents === 0 && <GetStarted />}

          <div className="grid grid-cols-2 divide-x divide-line rounded-lg border border-line bg-surface md:grid-cols-3 xl:grid-cols-6">
            <Stat label="Documents" value={num(o.documents)} hint={`${o.watches} watched folder${o.watches === 1 ? '' : 's'}`} />
            <Stat
              label="Chunks"
              value={num(o.chunks)}
              hint={`limit ${num(o.max_chunks)}`}
              tone={o.chunks >= o.max_chunks ? 'bad' : o.chunks > o.max_chunks * 0.9 ? 'warn' : undefined}
            />
            <Stat label="Index on disk" value={bytes(o.index.disk_bytes)} hint={`log ${bytes(o.index.wal_bytes)}`} />
            <Stat label="Store" value={bytes(o.db_bytes)} hint="documents & chunks" />
            <Stat label="Queries" value={num(o.activity.total)} hint={`p50 ${ms(o.activity.p50_ms)} · p95 ${ms(o.activity.p95_ms)}`} />
            <Stat
              label="Errors / empty"
              value={`${o.activity.errors} / ${o.activity.empty}`}
              hint="since start"
              tone={o.activity.errors > 0 ? 'bad' : undefined}
            />
          </div>

          <div className="grid gap-4 xl:grid-cols-[1fr_340px]">
            <div className="space-y-4 min-w-0">
              <Section
                title="Recent agent queries"
                actions={
                  <Link to="/logs" className="flex items-center gap-1 text-xs text-muted hover:text-fg">
                    All logs <ArrowRight size={12} />
                  </Link>
                }
              >
                {activity.data?.queries.length ? (
                  <table className="w-full table-fixed text-xs">
                    <tbody>
                      {activity.data.queries.map((q) => (
                        <tr key={q.id} className="row" onClick={() => navigate(`/logs?id=${q.id}`)}>
                          <td className="td w-20 text-faint">{ago(q.time)}</td>
                          <td className="td w-40">
                            <SourceBadge q={q} />
                          </td>
                          <td className="td truncate">{q.query}</td>
                          <td className="td w-16 text-right font-mono tabular text-muted">
                            {q.error ? <span className="text-bad">error</span> : `${q.hits?.length ?? 0} hits`}
                          </td>
                          <td className="td w-20 text-right font-mono tabular text-muted">{ms(q.duration_ms)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <div className="px-4 py-8 text-center text-xs text-faint">
                    No queries yet. Searches from agents (MCP), the API and the playground show up here.
                  </div>
                )}
              </Section>

              <Section
                title="Recent ingestion"
                actions={
                  <Link to="/ingestion" className="flex items-center gap-1 text-xs text-muted hover:text-fg">
                    Ingestion <ArrowRight size={12} />
                  </Link>
                }
              >
                {ingestion.data?.events.length ? (
                  <table className="w-full table-fixed text-xs">
                    <tbody>
                      {ingestion.data.events.map((e, i) => (
                        <tr
                          key={i}
                          className={e.document_id ? 'row' : undefined}
                          onClick={() => e.document_id && navigate(`/explorer/${e.document_id}`)}
                        >
                          <td className="td w-20 text-faint">{ago(e.time)}</td>
                          <td className="td w-24">
                            <StatusBadge status={e.status} />
                          </td>
                          <td className="td truncate font-mono" title={e.error ?? e.path}>
                            {shortPath(e.path, 80)}
                          </td>
                          <td className="td w-20 text-right font-mono tabular text-muted">{e.chunks} ch.</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <div className="px-4 py-8 text-center text-xs text-faint">Nothing indexed since the server started.</div>
                )}
              </Section>
            </div>

            <div className="space-y-4">
              <Section
                title="Search index"
                actions={o.index.rebuilding ? <Badge tone="warn">rebuilding</Badge> : o.in_sync ? <Badge tone="ok">in sync</Badge> : <Badge tone="warn">out of sync</Badge>}
              >
                <KV
                  rows={[
                    ['Engine', `VectixDB ${o.core_version}`],
                    ['Collection', o.index.collection],
                    ['Chunks indexed', `${num(o.index.chunks)} / ${num(o.chunks)}`],
                    ['Vectors', `${o.index.dim}d · ${o.index.metric}${o.index.quantized ? ' · int8' : ' · f32'}`],
                    ['Log sequence', `${num(o.index.last_seq)} (checkpoint ${num(o.index.checkpoint_seq)})`],
                  ]}
                />
              </Section>
              <MemoryCard />
              <Section title="Embeddings">
                <KV
                  rows={[
                    ['Provider', o.embedder.provider],
                    ['Model', o.embedder.model],
                    ['Endpoint', o.embedder.url],
                    ['Dimension', String(o.embedder.dim)],
                  ]}
                />
              </Section>
              <Section title="Clients">
                {Object.keys(o.activity.clients).length ? (
                  <ul className="text-xs">
                    {Object.entries(o.activity.clients)
                      .sort((a, b) => b[1] - a[1])
                      .slice(0, 6)
                      .map(([c, n]) => (
                        <li key={c} className="flex items-center gap-2 border-b border-line px-4 py-2 last:border-0">
                          <Dot tone="ok" />
                          <span className="truncate font-mono">{c}</span>
                          <span className="ml-auto font-mono tabular text-faint">{n}</span>
                        </li>
                      ))}
                  </ul>
                ) : (
                  <div className="px-4 py-4 text-xs text-faint">
                    No agent connected yet. <Link to="/connect" className="text-accent hover:underline">Connect one</Link>.
                  </div>
                )}
              </Section>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function GetStarted() {
  const toast = useToast();
  const navigate = useNavigate();
  const [path, setPath] = useState('');
  const [watch, setWatch] = useState(true);
  const [busy, setBusy] = useState(false);
  const [picking, setPicking] = useState(false);
  const embedder = useResource(api.embedder, []);

  const add = async () => {
    if (!path.trim()) return;
    setBusy(true);
    try {
      await api.indexDirectory(path.trim(), true);
      if (watch) await api.addWatch(path.trim(), true);
      toast.ok('Indexing started');
      navigate('/ingestion');
    } catch (e) {
      toast.error(e);
    } finally {
      setBusy(false);
    }
  };

  const e = embedder.data;
  return (
    <Section title="Get started">
      <ol className="divide-y divide-line text-[13px]">
        <li className="flex items-center gap-3 px-4 py-3">
          <Step n={1} done={!!e?.ok} />
          <div className="min-w-0 flex-1">
            <div className="font-medium">Embedding model</div>
            <div className="text-xs text-faint">
              {!e ? 'Checking…' : e.ok ? `Answers in ${ms(e.latency_ms)}, ${e.dim} dimensions.` : e.error}
            </div>
          </div>
          {e && !e.ok && (
            <Button size="sm" onClick={() => navigate('/settings')}>
              Settings
            </Button>
          )}
        </li>
        <li className="flex flex-wrap items-center gap-3 px-4 py-3">
          <Step n={2} done={false} />
          <div className="min-w-[200px] flex-1">
            <div className="font-medium">Index a folder</div>
            <div className="text-xs text-faint">Markdown, text, code, PDF, Office files; audio and images when enabled.</div>
          </div>
          <div className="flex w-full items-center gap-2 lg:w-auto">
            <Input
              mono
              className="w-full lg:w-80"
              placeholder="/Users/me/notes"
              value={path}
              onChange={(ev) => setPath(ev.target.value)}
              onKeyDown={(ev) => ev.key === 'Enter' && add()}
            />
            <Button icon={<FolderOpen size={14} />} onClick={() => setPicking(true)}>
              Browse…
            </Button>
            <Toggle checked={watch} onChange={setWatch} label="Watch" />
            <Button variant="primary" icon={<FolderPlus size={14} />} disabled={busy || !path.trim()} onClick={add}>
              Index
            </Button>
          </div>
          {picking && (
            <FolderPicker
              initial={path}
              onClose={() => setPicking(false)}
              onPick={(p) => {
                setPath(p);
                setPicking(false);
              }}
            />
          )}
        </li>
        <li className="flex items-center gap-3 px-4 py-3">
          <Step n={3} done={false} />
          <div className="flex-1">
            <div className="font-medium">Connect an agent</div>
            <div className="text-xs text-faint">Point any MCP client at this server.</div>
          </div>
          <Button size="sm" onClick={() => navigate('/connect')}>
            Connect
          </Button>
        </li>
      </ol>
    </Section>
  );
}

function Step({ n, done }: { n: number; done: boolean }) {
  return (
    <span
      className={
        'flex h-6 w-6 shrink-0 items-center justify-center rounded-full border font-mono text-2xs ' +
        (done ? 'border-ok/40 bg-ok/10 text-ok' : 'border-line-strong text-muted')
      }
    >
      {done ? '✓' : n}
    </span>
  );
}

function MemoryCard() {
  const navigate = useNavigate();
  const st = useResource(api.memoryStats, [], 10000);
  const s = st.data;
  return (
    <Section
      title="Memory"
      actions={
        <Link to="/memory" className="flex items-center gap-1 text-xs text-muted hover:text-fg">
          Open <ArrowRight size={12} />
        </Link>
      }
    >
      {!s ? (
        <div className="skeleton m-4 h-16" />
      ) : s.entries === 0 ? (
        <div className="px-4 py-4 text-xs text-faint">
          Nothing saved yet. Agents call memory_recall before solving a problem or searching the web, and memory_save to keep a fix or an answer.{' '}
          <button className="text-accent hover:underline" onClick={() => navigate('/connect')}>
            Connect an agent
          </button>
          .
        </div>
      ) : (
        <KV
          rows={[
            ['Entries', `${num(s.entries)}: ${s.by_kind?.fix ?? 0} fixes, ${s.by_kind?.knowledge ?? 0} answers`],
            ['Recall hit rate', s.recalls ? `${Math.round(s.hit_rate * 100)}% of ${num(s.recalls)}` : '–'],
            ['Tokens per recall', s.recalls ? s.avg_recall_tokens.toFixed(0) : '–'],
            ['Tokens saved', `~${num(s.tokens_saved_estimate)}`],
          ]}
        />
      )}
    </Section>
  );
}
