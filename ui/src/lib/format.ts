export function bytes(n: number | undefined): string {
  if (!n) return '0 B';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v < 10 && i > 0 ? v.toFixed(1) : Math.round(v)} ${units[i]}`;
}

export function num(n: number | undefined): string {
  return (n ?? 0).toLocaleString('en-US');
}

export function ms(n: number | undefined): string {
  if (n === undefined) return '–';
  if (n < 1) return `${n.toFixed(2)} ms`;
  if (n < 100) return `${n.toFixed(1)} ms`;
  if (n < 10_000) return `${Math.round(n)} ms`;
  return `${(n / 1000).toFixed(1)} s`;
}

export function duration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 48) return `${h}h ${m % 60}m`;
  return `${Math.floor(h / 24)}d ${h % 24}h`;
}

export function ago(iso: string | undefined): string {
  if (!iso) return '–';
  const s = Math.round((Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 5) return 'now';
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

export function clock(iso: string): string {
  return new Date(iso).toLocaleTimeString('en-GB', { hour12: false });
}

export function dateTime(iso: string | undefined): string {
  if (!iso) return '–';
  return new Date(iso).toLocaleString('en-GB', { hour12: false });
}

// shortPath keeps the end of a path, which is the part that identifies it.
export function shortPath(p: string, max = 64): string {
  if (p.length <= max) return p;
  return '…' + p.slice(p.length - max + 1);
}

export function dirname(p: string): string {
  const i = p.lastIndexOf('/');
  return i > 0 ? p.slice(0, i) : '';
}

export function mediaClock(ms: number): string {
  const s = Math.floor(ms / 1000);
  const h = Math.floor(s / 3600);
  const mm = String(Math.floor((s % 3600) / 60)).padStart(2, '0');
  const ss = String(s % 60).padStart(2, '0');
  return h ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}

interface Positioned {
  kind?: string;
  page?: number;
  line_start?: number;
  line_end?: number;
  start_ms?: number;
  end_ms?: number;
  section?: string;
}

// location renders where a chunk is in its source: "p. 12", "slide 3",
// "L40–58", "03:12–03:45", or "" when the source has no positions.
export function location(p: Positioned): string {
  if (p.end_ms) return `${mediaClock(p.start_ms ?? 0)}–${mediaClock(p.end_ms)}`;
  if (p.page && p.kind === 'slides') return `slide ${p.page}`;
  if (p.page && p.kind === 'sheet') return (p.section ?? '').replace(/^Sheet /, '') || `sheet ${p.page}`;
  if (p.page) return `p. ${p.page}`;
  if (p.line_start && p.line_end && p.line_end > p.line_start) return `L${p.line_start}–${p.line_end}`;
  if (p.line_start) return `L${p.line_start}`;
  return '';
}

// sourceLink opens the original file at the chunk's position: PDF viewers
// read #page=, media elements read #t=start,end.
export function sourceLink(fileURL: string, p: Positioned): string {
  if (p.end_ms) return `${fileURL}#t=${((p.start_ms ?? 0) / 1000).toFixed(1)},${(p.end_ms / 1000).toFixed(1)}`;
  if (p.page && p.kind === 'pdf') return `${fileURL}#page=${p.page}`;
  return fileURL;
}
