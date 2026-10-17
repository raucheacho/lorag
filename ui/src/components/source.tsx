import { useEffect, useRef, useState } from 'react';
import { ExternalLink, Pause, Play } from 'lucide-react';
import { api } from '../lib/api';
import { location, sourceLink } from '../lib/format';
import { Badge, Button, cx } from './ui';

export interface Positioned {
  kind?: string;
  page?: number;
  line_start?: number;
  line_end?: number;
  start_ms?: number;
  end_ms?: number;
  section?: string;
}

const kindLabel: Record<string, string> = {
  text: 'text',
  markdown: 'md',
  code: 'code',
  pdf: 'pdf',
  office: 'doc',
  sheet: 'sheet',
  slides: 'slides',
  audio: 'audio',
  video: 'video',
  image: 'image',
};

export function KindBadge({ kind }: { kind?: string }) {
  if (!kind) return null;
  const tone = kind === 'audio' || kind === 'video' ? 'vector' : kind === 'pdf' || kind === 'slides' || kind === 'sheet' || kind === 'office' ? 'keyword' : 'neutral';
  return <Badge tone={tone}>{kindLabel[kind] ?? kind}</Badge>;
}

// LocationBadge shows where a chunk is: page, slide, lines or minutes.
export function LocationBadge({ p }: { p: Positioned }) {
  const loc = location(p);
  if (!loc) return null;
  return <span className="shrink-0 rounded bg-raised px-1.5 font-mono text-2xs text-muted">{loc}</span>;
}

// OpenAt links to the original file at the chunk's position (new tab), or
// plays the audio or video range inline.
export function OpenAt({ documentId, p, className }: { documentId: string; p: Positioned; className?: string }) {
  const url = api.fileURL(documentId);
  const media = p.kind === 'audio' || p.kind === 'video';
  const [playing, setPlaying] = useState(false);
  if (media && p.end_ms) {
    return (
      <div className={className}>
        <Button size="sm" icon={playing ? <Pause size={12} /> : <Play size={12} />} onClick={() => setPlaying(!playing)}>
          {playing ? 'Hide player' : `Play ${location(p)}`}
        </Button>
        {playing && <MediaRange src={url} kind={p.kind!} start={(p.start_ms ?? 0) / 1000} end={p.end_ms / 1000} />}
      </div>
    );
  }
  const loc = location(p);
  return (
    <a href={sourceLink(url, p)} target="_blank" rel="noreferrer" className={cx('inline-flex items-center gap-1 text-xs text-accent hover:underline', className)}>
      <ExternalLink size={12} />
      Open source{loc && p.kind === 'pdf' ? ` at ${loc}` : ''}
    </a>
  );
}

// MediaRange plays [start, end] of a file and stops at end.
function MediaRange({ src, kind, start, end }: { src: string; kind: string; start: number; end: number }) {
  const ref = useRef<HTMLMediaElement | null>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const stop = () => el.currentTime >= end && el.pause();
    el.addEventListener('timeupdate', stop);
    return () => el.removeEventListener('timeupdate', stop);
  }, [end]);
  const props = {
    src: `${src}#t=${start.toFixed(1)},${end.toFixed(1)}`,
    controls: true,
    autoPlay: true,
    className: 'mt-2 max-h-72 w-full rounded-md border border-line bg-black object-contain',
  };
  return kind === 'video' ? (
    <video ref={(el) => (ref.current = el)} {...props} />
  ) : (
    <audio ref={(el) => (ref.current = el)} {...props} />
  );
}
