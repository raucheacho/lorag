import { ReactNode, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  Archive,
  BookmarkPlus,
  Brain,
  CheckCircle2,
  Download,
  FlaskConical,
  History,
  Pencil,
  RotateCcw,
  Search,
  ThumbsDown,
  ThumbsUp,
  Trash2,
  XCircle,
} from 'lucide-react';
import { api, MemoryDay, MemoryEntry, MemoryEvent, MemoryKind, MemoryStatus, RecallResult } from '../lib/api';
import { useResource } from '../lib/hooks';
import { ago, clock, dateTime, num } from '../lib/format';
import { useToast } from '../components/toast';
import {
  Badge,
  Button,
  CopyButton,
  cx,
  Drawer,
  Empty,
  ErrorLine,
  Input,
  KV,
  PageHeader,
  Section,
  Segmented,
  Select,
  SkeletonRows,
  Stat,
} from '../components/ui';

type Tab = 'trends' | 'entries' | 'timeline' | 'health';

export function StatusBadge({ status }: { status: MemoryStatus }) {
  const tone = status === 'valid' ? 'ok' : status === 'recheck' ? 'warn' : 'bad';
  const Icon = status === 'valid' ? CheckCircle2 : status === 'recheck' ? RotateCcw : Archive;
  return (
    <Badge tone={tone}>
      <Icon size={10} />
      {status}
    </Badge>
  );
}

export function KindBadge({ kind }: { kind: MemoryKind }) {
  return kind === 'knowledge' ? <Badge tone="vector">answer</Badge> : <Badge tone="keyword">fix</Badge>;
}

const answerLabel = (k: MemoryKind) => (k === 'knowledge' ? 'Answer' : 'Fix');
const questionLabel = (k: MemoryKind) => (k === 'knowledge' ? 'Question' : 'Problem');

function SourceLink({ source }: { source: string }) {
  if (/^https?:\/\//.test(source))
    return (
      <a href={source} target="_blank" rel="noreferrer" className="break-all text-accent hover:underline">
        {source}
      </a>
    );
  return <>{source}</>;
}

export function MemoryPage() {
  const [params, setParams] = useSearchParams();
  const tab = (params.get('tab') as Tab) || 'trends';
  const selected = params.get('id');
  const stats = useResource(api.memoryStats, [], 5000);
  const [tester, setTester] = useState(false);
  const [creating, setCreating] = useState(false);

  const set = (k: string, v: string | null) =>
    setParams((p) => {
      if (v) p.set(k, v);
      else p.delete(k);
      return p;
    });
  const open = (id: string) => set('id', id);

  const s = stats.data;
  return (
    <div className="flex h-full">
      <div className="flex min-w-0 flex-1 flex-col">
        <PageHeader
          title="Memory"
          description="What agents already know: fixes and answers, recalled in a few tokens instead of solving or searching again"
          actions={
            <>
              <Button icon={<FlaskConical size={14} />} onClick={() => setTester(true)}>
                Test a recall
              </Button>
              <Button icon={<BookmarkPlus size={14} />} onClick={() => setCreating(true)}>
                New entry
              </Button>
              <Button variant="ghost" tip="Export as JSON Lines" icon={<Download size={14} />} onClick={() => (window.location.href = api.memoryExportURL)} />
            </>
          }
        />
        <ErrorLine error={stats.error} />
        {s && (
          <div className="grid grid-cols-2 divide-x divide-line border-b border-line bg-surface md:grid-cols-5">
            <Stat
              label="Entries"
              value={num(s.entries)}
              hint={`${s.by_kind?.fix ?? 0} fixes · ${s.by_kind?.knowledge ?? 0} answers · ${s.by_status.recheck ?? 0} to recheck`}
            />
            <Stat
              label="Recall hit rate"
              value={s.recalls ? `${Math.round(s.hit_rate * 100)}%` : '–'}
              hint={`${num(s.hits)} hits · ${num(s.misses)} misses`}
            />
            <Stat label="Tokens per recall" value={s.recalls ? s.avg_recall_tokens.toFixed(0) : '–'} hint="average answer size" />
            <Stat label="Tokens saved" value={`~${num(s.tokens_saved_estimate)}`} hint="estimate, see Trends" tone={s.tokens_saved_estimate > 0 ? 'ok' : undefined} />
            <Stat
              label="Feedback"
              value={`${s.feedback.worked} / ${s.feedback.failed}`}
              hint="helped / did not"
              tone={s.feedback.failed > s.feedback.worked ? 'warn' : undefined}
            />
          </div>
        )}
        <div className="flex h-10 items-end gap-1 border-b border-line px-4">
          {(
            [
              ['trends', 'Trends'],
              ['entries', `Entries${s ? ` (${s.entries})` : ''}`],
              ['timeline', 'Timeline'],
              ['health', 'Health'],
            ] as [Tab, string][]
          ).map(([t, label]) => (
            <button
              key={t}
              onClick={() => set('tab', t === 'trends' ? null : t)}
              className={cx('-mb-px h-9 border-b-2 px-3 text-xs font-medium', tab === t ? 'border-accent text-fg' : 'border-transparent text-muted hover:text-fg')}
            >
              {label}
            </button>
          ))}
        </div>
        <div className="min-h-0 flex-1 overflow-auto">
          {tab === 'trends' && <Trends onOpen={open} />}
          {tab === 'entries' && <Entries selected={selected} onOpen={open} spaces={Object.keys(s?.by_space ?? {})} />}
          {tab === 'timeline' && <Timeline onOpen={open} />}
          {tab === 'health' && <HealthView onOpen={open} />}
        </div>
      </div>
      {selected && <EntryDrawer id={selected} onClose={() => set('id', null)} onChanged={stats.reload} />}
      {tester && !selected && <RecallTester onClose={() => setTester(false)} onOpen={open} />}
      {creating && !selected && <NewEntry onClose={() => setCreating(false)} onCreated={(id) => { setCreating(false); stats.reload(); open(id); }} />}
    </div>
  );
}

// ---- trends ---------------------------------------------------------------------

function Trends({ onOpen }: { onOpen: (id: string) => void }) {
  const stats = useResource(api.memoryStats, [], 10000);
  const s = stats.data;
  if (!s) return <SkeletonRows />;
  if (s.entries === 0 && s.recalls === 0)
    return (
      <Empty icon={<Brain size={22} />} title="The memory is empty">
        Agents look it up with memory_recall before solving a problem or searching the web, and fill it with memory_save: a fix to a problem likely to
        come back, or an answer found after searching, with its source. You can also add an entry by hand.
      </Empty>
    );
  return (
    <div className="space-y-4 p-5">
      <Section title="Recalls per day, last 30 days">
        <RecallChart days={s.daily} />
      </Section>
      <div className="grid gap-4 lg:grid-cols-2">
        <Ranked title="Most recalled" hint="problems and questions that come back" items={s.recurring ?? []} unit="recalls" onOpen={onOpen} />
        <Ranked title="Most useful" hint="helped minus did not" items={s.useful ?? []} unit="net" onOpen={onOpen} />
      </div>
      <Section title="How tokens saved is estimated">
        <p className="px-4 py-3 text-xs leading-relaxed text-muted">
          Each recall that returns an entry counts the tokens it cost to find (solving the problem or searching) (given by the agent at save as <code className="font-mono">solve_tokens</code>,
          1,500 when unknown) minus the tokens of the recall. Recalls of entries reported as wrong are not counted. It measures avoided work, not a bill: compare it
          over time, not in absolute terms.
        </p>
      </Section>
    </div>
  );
}

// RecallChart: hits and misses per day as stacked bars, with a legend, a
// tooltip per bar and a table view.
function RecallChart({ days }: { days: MemoryDay[] }) {
  const [hover, setHover] = useState<number | null>(null);
  const [table, setTable] = useState(false);
  const maxTotal = Math.max(1, ...days.map((d) => d.hits + d.misses));
  const H = 140;
  const ticks = niceTicks(maxTotal);
  const top = ticks[ticks.length - 1];

  if (table)
    return (
      <div>
        <ChartHeader table={table} setTable={setTable} />
        <div className="max-h-72 overflow-auto">
          <table className="w-full text-xs">
            <thead>
              <tr>
                <th className="th">Day</th>
                <th className="th text-right">Hits</th>
                <th className="th text-right">Misses</th>
                <th className="th text-right">Saved</th>
                <th className="th text-right">Worked / failed</th>
              </tr>
            </thead>
            <tbody>
              {[...days].reverse().map((d) => (
                <tr key={d.date}>
                  <td className="td font-mono">{d.date}</td>
                  <td className="td text-right font-mono tabular">{d.hits}</td>
                  <td className="td text-right font-mono tabular">{d.misses}</td>
                  <td className="td text-right font-mono tabular">{d.saves}</td>
                  <td className="td text-right font-mono tabular">
                    {d.worked} / {d.failed}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );

  const h = hover !== null ? days[hover] : null;
  return (
    <div>
      <ChartHeader table={table} setTable={setTable} />
      <div className="relative px-4 pb-3 pt-2">
        <div className="flex gap-2">
          <div className="relative w-6 shrink-0" style={{ height: H }}>
            {ticks.map((t) => (
              <span key={t} className="absolute right-0 -translate-y-1/2 font-mono text-[10px] text-faint" style={{ top: H - (t / top) * H }}>
                {t}
              </span>
            ))}
          </div>
          <div className="relative flex-1" style={{ height: H }} onMouseLeave={() => setHover(null)}>
            {ticks.map((t) => (
              <div key={t} className="absolute inset-x-0 border-t border-line" style={{ top: H - (t / top) * H }} />
            ))}
            <div className="absolute inset-0 flex items-end gap-[2px]">
              {days.map((d, i) => {
                const hitH = (d.hits / top) * H;
                const missH = (d.misses / top) * H;
                return (
                  <div
                    key={d.date}
                    className={cx('flex h-full flex-1 flex-col justify-end', hover === i && 'bg-raised/70')}
                    onMouseEnter={() => setHover(i)}
                  >
                    {d.misses > 0 && <div className="mb-[2px] rounded-t-[4px] bg-series-2" style={{ height: Math.max(missH - 2, 2) }} />}
                    {d.hits > 0 && <div className={cx('bg-series-1', d.misses > 0 ? '' : 'rounded-t-[4px]')} style={{ height: Math.max(hitH, 2) }} />}
                  </div>
                );
              })}
            </div>
            {h && hover !== null && (
              <div
                className="pointer-events-none absolute z-10 -translate-x-1/2 rounded-md border border-line-strong bg-surface px-2.5 py-2 text-xs shadow-lg"
                style={{ left: `${((hover + 0.5) / days.length) * 100}%`, top: 0 }}
              >
                <div className="mb-1 font-mono text-faint">{h.date}</div>
                <LegendRow color="bg-series-1" label="Hits" value={h.hits} />
                <LegendRow color="bg-series-2" label="Misses" value={h.misses} />
                <div className="mt-1 border-t border-line pt-1 text-faint">
                  {h.saves} saved · {h.worked} worked · {h.failed} failed
                </div>
              </div>
            )}
          </div>
        </div>
        <div className="ml-8 mt-1 flex justify-between font-mono text-[10px] text-faint">
          <span>{days[0]?.date.slice(5)}</span>
          <span>{days[days.length - 1]?.date.slice(5)}</span>
        </div>
      </div>
    </div>
  );
}

function ChartHeader({ table, setTable }: { table: boolean; setTable: (v: boolean) => void }) {
  return (
    <div className="flex items-center gap-4 px-4 pt-3 text-xs text-muted">
      <LegendRow color="bg-series-1" label="Hits: a fix was returned" />
      <LegendRow color="bg-series-2" label="Misses: nothing known" />
      <button className="ml-auto text-faint hover:text-fg" onClick={() => setTable(!table)}>
        {table ? 'Chart' : 'Table'}
      </button>
    </div>
  );
}

function LegendRow({ color, label, value }: { color: string; label: string; value?: number }) {
  return (
    <span className="flex items-center gap-1.5">
      <span className={cx('h-2 w-2 rounded-sm', color)} />
      <span>{label}</span>
      {value !== undefined && <span className="ml-auto pl-3 font-mono tabular text-fg">{value}</span>}
    </span>
  );
}

function niceTicks(max: number): number[] {
  const step = max <= 4 ? 1 : max <= 10 ? 2 : max <= 25 ? 5 : Math.pow(10, Math.floor(Math.log10(max))) * (max / Math.pow(10, Math.floor(Math.log10(max))) > 5 ? 2 : 1);
  const out = [0];
  while (out[out.length - 1] < max) out.push(out[out.length - 1] + step);
  return out;
}

function Ranked({ title, hint, items, unit, onOpen }: { title: string; hint: string; items: { id: string; problem: string; count: number }[]; unit: string; onOpen: (id: string) => void }) {
  const top = Math.max(1, ...items.map((i) => i.count));
  return (
    <Section title={<>{title} <span className="font-normal text-faint">· {hint}</span></>}>
      {items.length === 0 ? (
        <div className="px-4 py-6 text-center text-xs text-faint">Nothing yet.</div>
      ) : (
        <ul>
          {items.map((it) => (
            <li key={it.id}>
              <button onClick={() => onOpen(it.id)} className="flex w-full items-center gap-3 border-b border-line px-4 py-2 text-left last:border-0 hover:bg-raised">
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-xs">{it.problem}</span>
                  <span className="mt-1 block h-1 rounded-full bg-raised">
                    <span className="block h-1 rounded-full bg-series-1" style={{ width: `${(it.count / top) * 100}%` }} />
                  </span>
                </span>
                <span className="w-24 whitespace-nowrap text-right font-mono text-xs tabular">
                  {it.count} <span className="text-faint">{unit}</span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </Section>
  );
}

// ---- entries --------------------------------------------------------------------

function Entries({ selected, onOpen, spaces }: { selected: string | null; onOpen: (id: string) => void; spaces: string[] }) {
  const [kind, setKind] = useState<'' | MemoryKind>('');
  const [status, setStatus] = useState<'' | MemoryStatus>('');
  const [space, setSpace] = useState('');
  const [q, setQ] = useState('');
  const [debounced, setDebounced] = useState('');
  useEffect(() => {
    const t = setTimeout(() => setDebounced(q), 250);
    return () => clearTimeout(t);
  }, [q]);
  const list = useResource(() => api.memory({ kind, status, space, q: debounced, limit: 1000 }), [kind, status, space, debounced], 8000);

  return (
    <>
      <div className="flex flex-wrap items-center gap-2 border-b border-line px-5 py-2.5">
        <Segmented<'' | MemoryKind>
          value={kind}
          onChange={setKind}
          options={[
            { value: '', label: 'All' },
            { value: 'fix', label: 'Fixes' },
            { value: 'knowledge', label: 'Answers' },
          ]}
        />
        <Segmented<'' | MemoryStatus>
          value={status}
          onChange={setStatus}
          options={[
            { value: '', label: 'All' },
            { value: 'valid', label: 'Valid' },
            { value: 'recheck', label: 'Recheck' },
            { value: 'obsolete', label: 'Obsolete' },
          ]}
        />
        <Select
          aria-label="Space"
          prefix="Space"
          className="w-48"
          value={space}
          onChange={setSpace}
          options={[{ value: '', label: 'All' }, ...spaces.sort().map((s) => ({ value: s, label: s }))]}
        />
        <div className="relative w-80">
          <Search size={13} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-faint" />
          <Input className="w-full pl-8" placeholder="Search problems, questions, fixes and answers" value={q} onChange={(e) => setQ(e.target.value)} />
        </div>
        {list.data && <span className="ml-auto font-mono text-xs text-faint">{list.data.total} entries</span>}
      </div>
      <ErrorLine error={list.error} />
      {list.loading && !list.data ? (
        <SkeletonRows />
      ) : !list.data?.entries.length ? (
        <Empty icon={<Brain size={22} />} title={q || status || space || kind ? 'No entry matches' : 'No entries yet'} />
      ) : (
        <table className="w-full table-fixed text-[13px]">
          <thead>
            <tr>
              <th className="th">Problem or question</th>
              <th className="th w-28">Status</th>
              <th className="th w-20 text-right">Result</th>
              {!selected && <th className="th w-20 text-right">Recalls</th>}
              {!selected && <th className="th w-28">Space</th>}
              <th className="th w-24">Verified</th>
            </tr>
          </thead>
          <tbody>
            {list.data.entries.map((e) => (
              <tr key={e.id} className={cx('row align-top', e.id === selected && 'row-active')} onClick={() => onOpen(e.id)}>
                <td className="td whitespace-normal py-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <KindBadge kind={e.kind} />
                    <span className="truncate">{e.problem}</span>
                  </div>
                  <div className="mt-0.5 flex items-center gap-2 font-mono text-2xs text-faint">
                    <span className="shrink-0 whitespace-nowrap">{e.id}</span>
                    {e.error && <span className="truncate">{e.error.split('\n')[0]}</span>}
                  </div>
                </td>
                <td className="td">
                  <StatusBadge status={e.status} />
                </td>
                <td className="td text-right font-mono text-xs tabular">
                  <span className="text-ok">+{e.successes}</span>/<span className={e.failures ? 'text-bad' : 'text-faint'}>-{e.failures}</span>
                </td>
                {!selected && <td className="td text-right font-mono text-xs tabular">{e.recalls}</td>}
                {!selected && <td className="td truncate font-mono text-xs text-muted">{e.space}</td>}
                <td className="td text-xs text-muted" title={dateTime(e.verified_at)}>
                  {ago(e.verified_at)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

function EntryDrawer({ id, onClose, onChanged }: { id: string; onClose: () => void; onChanged: () => void }) {
  const toast = useToast();
  const res = useResource(() => api.memoryEntry(id), [id]);
  const [editing, setEditing] = useState(false);
  const [problem, setProblem] = useState('');
  const [solution, setSolution] = useState('');
  const [kind, setKind] = useState<MemoryKind>('fix');
  const [source, setSource] = useState('');
  const [confirmDelete, setConfirmDelete] = useState(false);
  const e = res.data?.entry;
  useEffect(() => {
    setEditing(false);
    setConfirmDelete(false);
  }, [id]);

  const act = async (f: () => Promise<unknown>, done: string) => {
    try {
      await f();
      toast.ok(done);
      res.reload();
      onChanged();
    } catch (err) {
      toast.error(err);
    }
  };

  return (
    <Drawer
      open
      onClose={onClose}
      width="w-[540px]"
      title={e?.problem ?? (res.data ? 'Deleted entry' : '…')}
      subtitle={e ? `${e.id} · ${e.space} · revision ${e.revision}` : id}
      actions={
        e && (
          <>
            {e.status !== 'valid' && <Button size="sm" variant="ghost" tip="Mark valid" icon={<CheckCircle2 size={13} />} onClick={() => act(() => api.updateMemory(id, { status: 'valid' }), 'Marked valid')} />}
            {e.status !== 'recheck' && <Button size="sm" variant="ghost" tip="Mark to recheck" icon={<RotateCcw size={13} />} onClick={() => act(() => api.updateMemory(id, { status: 'recheck' }), 'Marked to recheck')} />}
            {e.status !== 'obsolete' && <Button size="sm" variant="ghost" tip="Mark obsolete" icon={<Archive size={13} />} onClick={() => act(() => api.updateMemory(id, { status: 'obsolete' }), 'Marked obsolete')} />}
            <Button
              size="sm"
              variant="ghost"
              tip="Edit"
              icon={<Pencil size={13} />}
              onClick={() => {
                setProblem(e.problem);
                setSolution(e.solution);
                setKind(e.kind);
                setSource(e.source ?? '');
                setEditing(true);
              }}
            />
            {confirmDelete ? (
              <Button size="sm" variant="danger" onClick={() => act(() => api.deleteMemory(id), 'Deleted; the history is kept')}>
                Confirm delete
              </Button>
            ) : (
              <Button size="sm" variant="ghost" tip="Delete" icon={<Trash2 size={13} />} onClick={() => setConfirmDelete(true)} />
            )}
          </>
        )
      }
    >
      <ErrorLine error={res.error} />
      {e && (
        <>
          <div className="space-y-3 border-b border-line p-4">
            {editing ? (
              <>
                <Segmented<MemoryKind>
                  value={kind}
                  onChange={setKind}
                  options={[
                    { value: 'fix', label: 'Fix' },
                    { value: 'knowledge', label: 'Knowledge' },
                  ]}
                />
                <Label>{questionLabel(kind)}</Label>
                <Input className="w-full" value={problem} onChange={(ev) => setProblem(ev.target.value)} />
                <Label>{answerLabel(kind)}</Label>
                <textarea
                  value={solution}
                  onChange={(ev) => setSolution(ev.target.value)}
                  rows={6}
                  className="block w-full resize-y rounded-md border border-line-strong bg-canvas px-3 py-2 font-mono text-xs"
                />
                <Label>Source</Label>
                <Input className="w-full" value={source} onChange={(ev) => setSource(ev.target.value)} placeholder="https://… or a document" />
                <div className="flex justify-end gap-2">
                  <Button variant="ghost" onClick={() => setEditing(false)}>
                    Cancel
                  </Button>
                  <Button
                    variant="primary"
                    onClick={() => act(() => api.updateMemory(id, { problem, solution, kind, source }), 'Saved as a new revision').then(() => setEditing(false))}
                  >
                    Save revision
                  </Button>
                </div>
              </>
            ) : (
              <>
                <div className="flex items-center justify-between">
                  <span className="flex items-center gap-2">
                    <Label>{answerLabel(e.kind)}</Label>
                    <KindBadge kind={e.kind} />
                  </span>
                  <CopyButton text={e.solution} />
                </div>
                <pre className="whitespace-pre-wrap break-words rounded-md border border-line bg-raised px-3 py-2 font-mono text-xs leading-relaxed">{e.solution}</pre>
              </>
            )}
            {e.error && (
              <>
                <Label>Error</Label>
                <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-md border border-line bg-raised px-3 py-2 font-mono text-2xs text-muted">{e.error}</pre>
              </>
            )}
          </div>
          <KV
            rows={[
              ['Status', <span className="flex items-center gap-2"><StatusBadge status={e.status} /> confidence {Math.round(confidence(e) * 100)}%</span>],
              ['Result', `${e.successes} helped · ${e.failures} did not · ${e.recalls} recalls`],
              ...(e.source ? ([['Source', <SourceLink source={e.source} />]] as [string, JSX.Element][]) : []),
              ...(e.versions?.length ? ([['Verified with', e.versions.join(' ')]] as [string, string][]) : []),
              ...(e.platform ? ([['Platform', e.platform]] as [string, string][]) : []),
              ['Author', `${e.author}${e.origin ? ` (${e.origin})` : ''}`],
              ['Last verified', dateTime(e.verified_at)],
              ['Created', dateTime(e.created_at)],
              ['Signature', `${e.signature} · variant ${e.variant}`],
              ['Cost to find', `${num(e.solve_tokens)} tokens`],
            ]}
          />
          <div className="flex gap-2 border-b border-line px-4 py-3">
            <Button size="sm" icon={<ThumbsUp size={12} />} onClick={() => act(() => api.memoryFeedback(id, true), 'Recorded: it helped')}>
              {e.kind === 'knowledge' ? 'Right' : 'It worked'}
            </Button>
            <Button size="sm" icon={<ThumbsDown size={12} />} onClick={() => act(() => api.memoryFeedback(id, false), 'Recorded: it did not help')}>
              {e.kind === 'knowledge' ? 'Wrong' : 'It failed'}
            </Button>
          </div>
        </>
      )}
      <div className="flex items-center gap-1.5 px-4 pb-2 pt-4 text-2xs font-medium uppercase tracking-wide text-faint">
        <History size={12} /> History
      </div>
      <ol className="px-4 pb-6">
        {res.data?.history.map((ev) => <HistoryItem key={ev.id} ev={ev} />)}
      </ol>
    </Drawer>
  );
}

function confidence(e: MemoryEntry) {
  const prior = e.author === 'person' ? 3 : 1;
  return (e.successes + prior) / (e.successes + e.failures + prior + 1);
}

function Label({ children }: { children: ReactNode }) {
  return <div className="text-2xs font-medium uppercase tracking-wide text-faint">{children}</div>;
}

const eventLabel: Record<MemoryEvent['type'], string> = {
  create: 'Saved',
  variant: 'New variant',
  confirm: 'Confirmed again',
  recall: 'Recalled',
  miss: 'Recall missed',
  feedback: 'Feedback',
  edit: 'Edited',
  status: 'Status changed',
  delete: 'Deleted',
};

function eventTone(ev: MemoryEvent): 'ok' | 'warn' | 'bad' | 'neutral' | 'accent' {
  switch (ev.type) {
    case 'create':
    case 'variant':
      return 'accent';
    case 'confirm':
      return 'ok';
    case 'miss':
      return 'warn';
    case 'feedback':
      return ev.worked ? 'ok' : 'bad';
    case 'delete':
      return 'bad';
  }
  return 'neutral';
}

function HistoryItem({ ev }: { ev: MemoryEvent }) {
  const [open, setOpen] = useState(false);
  const snapshot = ev.after && (ev.type === 'edit' || ev.type === 'create' || ev.type === 'variant');
  return (
    <li className="relative border-l border-line pb-3 pl-4">
      <span className={cx('absolute -left-[4px] top-1.5 h-2 w-2 rounded-full', { ok: 'bg-ok', warn: 'bg-warn', bad: 'bg-bad', neutral: 'bg-faint', accent: 'bg-accent' }[eventTone(ev)])} />
      <div className="flex items-center gap-2 text-xs">
        <span className="font-medium">{ev.type === 'feedback' ? (ev.worked ? 'Helped' : 'Did not help') : eventLabel[ev.type]}</span>
        {ev.actor && <span className="font-mono text-2xs text-muted">{ev.actor}</span>}
        {ev.tokens ? <span className="font-mono text-2xs text-faint">{ev.tokens} tok</span> : null}
        <span className="ml-auto text-2xs text-faint" title={dateTime(ev.at)}>
          {ago(ev.at)}
        </span>
      </div>
      {ev.type === 'feedback' && ev.summary && <div className="mt-0.5 text-xs text-muted">{ev.summary}</div>}
      {ev.query && <div className="mt-0.5 truncate font-mono text-2xs text-faint">{ev.query}</div>}
      {snapshot && (
        <button className="mt-0.5 text-2xs text-faint hover:text-fg" onClick={() => setOpen(!open)}>
          {open ? 'Hide' : 'Show'} revision {ev.after!.revision}
        </button>
      )}
      {open && ev.after && <pre className="mt-1 whitespace-pre-wrap break-words rounded border border-line bg-raised px-2 py-1.5 font-mono text-2xs">{ev.after.solution}</pre>}
    </li>
  );
}

// ---- timeline -------------------------------------------------------------------

function Timeline({ onOpen }: { onOpen: (id: string) => void }) {
  const [filter, setFilter] = useState<'all' | 'learned' | 'used' | 'forgotten'>('all');
  const evs = useResource(() => api.memoryEvents(500), [], 5000);
  const groups = useMemo(() => {
    const keep = (e: MemoryEvent) =>
      filter === 'all' ||
      (filter === 'learned' && ['create', 'variant', 'confirm', 'edit'].includes(e.type)) ||
      (filter === 'used' && ['recall', 'miss', 'feedback'].includes(e.type)) ||
      (filter === 'forgotten' && (e.type === 'delete' || e.type === 'status' || (e.type === 'feedback' && !e.worked)));
    const out: [string, MemoryEvent[]][] = [];
    for (const e of evs.data?.events ?? []) {
      if (!keep(e)) continue;
      const day = e.at.slice(0, 10);
      if (out.length && out[out.length - 1][0] === day) out[out.length - 1][1].push(e);
      else out.push([day, [e]]);
    }
    return out;
  }, [evs.data, filter]);

  return (
    <>
      <div className="border-b border-line px-5 py-2.5">
        <Segmented
          value={filter}
          onChange={setFilter}
          options={[
            { value: 'all', label: 'All' },
            { value: 'learned', label: 'Learned' },
            { value: 'used', label: 'Used' },
            { value: 'forgotten', label: 'Failed or forgotten' },
          ]}
        />
      </div>
      {evs.loading && !evs.data ? (
        <SkeletonRows />
      ) : groups.length === 0 ? (
        <Empty icon={<History size={22} />} title="Nothing yet" />
      ) : (
        <div className="p-5">
          {groups.map(([day, list]) => (
            <div key={day} className="mb-4">
              <div className="mb-1.5 font-mono text-2xs font-medium text-faint">{day}</div>
              <div className="divide-y divide-line rounded-lg border border-line bg-surface">
                {list.map((ev) => (
                  <button
                    key={ev.id}
                    disabled={!ev.entry || ev.type === 'delete'}
                    onClick={() => ev.entry && onOpen(ev.entry)}
                    className="flex w-full items-center gap-3 px-3 py-2 text-left text-xs enabled:hover:bg-raised"
                  >
                    <span className="w-16 shrink-0 font-mono text-faint">{clock(ev.at)}</span>
                    <span className="w-32 shrink-0">
                      <Badge tone={eventTone(ev)}>{ev.type === 'feedback' ? (ev.worked ? 'helped' : 'did not help') : eventLabel[ev.type].toLowerCase()}</Badge>
                    </span>
                    <span className="min-w-0 flex-1 truncate">{ev.type === 'miss' ? <span className="font-mono text-muted">{ev.query}</span> : ev.summary || ev.entry}</span>
                    {ev.actor && <span className="shrink-0 font-mono text-2xs text-faint">{ev.actor}</span>}
                    {ev.tokens ? <span className="w-14 shrink-0 text-right font-mono text-2xs text-faint">{ev.tokens} tok</span> : null}
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </>
  );
}

// ---- health ---------------------------------------------------------------------

function HealthView({ onOpen }: { onOpen: (id: string) => void }) {
  const h = useResource(api.memoryHealth, []);
  if (h.loading && !h.data) return <SkeletonRows />;
  const d = h.data;
  if (!d) return <ErrorLine error={h.error} />;
  const total = d.recheck.length + d.obsolete.length + d.stale.length + d.duplicates.length;
  if (total === 0)
    return (
      <Empty icon={<CheckCircle2 size={22} />} title="Nothing needs attention">
        No entry to recheck, none obsolete or unverified for six months, no likely duplicate.
      </Empty>
    );
  return (
    <div className="space-y-4 p-5">
      <HealthList title="To recheck" hint="did not help once, or a person flagged it" entries={d.recheck} onOpen={onOpen} />
      <HealthList title="Obsolete" hint="wrong more often than right; never recalled" entries={d.obsolete} onOpen={onOpen} />
      <HealthList title="Not verified for six months" hint="recalled with a recheck mark" entries={d.stale} onOpen={onOpen} />
      {d.duplicates.length > 0 && (
        <Section title={<>Likely duplicates <span className="font-normal text-faint">· different signatures, same problem</span></>}>
          {d.duplicates.map(([a, b]) => (
            <div key={a.id + b.id} className="grid grid-cols-2 divide-x divide-line border-b border-line last:border-0">
              {[a, b].map((e) => (
                <button key={e.id} onClick={() => onOpen(e.id)} className="px-4 py-2 text-left hover:bg-raised">
                  <div className="truncate text-xs">{e.problem}</div>
                  <div className="font-mono text-2xs text-faint">
                    {e.id} · +{e.successes}/-{e.failures}
                  </div>
                </button>
              ))}
            </div>
          ))}
        </Section>
      )}
    </div>
  );
}

function HealthList({ title, hint, entries, onOpen }: { title: string; hint: string; entries: MemoryEntry[]; onOpen: (id: string) => void }) {
  if (entries.length === 0) return null;
  return (
    <Section title={<>{title} ({entries.length}) <span className="font-normal text-faint">· {hint}</span></>}>
      {entries.map((e) => (
        <button key={e.id} onClick={() => onOpen(e.id)} className="flex w-full items-center gap-3 border-b border-line px-4 py-2 text-left last:border-0 hover:bg-raised">
          <StatusBadge status={e.status} />
          <span className="min-w-0 flex-1 truncate text-xs">{e.problem}</span>
          <span className="font-mono text-2xs text-faint">
            +{e.successes}/-{e.failures} · verified {ago(e.verified_at)}
          </span>
        </button>
      ))}
    </Section>
  );
}

// ---- recall tester and new entry -----------------------------------------------------

function RecallTester({ onClose, onOpen }: { onClose: () => void; onOpen: (id: string) => void }) {
  const toast = useToast();
  const [text, setText] = useState('');
  const [project, setProject] = useState('');
  const [budget, setBudget] = useState(60);
  const [res, setRes] = useState<RecallResult>();
  const run = async () => {
    try {
      setRes(await api.recall({ query: text, project, budget_tokens: budget, k: 3 }));
    } catch (e) {
      toast.error(e);
    }
  };
  return (
    <Drawer open onClose={onClose} title="Test a recall" subtitle="Exactly what an agent receives from memory_recall">
      <div className="space-y-3 p-4">
        <textarea
          autoFocus
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={5}
          placeholder="Paste an error message, or ask a question an agent would search for…"
          className="block w-full resize-y rounded-md border border-line-strong bg-canvas px-3 py-2 font-mono text-xs"
        />
        <div className="flex items-center gap-2">
          <Input className="flex-1" placeholder="project (optional)" value={project} onChange={(e) => setProject(e.target.value)} />
          <Select
            aria-label="Token budget"
            prefix="budget"
            className="w-36"
            value={budget}
            onChange={setBudget}
            options={[30, 60, 120, 300].map((n) => ({ value: n, label: `${n} tok` }))}
          />
          <Button variant="primary" disabled={!text.trim()} onClick={run}>
            Recall
          </Button>
        </div>
      </div>
      {res && (
        <div className="border-t border-line p-4">
          <div className="mb-1.5 flex items-center justify-between text-2xs text-faint">
            <span className="font-medium uppercase tracking-wide">Answer · ~{res.tokens} tokens</span>
            <CopyButton text={res.text} />
          </div>
          <pre className="whitespace-pre-wrap break-words rounded-md border border-line bg-raised px-3 py-2 font-mono text-xs leading-relaxed">{res.text}</pre>
          {res.matches?.map((m) => (
            <button key={m.entry.id} onClick={() => onOpen(m.entry.id)} className="mt-2 flex w-full items-center gap-2 rounded-md border border-line px-3 py-2 text-left text-xs hover:bg-raised">
              {m.by === 'signature' ? <Badge tone="accent">same error</Badge> : <Badge>similar {(m.similarity ?? 0).toFixed(2)}</Badge>}
              <StatusBadge status={m.status} />
              <span className="min-w-0 flex-1 truncate">{m.entry.problem}</span>
              {m.reason && <span className="font-mono text-2xs text-warn">{m.reason}</span>}
            </button>
          ))}
          {!res.matches?.length && (
            <div className="mt-2 space-y-2 text-xs text-faint">
              <div className="flex items-center gap-2">
                <XCircle size={13} /> A miss costs the agent a few tokens; it then searches as usual, and saves what is worth keeping.
              </div>
              {res.nearest && (
                <button
                  onClick={() => onOpen(res.nearest!.entry.id)}
                  className="flex w-full items-center gap-2 rounded-md border border-dashed border-line-strong px-3 py-2 text-left hover:bg-raised"
                >
                  <Badge tone="warn">rejected {(res.nearest.similarity ?? 0).toFixed(3)}</Badge>
                  <span className="min-w-0 flex-1 truncate text-muted">{res.nearest.entry.problem}</span>
                </button>
              )}
              <div>
                Kept when similarity ≥ {res.thresholds.similarity}, or ≥ {res.thresholds.similarity_text} when the words match too
                {res.thresholds.calibrated ? ' (calibrated for this model)' : ' (set in Settings, or defaults for an uncalibrated model)'}.
              </div>
            </div>
          )}
        </div>
      )}
    </Drawer>
  );
}

function NewEntry({ onClose, onCreated }: { onClose: () => void; onCreated: (id: string) => void }) {
  const toast = useToast();
  const [kind, setKind] = useState<MemoryKind>('fix');
  const [f, setF] = useState({ problem: '', error: '', solution: '', source: '', project: '', versions: '', platform: '', tags: '' });
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value });
  const split = (s: string) => s.split(/[,\s]+/).map((x) => x.trim()).filter(Boolean);
  const save = async () => {
    try {
      const r = await api.saveMemory({
        kind,
        source: f.source,
        problem: f.problem,
        error: f.error,
        solution: f.solution,
        project: f.project,
        versions: split(f.versions),
        platform: f.platform,
        tags: split(f.tags),
      });
      toast.ok(r.action === 'created' ? 'Saved' : r.action === 'confirmed' ? 'Already known: confirmed' : 'Saved as another variant');
      onCreated(r.entry.id);
    } catch (e) {
      toast.error(e);
    }
  };
  return (
    <Drawer open onClose={onClose} title="New entry" subtitle="Saved as written by you: it starts with a higher confidence than an agent's">
      <div className="space-y-3 p-4">
        <Segmented<MemoryKind>
          value={kind}
          onChange={setKind}
          options={[
            { value: 'fix', label: 'Fix', title: 'A problem likely to come back, and what solved it' },
            { value: 'knowledge', label: 'Knowledge', title: 'A question and its verified answer: a command, a value, an API detail, a decision' },
          ]}
        />
        <Label>{kind === 'fix' ? 'Problem, in one line' : 'Question, in one line'}</Label>
        <Input
          className="w-full"
          value={f.problem}
          onChange={set('problem')}
          placeholder={kind === 'fix' ? 'Dev server port already in use' : 'Rate limit of the GitHub REST API'}
        />
        {kind === 'fix' && (
          <>
            <Label>Error message (optional, matched by signature)</Label>
            <textarea value={f.error} onChange={set('error')} rows={3} className="block w-full resize-y rounded-md border border-line-strong bg-canvas px-3 py-2 font-mono text-xs" />
          </>
        )}
        <Label>{kind === 'fix' ? 'Fix' : 'Answer'}</Label>
        <textarea value={f.solution} onChange={set('solution')} rows={5} className="block w-full resize-y rounded-md border border-line-strong bg-canvas px-3 py-2 font-mono text-xs" />
        <Label>Source {kind === 'knowledge' ? '(recommended)' : '(optional)'}</Label>
        <Input className="w-full" value={f.source} onChange={set('source')} placeholder="https://… or a document" />
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label>Project (empty: global)</Label>
            <Input className="w-full" value={f.project} onChange={set('project')} />
          </div>
          <div className="space-y-1.5">
            <Label>Platform</Label>
            <Input className="w-full" value={f.platform} onChange={set('platform')} placeholder="macos-arm64" />
          </div>
          <div className="space-y-1.5">
            <Label>Verified with</Label>
            <Input mono className="w-full" value={f.versions} onChange={set('versions')} placeholder="node@22, expo@52" />
          </div>
          <div className="space-y-1.5">
            <Label>Tags</Label>
            <Input className="w-full" value={f.tags} onChange={set('tags')} />
          </div>
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={!f.solution.trim() || !(f.problem.trim() || f.error.trim())} onClick={save}>
            Save
          </Button>
        </div>
      </div>
    </Drawer>
  );
}
