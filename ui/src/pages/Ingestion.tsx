import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Eye, FolderInput, Loader2, Plus, Trash2 } from 'lucide-react';
import { api, ApiError, IngestEvent } from '../lib/api';
import { useResource } from '../lib/hooks';
import { ago, clock, dateTime, ms, num, shortPath } from '../lib/format';
import { useOverview } from '../components/Shell';
import { useToast } from '../components/toast';
import { Badge, Button, Empty, ErrorLine, Input, PageHeader, Section, Segmented, Stat, Toggle } from '../components/ui';

export function StatusBadge({ status }: { status: IngestEvent['status'] }) {
  return <Badge tone={status === 'indexed' ? 'ok' : status === 'failed' ? 'bad' : 'neutral'}>{status}</Badge>;
}

export function IngestionPage() {
  const toast = useToast();
  const navigate = useNavigate();
  const overview = useOverview();
  const ingestion = useResource(() => api.ingestion(200), [], 1500);
  const watches = useResource(api.watches, []);
  const [path, setPath] = useState('');
  const [recursive, setRecursive] = useState(true);
  const [watch, setWatch] = useState(false);
  const [busy, setBusy] = useState(false);
  const [filter, setFilter] = useState<'all' | 'failed'>('all');

  const add = async () => {
    const p = path.trim();
    if (!p) return;
    setBusy(true);
    try {
      try {
        await api.indexDirectory(p, recursive);
        if (watch) {
          await api.addWatch(p, recursive);
          watches.reload();
        }
        toast.ok(`Indexing ${p}`);
      } catch (e) {
        // Not a directory: index it as a single file.
        if (!(e instanceof ApiError) || e.status !== 400) throw e;
        const r = await api.indexFile(p);
        toast.ok(r.task_id ? `Queued ${p}` : `Indexed ${p}: ${r.chunks} chunks`);
      }
      setPath('');
      ingestion.reload();
      overview.reload();
    } catch (e) {
      toast.error(e);
    } finally {
      setBusy(false);
    }
  };

  const removeWatch = async (id: string) => {
    try {
      await api.removeWatch(id);
      watches.reload();
      toast.ok('Folder no longer watched');
    } catch (e) {
      toast.error(e);
    }
  };

  const d = ingestion.data;
  const events = (d?.events ?? []).filter((e) => filter === 'all' || e.status === 'failed');
  const failed = d?.events.filter((e) => e.status === 'failed').length ?? 0;

  return (
    <div className="flex h-full flex-col overflow-y-auto">
      <PageHeader title="Ingestion" description="Index files and folders, watch folders for changes, follow the queue" />
      <ErrorLine error={ingestion.error} />

      <div className="space-y-4 p-5">
        <Section title="Add files">
          <div className="flex flex-wrap items-center gap-3 px-4 py-3">
            <Input
              mono
              className="min-w-[280px] flex-1"
              placeholder="/absolute/path/to/a/folder/or/file.pdf"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && add()}
            />
            <Toggle checked={recursive} onChange={setRecursive} label="Subfolders" />
            <Toggle checked={watch} onChange={setWatch} label="Keep watching" />
            <Button variant="primary" icon={<Plus size={14} />} disabled={busy || !path.trim()} onClick={add}>
              Index
            </Button>
          </div>
          <div className="border-t border-line px-4 py-2 text-xs text-faint">
            The path is read by the server, so it must exist on the machine running Lorag. Unchanged files are skipped.
          </div>
        </Section>

        <div className="grid grid-cols-2 divide-x divide-line rounded-lg border border-line bg-surface md:grid-cols-4">
          {d?.queue.enabled ? (
            <>
              <Stat label="Workers" value={d.queue.workers ?? 0} />
              <Stat label="Running" value={d.queue.running ?? 0} tone={(d.queue.running ?? 0) > 0 ? 'ok' : undefined} />
              <Stat label="Queued" value={`${d.queue.queue_depth ?? 0} / ${d.queue.max_queue ?? 0}`} />
              <Stat label="Completed" value={num(d.queue.completed)} hint={failed ? `${failed} failed recently` : undefined} tone={failed ? 'bad' : undefined} />
            </>
          ) : (
            <Stat label="Queue" value="off" hint="index_workers is 0: files are indexed one by one" />
          )}
        </div>

        {!!d?.running?.length && (
          <Section title="In progress">
            <ul className="text-xs">
              {d.running.map((t) => (
                <li key={t.id} className="flex items-center gap-2 border-b border-line px-4 py-2 last:border-0">
                  <Loader2 size={13} className="animate-spin text-accent" />
                  <span className="truncate font-mono">{t.path}</span>
                  <span className="ml-auto text-faint">{ago(t.started_at)}</span>
                </li>
              ))}
            </ul>
          </Section>
        )}

        <Section title={`Watched folders${watches.data ? ` (${watches.data.watches.length})` : ''}`}>
          {watches.data?.watches.length ? (
            <ul className="text-xs">
              {watches.data.watches.map((w) => (
                <li key={w.id} className="flex items-center gap-2 border-b border-line px-4 py-2 last:border-0">
                  <Eye size={13} className="text-accent" />
                  <span className="truncate font-mono">{w.path}</span>
                  {w.recursive && <Badge>recursive</Badge>}
                  {w.patterns?.map((p) => (
                    <Badge key={p}>{p}</Badge>
                  ))}
                  <span className="ml-auto text-faint" title={dateTime(w.created_at)}>
                    since {ago(w.created_at)}
                  </span>
                  <Button size="sm" variant="ghost" icon={<Trash2 size={13} />} tip="Stop watching" onClick={() => removeWatch(w.id)} />
                </li>
              ))}
            </ul>
          ) : (
            <div className="px-4 py-4 text-xs text-faint">No folder watched. Turn on “Keep watching” when adding a folder to reindex its files as they change.</div>
          )}
        </Section>

        <Section
          title="Events"
          actions={
            <Segmented
              value={filter}
              onChange={setFilter}
              options={[
                { value: 'all', label: 'All' },
                { value: 'failed', label: `Failed${failed ? ` (${failed})` : ''}` },
              ]}
            />
          }
        >
          {events.length === 0 ? (
            <Empty icon={<FolderInput size={22} />} title={filter === 'failed' ? 'No failure' : 'No event yet'}>
              Each indexed, skipped or failed file appears here.
            </Empty>
          ) : (
            <table className="w-full table-fixed text-[13px]">
              <thead>
                <tr>
                  <th className="th w-24">Time</th>
                  <th className="th w-24">Status</th>
                  <th className="th">File</th>
                  <th className="th w-20 text-right">Chunks</th>
                  <th className="th w-24 text-right">Duration</th>
                </tr>
              </thead>
              <tbody>
                {events.map((e, i) => (
                  <tr key={i} className={e.document_id ? 'row align-top' : 'align-top'} onClick={() => e.document_id && navigate(`/explorer/${e.document_id}`)}>
                    <td className="td font-mono text-xs text-faint" title={dateTime(e.time)}>
                      {clock(e.time)}
                    </td>
                    <td className="td">
                      <StatusBadge status={e.status} />
                    </td>
                    <td className="td whitespace-normal py-2">
                      <div className="truncate font-mono text-xs" title={e.path}>
                        {shortPath(e.path, 100)}
                      </div>
                      {e.error && <div className="mt-0.5 break-words text-xs text-bad">{e.error}</div>}
                    </td>
                    <td className="td text-right font-mono text-xs tabular">{e.chunks}</td>
                    <td className="td text-right font-mono text-xs tabular text-muted">{ms(e.duration_ms)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Section>
      </div>
    </div>
  );
}
