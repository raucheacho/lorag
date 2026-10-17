import { useEffect, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ChevronLeft, ChevronRight, Database, RefreshCw, Search, TerminalSquare, Trash2 } from 'lucide-react';
import { api, Chunk } from '../lib/api';
import { useHotkey, useResource, useStored } from '../lib/hooks';
import { ago, bytes, dateTime, dirname, num, shortPath } from '../lib/format';
import { useToast } from '../components/toast';
import { KindBadge, LocationBadge, OpenAt } from '../components/source';
import { Badge, Button, CopyButton, cx, Drawer, Empty, ErrorLine, Input, Kbd, KV, PageHeader, Select, SkeletonRows } from '../components/ui';

const PAGE = 50;

export function ExplorerPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [q, setQ] = useState('');
  const [debounced, setDebounced] = useState('');
  const [ext, setExt] = useState('');
  const [sort, setSort] = useStored('lorag.explorer.sort', 'updated');
  const [offset, setOffset] = useState(0);
  const [exts, setExts] = useState<string[]>([]);
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q), 200);
    return () => clearTimeout(t);
  }, [q]);
  useEffect(() => setOffset(0), [debounced, ext, sort]);
  useHotkey('/', () => searchRef.current?.focus());

  const docs = useResource(() => api.documents({ q: debounced, ext, sort, offset, limit: PAGE }), [debounced, ext, sort, offset], 10000);

  // Extensions seen so far feed the filter.
  useEffect(() => {
    const seen = docs.data?.documents.map((d) => d.extension).filter(Boolean) ?? [];
    if (seen.some((e) => !exts.includes(e))) setExts((xs) => Array.from(new Set([...xs, ...seen])).sort());
  }, [docs.data, exts]);

  const total = docs.data?.total ?? 0;
  const list = docs.data?.documents ?? [];

  return (
    <div className="flex h-full">
      <div className="flex min-w-0 flex-1 flex-col">
        <PageHeader title="Explorer" description="Documents in the store and the chunks the search index holds for them" />
        <div className="flex flex-wrap items-center gap-2 border-b border-line px-5 py-2.5">
          <div className="relative w-72">
            <Search size={13} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-faint" />
            <Input ref={searchRef} className="w-full pl-8 pr-8" placeholder="Filter by path or name" value={q} onChange={(e) => setQ(e.target.value)} />
            <span className="absolute right-2 top-1/2 -translate-y-1/2">
              <Kbd>/</Kbd>
            </span>
          </div>
          <Select
            aria-label="File type"
            className="w-36"
            prefix="Type"
            value={ext}
            onChange={setExt}
            options={[{ value: '', label: 'All' }, ...exts.map((e) => ({ value: e, label: e }))]}
          />
          <Select
            aria-label="Sort"
            className="w-48"
            prefix="Sort"
            value={sort}
            onChange={setSort}
            options={[
              { value: 'updated', label: 'Recently updated' },
              { value: 'name', label: 'Name' },
              { value: 'size', label: 'Largest' },
              { value: 'chunks', label: 'Most chunks' },
            ]}
          />
          <div className="ml-auto flex items-center gap-2 text-xs text-faint">
            <span className="font-mono tabular">
              {total === 0 ? '0' : `${offset + 1}–${Math.min(offset + PAGE, total)}`} of {num(total)}
            </span>
            <Button size="sm" variant="ghost" tip="Previous page" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE))} icon={<ChevronLeft size={14} />} />
            <Button size="sm" variant="ghost" tip="Next page" disabled={offset + PAGE >= total} onClick={() => setOffset(offset + PAGE)} icon={<ChevronRight size={14} />} />
          </div>
        </div>
        <ErrorLine error={docs.error} />

        <div className="min-h-0 flex-1 overflow-auto">
          {docs.loading && !docs.data ? (
            <SkeletonRows />
          ) : list.length === 0 ? (
            <Empty icon={<Database size={22} />} title={debounced || ext ? 'No document matches' : 'No documents yet'}>
              {debounced || ext ? 'Try another filter.' : 'Index a folder from Ingestion, or let an agent call index_file.'}
            </Empty>
          ) : (
            <table className="w-full table-fixed text-[13px]">
              <thead>
                <tr>
                  <th className="th">Name</th>
                  <th className="th w-32">Type</th>
                  <th className="th w-24 text-right">Size</th>
                  <th className="th w-20 text-right">Chunks</th>
                  <th className="th w-28">Indexed</th>
                </tr>
              </thead>
              <tbody>
                {list.map((d) => (
                  <tr key={d.id} className={cx('row', d.id === id && 'row-active')} onClick={() => navigate(d.id === id ? '/explorer' : `/explorer/${d.id}`)}>
                    <td className="td">
                      <div className="flex min-w-0 items-baseline gap-2">
                        <span className="max-w-[70%] shrink-0 truncate font-medium">{d.filename}</span>
                        <span className="min-w-0 truncate font-mono text-2xs text-faint" title={d.path}>
                          {shortPath(dirname(d.path), 60)}
                        </span>
                      </div>
                    </td>
                    <td className="td">
                      <span className="flex items-center gap-1.5">
                        <KindBadge kind={d.kind} />
                        <span className="font-mono text-2xs text-faint">{d.extension}</span>
                      </span>
                    </td>
                    <td className="td text-right font-mono text-xs tabular text-muted">{bytes(d.size)}</td>
                    <td className="td text-right font-mono text-xs tabular">{d.chunk_count}</td>
                    <td className="td text-xs text-muted" title={dateTime(d.indexed_at)}>
                      {d.indexed_at ? ago(d.indexed_at) : <Badge tone="warn">pending</Badge>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {id && <DocumentDrawer id={id} onClose={() => navigate('/explorer')} onChanged={docs.reload} />}
    </div>
  );
}

function DocumentDrawer({ id, onClose, onChanged }: { id: string; onClose: () => void; onChanged: () => void }) {
  const toast = useToast();
  const navigate = useNavigate();
  const doc = useResource(() => api.document(id), [id]);
  const chunks = useResource(() => api.chunks(id), [id]);
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);
  useEffect(() => setConfirm(false), [id]);

  const d = doc.data;
  const missing = chunks.data?.chunks.filter((c) => !c.indexed).length ?? 0;

  const reindex = async () => {
    setBusy(true);
    try {
      const nd = await api.reindexDocument(id);
      toast.ok(`Reindexed: ${nd.chunk_count} chunks`);
      onChanged();
      navigate(`/explorer/${nd.id}`);
    } catch (e) {
      toast.error(e);
    } finally {
      setBusy(false);
    }
  };
  const remove = async () => {
    setBusy(true);
    try {
      await api.deleteDocument(id);
      toast.ok('Document removed from the store and the index');
      onChanged();
      onClose();
    } catch (e) {
      toast.error(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Drawer
      open
      onClose={onClose}
      title={d?.filename ?? '…'}
      subtitle={d?.path}
      width="w-[600px]"
      actions={
        d && (
          <>
            <Button
              size="sm"
              variant="ghost"
              icon={<TerminalSquare size={13} />}
              tip="Search inside this file"
              onClick={() => navigate(`/playground?path=${encodeURIComponent(d.path)}`)}
            />
            <Button size="sm" variant="ghost" icon={<RefreshCw size={13} />} disabled={busy} onClick={reindex} tip="Reindex" />
            {confirm ? (
              <Button size="sm" variant="danger" disabled={busy} onClick={remove}>
                Confirm delete
              </Button>
            ) : (
              <Button size="sm" variant="ghost" icon={<Trash2 size={13} />} onClick={() => setConfirm(true)} tip="Delete" />
            )}
          </>
        )
      }
    >
      {doc.error && <ErrorLine error={doc.error} />}
      {d && (
        <KV
          rows={[
            ['ID', <span className="flex items-center justify-between gap-2">{d.id}<CopyButton text={d.id} /></span>],
            ['Size', `${bytes(d.size)} · ${d.extension || 'no extension'}${d.mime_type ? ` · ${d.mime_type}` : ''}`],
            ['Checksum', d.checksum.slice(0, 16)],
            ['Source', <a href={api.fileURL(d.id)} target="_blank" rel="noreferrer" className="text-accent hover:underline">Open original file</a>],
            ['Indexed', dateTime(d.indexed_at)],
            [
              'Chunks',
              <span className="flex items-center gap-2">
                {d.chunk_count}
                {chunks.data && (missing === 0 ? <Badge tone="ok">all in index</Badge> : <Badge tone="bad">{missing} missing from index</Badge>)}
              </span>,
            ],
            ...Object.entries(d.metadata ?? {}).map(([k, v]) => [k, v] as [string, string]),
          ]}
        />
      )}
      <div className="px-4 pb-2 pt-4 text-2xs font-medium uppercase tracking-wide text-faint">Chunks</div>
      <div className="space-y-2 px-4 pb-4">
        {chunks.data?.chunks.map((c) => <ChunkCard key={c.id} c={c} kind={d?.kind} />)}
      </div>
    </Drawer>
  );
}

function ChunkCard({ c, kind }: { c: Chunk; kind?: string }) {
  const [open, setOpen] = useState(false);
  const long = c.content.length > 420;
  return (
    <div className="rounded-md border border-line">
      <div className="flex items-center gap-2 border-b border-line px-3 py-1.5 text-2xs text-faint">
        <span className="font-mono text-muted">#{c.position}</span>
        <LocationBadge p={{ ...c, kind }} />
        {c.section && c.section !== 'screen' && (
          <span className="min-w-0 truncate" title={c.section}>
            {c.section}
          </span>
        )}
        <span className="font-mono">{c.tokens} tok</span>
        <span className="font-mono">{c.dim}d</span>
        <span className="ml-auto">{c.indexed ? <Badge tone="ok">indexed</Badge> : <Badge tone="bad">not in index</Badge>}</span>
        <CopyButton text={c.id} />
      </div>
      <pre
        className={cx('whitespace-pre-wrap break-words px-3 py-2 font-mono text-xs leading-relaxed', !open && long && 'line-clamp-6')}
        onClick={() => long && setOpen(!open)}
      >
        {c.content}
      </pre>
      {(kind === 'audio' || kind === 'video' || kind === 'pdf') && (
        <OpenAt documentId={c.document_id} p={{ ...c, kind }} className="block px-3 pb-2" />
      )}
      {long && (
        <button className="w-full border-t border-line py-1 text-2xs text-faint hover:text-fg" onClick={() => setOpen(!open)}>
          {open ? 'Collapse' : `Show all ${num(c.content.length)} characters`}
        </button>
      )}
    </div>
  );
}
