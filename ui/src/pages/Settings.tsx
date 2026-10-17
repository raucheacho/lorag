import { ReactNode, useEffect, useState } from 'react';
import { AlertTriangle, Archive, Download, Hammer, PlugZap, RotateCw, Save } from 'lucide-react';
import { api, DownloadProgress, EmbedderStatus, Settings, WhisperStatus } from '../lib/api';
import { useResource } from '../lib/hooks';
import { bytes, ms } from '../lib/format';
import { useOverview } from '../components/Shell';
import { useToast } from '../components/toast';
import { Badge, Button, ErrorLine, Field, Input, NumberField, PageHeader, Section, Select, Toggle } from '../components/ui';

// Parameters baked into the index: changing one starts a new generation,
// rebuilt from the document store on the next start.
const indexKeys: (keyof Settings)[] = ['embedding_model', 'embedding_dim', 'vector_metric', 'vector_quantized', 'vector_m', 'vector_ef', 'bm25_k1', 'bm25_b'];

export function SettingsPage() {
  const toast = useToast();
  const overview = useOverview();
  const saved = useResource(api.settings, []);
  const models = useResource(api.models, []);
  const memStats = useResource(api.memoryStats, []);
  const memThresholds = memStats.data?.thresholds;
  const [draft, setDraft] = useState<Settings>();
  const [saving, setSaving] = useState(false);
  const [restart, setRestart] = useState(false);
  const [probe, setProbe] = useState<EmbedderStatus | 'running'>();

  useEffect(() => {
    if (saved.data && !draft) setDraft(saved.data);
  }, [saved.data, draft]);

  // Follow a Whisper model download until it ends.
  const downloading = Object.values(models.data?.transcription.model_status ?? {}).includes('downloading');
  useEffect(() => {
    if (!downloading) return;
    const t = setInterval(models.reload, 700);
    return () => clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [downloading]);

  if (!draft || !saved.data) return <PageHeader title="Settings" description={saved.error ?? 'Loading…'} />;

  const set = <K extends keyof Settings>(k: K, v: Settings[K]) => setDraft({ ...draft, [k]: v });
  const changed = (Object.keys(draft) as (keyof Settings)[]).filter((k) => draft[k] !== saved.data![k]);
  const rebuilds = changed.some((k) => indexKeys.includes(k));
  const recommendBge = draft.embedding_provider === 'ollama' && draft.embedding_model.split(':')[0] !== 'bge-m3';

  const save = async () => {
    setSaving(true);
    try {
      const r = await api.saveSettings(draft);
      setDraft(r.settings);
      saved.reload();
      setRestart(r.restart_required || restart);
      toast.ok(r.restart_required ? 'Saved. Restart Lorag to apply.' : 'Saved');
    } catch (e) {
      toast.error(e);
    } finally {
      setSaving(false);
    }
  };

  const testEmbedder = async () => {
    setProbe('running');
    try {
      setProbe(await api.embedder());
    } catch (e) {
      setProbe({ ok: false, error: e instanceof Error ? e.message : String(e) });
    }
  };

  const maintenance = (label: string, f: () => Promise<unknown>) => async () => {
    try {
      await f();
      toast.ok(label);
      overview.reload();
    } catch (e) {
      toast.error(e);
    }
  };

  const num = (k: keyof Settings, opts: { min?: number; max?: number; step?: number; width?: string; prefix?: string } = {}) => (
    <NumberField
      className={opts.width ?? 'w-28'}
      prefix={opts.prefix}
      min={opts.min}
      max={opts.max}
      step={opts.step}
      value={draft[k] as number}
      onChange={(v) => set(k, v as never)}
    />
  );
  const text = (k: keyof Settings, placeholder?: string) => (
    <Input mono className="w-full max-w-md" placeholder={placeholder} value={draft[k] as string} onChange={(e) => set(k, e.target.value as never)} />
  );

  return (
    <div className="flex h-full flex-col">
      <PageHeader title="Settings" description="Stored in the Lorag database; LORAG_EMBEDDING_* environment variables override them at start" />
      <ErrorLine error={saved.error} />
      {restart && (
        <div className="flex items-center gap-2 border-b border-warn/30 bg-warn/5 px-5 py-2 text-xs text-warn">
          <AlertTriangle size={13} /> Restart Lorag to apply the saved changes.
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-4xl space-y-4 p-5 pb-24">
          <Group title="Embeddings" note="The model that turns text into vectors, for documents and queries alike.">
            <Field label="Provider">
              <Select
                aria-label="Provider"
                className="w-56"
                value={draft.embedding_provider}
                onChange={(v) => set('embedding_provider', v)}
                options={[
                  { value: 'ollama', label: 'Ollama' },
                  { value: 'openai', label: 'OpenAI-compatible' },
                  { value: 'local', label: 'Local' },
                  { value: 'custom', label: 'Custom' },
                ]}
              />
            </Field>
            <Field
              label="Model"
              hint={
                recommendBge
                  ? 'bge-m3 is recommended: multilingual (a question in French finds an answer in English), and the memory thresholds are calibrated for it. Pull it first: ollama pull bge-m3.'
                  : undefined
              }
            >
              {text('embedding_model', 'bge-m3')}
              {recommendBge && (
                <Button size="sm" onClick={() => setDraft({ ...draft, embedding_model: 'bge-m3', embedding_dim: 1024 })}>
                  Use bge-m3
                </Button>
              )}
            </Field>
            <Field label="Endpoint">{text('embedding_url', 'http://localhost:11434')}</Field>
            <Field label="Dimension" hint="Must match what the model returns.">
              {num('embedding_dim', { min: 1, max: 65536 })}
            </Field>
            <Field label="Check" hint="Embeds a short text with the running configuration.">
              <Button size="sm" icon={<PlugZap size={13} />} onClick={testEmbedder} disabled={probe === 'running'}>
                Test embedder
              </Button>
              {probe && probe !== 'running' && (
                <span className={probe.ok ? 'text-xs text-ok' : 'text-xs text-bad'}>
                  {probe.ok ? `OK · ${probe.dim} dimensions · ${ms(probe.latency_ms)}` : probe.error}
                </span>
              )}
            </Field>
          </Group>

          <Group
            title="Search index"
            note="VectixDB parameters. Changing any of them starts a new index, rebuilt from the stored chunks after the restart; searches use the old one meanwhile."
          >
            <Field label="Distance">
              <Select
                aria-label="Distance"
                className="w-56"
                value={draft.vector_metric}
                onChange={(v) => set('vector_metric', v)}
                options={[
                  { value: 'cosine', label: 'Cosine', hint: 'default' },
                  { value: 'dot', label: 'Dot product' },
                  { value: 'l2', label: 'Euclidean (L2)' },
                ]}
              />
            </Field>
            <Field label="int8 vectors" hint="4× less disk and memory, recall about 0.93 instead of 0.99. Cosine only.">
              <Toggle checked={draft.vector_quantized} onChange={(v) => set('vector_quantized', v)} />
            </Field>
            <Field label="HNSW M" hint="Links per node. Higher: better recall, more memory.">
              {num('vector_m', { min: 2, max: 256 })}
            </Field>
            <Field label="ef construction" hint="Build-time search width.">
              {num('vector_ef', { min: 2 })}
            </Field>
            <Field label="ef search" hint="Query-time search width. Applies on restart, no rebuild.">
              {num('vector_ef_search', { min: 1 })}
            </Field>
            <Field label="BM25 k1 / b" hint="Term frequency saturation and length normalization.">
              {num('bm25_k1', { min: 0, step: 0.05, width: 'w-28', prefix: 'k1' })}
              {num('bm25_b', { min: 0, max: 1, step: 0.05, width: 'w-28', prefix: 'b' })}
            </Field>
          </Group>

          <Group title="Chunking and indexing" note="Applies to files indexed after the restart; reindex a document to re-chunk it.">
            <Field label="Chunk size" hint="In characters. Pages, slides and media segments are never merged across.">
              {num('chunk_size', { min: 32 })}
            </Field>
            <Field label="Overlap" hint="Tokens shared by consecutive chunks.">
              {num('chunk_overlap', { min: 0 })}
            </Field>
            <Field label="Workers" hint="Files indexed in parallel; 0 disables the queue.">
              {num('index_workers', { min: 0, max: 32 })}
            </Field>
            <Field label="Max chunks">{num('max_chunks', { min: 1 })}</Field>
          </Group>

          <Group title="MCP">
            <Field label="Result template" hint="How search results are framed for agents. Applies immediately.">
              <Select
                aria-label="Result template"
                className="w-56"
                value={draft.default_template}
                onChange={(v) => set('default_template', v)}
                options={[
                  { value: 'default', label: 'Default', hint: 'results only' },
                  { value: 'concise', label: 'Concise' },
                  { value: 'detailed', label: 'Detailed', hint: 'citations' },
                  { value: 'academic', label: 'Academic' },
                  { value: 'bullet', label: 'Bullet points' },
                ]}
              />
            </Field>
          </Group>

          <Group
            title="Memory"
            note="A recalled entry that is not an exact error match is kept only above these similarities. They depend on the embedding model: leave 0 for the values calibrated for it."
          >
            <Field
              label="Minimum similarity"
              hint={`0 = automatic${memThresholds ? ` (now ${memThresholds.similarity}${memThresholds.calibrated ? ', calibrated' : ''})` : ''}.`}
            >
              {num('memory_min_similarity', { min: 0, max: 0.99, step: 0.01 })}
            </Field>
            <Field
              label="With matching words"
              hint={`When the words match too. 0 = automatic${memThresholds ? ` (now ${memThresholds.similarity_text})` : ''}.`}
            >
              {num('memory_min_similarity_text', { min: 0, max: 0.99, step: 0.01 })}
            </Field>
          </Group>

          <Group title="Images and audio" note="Optional local models: images are described by a vision model through Ollama, audio is transcribed with Whisper.">
            <Field label="Image description" hint="Text in images is always read with tesseract when installed.">
              <Toggle checked={draft.ocr_enabled} onChange={(v) => set('ocr_enabled', v)} />
              {draft.ocr_enabled && text('ocr_model', 'llava:7b')}
            </Field>
            <Field label="Audio transcription" hint="Audio and video, with timestamps. Needs whisper-cli (brew install whisper-cpp) and ffmpeg.">
              <Toggle checked={draft.transcription_enabled} onChange={(v) => set('transcription_enabled', v)} />
              {draft.transcription_enabled && (
                <>
                  <Select
                    aria-label="Whisper model"
                    className="w-44"
                    value={draft.whisper_model}
                    onChange={(v) => set('whisper_model', v)}
                    options={[
                      { value: 'tiny', label: 'Whisper tiny', hint: '75 MB' },
                      { value: 'base', label: 'Whisper base', hint: '142 MB' },
                      { value: 'small', label: 'Whisper small', hint: '466 MB' },
                    ]}
                  />
                  {models.data && (
                    <WhisperModelState
                      status={models.data.transcription.model_status[draft.whisper_model]}
                      progress={models.data.transcription.progress?.[draft.whisper_model]}
                      onDownload={async () => {
                        try {
                          await api.downloadWhisper(draft.whisper_model);
                        } catch (e) {
                          toast.error(e);
                        }
                        models.reload();
                      }}
                    />
                  )}
                </>
              )}
            </Field>
          </Group>

          <Group title="Maintenance" note="Writes are durable as soon as they return; these only reorganize the index.">
            <Field label="Checkpoint" hint="Writes snapshots and trims the write-ahead log, for a faster start.">
              <Button size="sm" icon={<Archive size={13} />} onClick={maintenance('Checkpoint written', api.checkpoint)}>
                Checkpoint
              </Button>
            </Field>
            <Field label="Compact" hint="Rewrites the index without deleted chunks.">
              <Button size="sm" icon={<Hammer size={13} />} onClick={maintenance('Index compacted', api.compact)}>
                Compact
              </Button>
            </Field>
            <Field label="Rebuild" hint="Rebuilds the whole index from the stored chunks, in the background. Searches keep working.">
              <Button size="sm" variant="danger" icon={<RotateCw size={13} />} onClick={maintenance('Rebuild started', api.rebuild)}>
                Rebuild index
              </Button>
            </Field>
          </Group>
        </div>
      </div>

      {changed.length > 0 && (
        <div className="flex items-center gap-3 border-t border-line bg-surface px-5 py-2.5">
          <span className="text-xs text-muted">
            {changed.length} unsaved change{changed.length > 1 ? 's' : ''}
            {rebuilds && <span className="text-warn"> · the index will be rebuilt after the restart</span>}
          </span>
          <div className="ml-auto flex gap-2">
            <Button variant="ghost" onClick={() => setDraft(saved.data)}>
              Discard
            </Button>
            <Button variant="primary" icon={<Save size={13} />} disabled={saving} onClick={save}>
              Save
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

function Group({ title, note, children }: { title: string; note?: string; children: ReactNode }) {
  return (
    <Section title={title}>
      {note && <div className="border-b border-line px-4 py-2 text-xs text-faint">{note}</div>}
      {children}
    </Section>
  );
}

// The state of the chosen Whisper model: ready, a download button, or the
// download's progress.
function WhisperModelState({
  status,
  progress,
  onDownload,
}: {
  status: WhisperStatus | undefined;
  progress: DownloadProgress | undefined;
  onDownload: () => void;
}) {
  if (status === 'ready') return <Badge tone="ok">Downloaded</Badge>;
  if (status === 'downloading') {
    const done = progress?.downloaded ?? 0;
    const total = progress?.total ?? 0;
    const pct = total > 0 ? Math.min(100, (done / total) * 100) : undefined;
    return (
      <div className="flex min-w-0 items-center gap-2" role="status" aria-live="polite">
        <div
          className="h-1.5 w-28 overflow-hidden rounded-full bg-raised"
          role="progressbar"
          aria-label="Download progress"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={pct === undefined ? undefined : Math.round(pct)}
        >
          <div
            className={pct === undefined ? 'h-full w-1/3 animate-pulse rounded-full bg-accent' : 'h-full rounded-full bg-accent transition-[width] duration-500'}
            style={pct === undefined ? undefined : { width: `${pct}%` }}
          />
        </div>
        <span className="whitespace-nowrap font-mono text-2xs tabular text-muted">
          {pct === undefined ? `Downloading… ${bytes(done)}` : `${Math.floor(pct)}% · ${bytes(done)} / ${bytes(total)}`}
        </span>
      </div>
    );
  }
  return (
    <div className="flex min-w-0 items-center gap-2">
      <Button size="sm" icon={<Download size={13} />} onClick={onDownload}>
        {status === 'error' ? 'Retry download' : 'Download now'}
      </Button>
      {status === 'error' && progress?.error && (
        <span className="truncate text-2xs text-bad" title={progress.error}>
          {progress.error}
        </span>
      )}
    </div>
  );
}
