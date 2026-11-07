import { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, forwardRef, useEffect, useState } from 'react';
import { Check, Copy, X } from 'lucide-react';
import { isMac } from '../lib/hooks';
import { Tooltip } from './tooltip';
export { Select } from './select';
export type { Option } from './select';

export function cx(...xs: (string | false | null | undefined)[]) {
  return xs.filter(Boolean).join(' ');
}

type Variant = 'default' | 'primary' | 'ghost' | 'danger';

const variants: Record<Variant, string> = {
  default: 'bg-surface border border-line-strong text-fg hover:bg-raised',
  primary: 'bg-accent text-accent-fg border border-accent hover:brightness-110',
  ghost: 'text-muted hover:text-fg hover:bg-raised border border-transparent',
  danger: 'bg-surface border border-bad/40 text-bad hover:bg-bad/10',
};

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: 'sm' | 'md';
  icon?: ReactNode;
  // Tooltip, for icon-only buttons.
  tip?: ReactNode;
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'default', size = 'md', icon, tip, className, children, ...rest },
  ref,
) {
  const iconOnly = !children;
  const button = (
    <button
      ref={ref}
      className={cx(
        'inline-flex shrink-0 items-center justify-center gap-1.5 rounded-md font-medium transition-colors disabled:pointer-events-none disabled:opacity-40 whitespace-nowrap',
        size === 'sm' ? (iconOnly ? 'h-7 w-7' : 'h-7 px-2 text-xs') : iconOnly ? 'h-8 w-8' : 'h-8 px-3 text-[13px]',
        variants[variant],
        className,
      )}
      {...rest}
    >
      {icon}
      {children}
    </button>
  );
  return tip ? <Tooltip label={tip}>{button}</Tooltip> : button;
});

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement> & { mono?: boolean }>(
  function Input({ className, mono, ...rest }, ref) {
    return (
      <input
        ref={ref}
        className={cx(
          'h-8 rounded-md border border-line-strong bg-surface px-2.5 text-[13px] text-fg transition-colors placeholder:text-faint hover:border-faint/60',
          mono && 'font-mono text-xs',
          className,
        )}
        {...rest}
      />
    );
  },
);

export function Segmented<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: { value: T; label: ReactNode; title?: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div className="inline-flex h-8 rounded-md border border-line-strong bg-raised p-0.5" role="radiogroup">
      {options.map((o) => (
        <button
          key={o.value}
          role="radio"
          aria-checked={value === o.value}
          title={o.title}
          onClick={() => onChange(o.value)}
          className={cx(
            'rounded px-2.5 text-xs font-medium transition-colors',
            value === o.value ? 'bg-surface text-fg shadow-sm ring-1 ring-line-strong' : 'text-muted hover:text-fg',
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function Toggle({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label?: ReactNode }) {
  return (
    <label className="inline-flex cursor-pointer items-center gap-2 text-[13px] text-muted select-none">
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={cx('relative h-[18px] w-8 rounded-full transition-colors', checked ? 'bg-accent' : 'bg-line-strong')}
      >
        <span
          className={cx(
            'absolute left-0 top-0.5 h-3.5 w-3.5 rounded-full bg-white shadow transition-transform',
            checked ? 'translate-x-[15px]' : 'translate-x-0.5',
          )}
        />
      </button>
      {label}
    </label>
  );
}

type Tone = 'neutral' | 'accent' | 'ok' | 'warn' | 'bad' | 'vector' | 'keyword';

const tones: Record<Tone, string> = {
  neutral: 'bg-raised text-muted border-line',
  accent: 'bg-accent/10 text-accent border-accent/25',
  ok: 'bg-ok/10 text-ok border-ok/25',
  warn: 'bg-warn/10 text-warn border-warn/25',
  bad: 'bg-bad/10 text-bad border-bad/25',
  vector: 'bg-vector/10 text-vector border-vector/25',
  keyword: 'bg-keyword/10 text-keyword border-keyword/25',
};

export function Badge({ tone = 'neutral', children, className }: { tone?: Tone; children: ReactNode; className?: string }) {
  return (
    <span className={cx('inline-flex h-5 shrink-0 items-center gap-1 whitespace-nowrap rounded border px-1.5 text-2xs font-medium', tones[tone], className)}>
      {children}
    </span>
  );
}

export function Dot({ tone }: { tone: 'ok' | 'warn' | 'bad' | 'neutral' }) {
  const c = { ok: 'bg-ok', warn: 'bg-warn', bad: 'bg-bad', neutral: 'bg-faint' }[tone];
  return <span className={cx('inline-block h-1.5 w-1.5 shrink-0 rounded-full', c)} />;
}

export function Kbd({ children }: { children: ReactNode }) {
  return (
    <kbd className="inline-flex h-[18px] min-w-[18px] items-center justify-center rounded border border-line-strong bg-raised px-1 font-mono text-[10px] text-muted">
      {children}
    </kbd>
  );
}

export const modKey = isMac ? '⌘' : 'Ctrl';

export function PageHeader({ title, description, actions }: { title: string; description?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="flex min-h-[52px] items-center justify-between gap-4 border-b border-line px-5">
      <div className="min-w-0 py-2">
        <h1 className="text-sm font-semibold">{title}</h1>
        {description && <p className="truncate text-xs text-faint">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  );
}

export function Section({ title, actions, children, className }: { title?: ReactNode; actions?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={cx('rounded-lg border border-line bg-surface', className)}>
      {(title || actions) && (
        <header className="flex h-10 items-center justify-between border-b border-line px-3">
          <h2 className="text-xs font-semibold text-muted">{title}</h2>
          {actions}
        </header>
      )}
      {children}
    </section>
  );
}

export function Stat({ label, value, hint, tone }: { label: string; value: ReactNode; hint?: ReactNode; tone?: 'ok' | 'warn' | 'bad' }) {
  return (
    <div className="min-w-0 px-4 py-3">
      <div className="text-2xs font-medium uppercase tracking-wide text-faint">{label}</div>
      <div className={cx('mt-1 truncate font-mono text-lg tabular', tone && { ok: 'text-ok', warn: 'text-warn', bad: 'text-bad' }[tone])}>
        {value}
      </div>
      {hint && <div className="mt-0.5 truncate text-xs text-faint">{hint}</div>}
    </div>
  );
}

export function Empty({ icon, title, children }: { icon?: ReactNode; title: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 px-6 py-14 text-center">
      {icon && <div className="text-faint">{icon}</div>}
      <div className="text-[13px] font-medium">{title}</div>
      {children && <div className="max-w-md text-xs text-faint">{children}</div>}
    </div>
  );
}

export function ErrorLine({ error }: { error?: string }) {
  if (!error) return null;
  return <div className="border-b border-bad/30 bg-bad/5 px-5 py-2 text-xs text-bad">{error}</div>;
}

export function CopyButton({ text, label }: { text: string; label?: string }) {
  const [done, setDone] = useState(false);
  useEffect(() => {
    if (!done) return;
    const t = setTimeout(() => setDone(false), 1200);
    return () => clearTimeout(t);
  }, [done]);
  return (
    <Button
      size="sm"
      variant="ghost"
      onClick={() => navigator.clipboard.writeText(text).then(() => setDone(true))}
      icon={done ? <Check size={13} className="text-ok" /> : <Copy size={13} />}
      tip={label ? undefined : done ? 'Copied' : 'Copy'}
    >
      {label && (done ? 'Copied' : label)}
    </Button>
  );
}

export function Code({ children, copy }: { children: string; copy?: boolean }) {
  return (
    <div className="group relative">
      <pre className="overflow-x-auto rounded-md border border-line bg-raised px-3 py-2.5 font-mono text-xs leading-relaxed text-fg">{children}</pre>
      {copy && (
        <div className="absolute right-1.5 top-1.5 opacity-0 transition-opacity group-hover:opacity-100">
          <CopyButton text={children} />
        </div>
      )}
    </div>
  );
}

// Drawer is the right-hand detail panel used by every table.
export function Drawer({ open, onClose, title, subtitle, actions, children, width = 'w-[560px]', overlay = true }: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  width?: string;
  /** Over the page (the parent is `relative`), the default: the content keeps its width. false: beside it. */
  overlay?: boolean;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose();
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);
  if (!open) return null;
  return (
    <aside
      className={cx(
        'slide-in flex h-full max-w-full shrink-0 flex-col border-l border-line bg-surface',
        overlay && 'absolute inset-y-0 right-0 z-30 shadow-[-12px_0_32px_-12px_rgba(0,0,0,0.45)]',
        width,
      )}
    >
      <header className="flex min-h-[52px] items-center gap-3 border-b border-line px-4">
        <div className="min-w-0 flex-1 py-2">
          <div className="truncate text-[13px] font-semibold">{title}</div>
          {subtitle && <div className="truncate text-xs text-faint">{subtitle}</div>}
        </div>
        {actions}
        <Button variant="ghost" size="sm" onClick={onClose} tip={<>Close <span className="ml-1 text-faint">Esc</span></>} icon={<X size={14} />} />
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
    </aside>
  );
}

export function KV({ rows }: { rows: [ReactNode, ReactNode][] }) {
  return (
    <dl className="grid grid-cols-[140px_1fr] text-xs">
      {rows.map(([k, v], i) => (
        <div key={i} className="contents">
          <dt className="border-b border-line px-4 py-2 text-faint">{k}</dt>
          <dd className="min-w-0 break-all border-b border-line px-4 py-2 font-mono">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

export function Field({ label, hint, children }: { label: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <div className="grid grid-cols-1 gap-1.5 border-b border-line px-4 py-3 last:border-0 md:grid-cols-[220px_1fr] md:items-center md:gap-6">
      <div>
        <div className="text-[13px] font-medium">{label}</div>
        {hint && <div className="mt-0.5 text-xs text-faint">{hint}</div>}
      </div>
      <div className="flex min-w-0 items-center gap-2">{children}</div>
    </div>
  );
}

// SkeletonRows stands in for a table while its first page loads.
export function SkeletonRows({ rows = 8, cols = [40, 12, 10, 8] }: { rows?: number; cols?: number[] }) {
  return (
    <div className="divide-y divide-line" aria-busy>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex h-9 items-center gap-6 px-3">
          {cols.map((w, j) => (
            <div key={j} className="skeleton h-2.5" style={{ width: `${w - (i % 3) * (j === 0 ? 6 : 0)}%` }} />
          ))}
        </div>
      ))}
    </div>
  );
}

// InputShell draws the field border around a bare input, with an optional
// label inside on the left and an adornment on the right.
export function InputShell({ prefix, suffix, className, children }: { prefix?: ReactNode; suffix?: ReactNode; className?: string; children: ReactNode }) {
  return (
    <label
      className={cx(
        'inline-flex h-8 min-w-0 items-center gap-2 rounded-md border border-line-strong bg-surface px-2.5 text-[13px] transition-colors hover:border-faint/60',
        'focus-within:border-accent/70 focus-within:ring-[3px] focus-within:ring-accent/15 focus-within:hover:border-accent/70',
        className,
      )}
    >
      {prefix && <span className="shrink-0 select-none text-faint">{prefix}</span>}
      {children}
      {suffix}
    </label>
  );
}

export const bareInput =
  'min-w-0 flex-1 border-0 bg-transparent p-0 text-fg outline-none placeholder:text-faint focus:border-0 focus:ring-0';

// NumberField edits freely and commits a clamped value on blur or Enter, so
// clearing the field to type a new number does not snap back.
export function NumberField({ value, onChange, min, max, step, prefix, className }: {
  value: number;
  onChange: (v: number) => void;
  min?: number;
  max?: number;
  step?: number;
  prefix?: ReactNode;
  className?: string;
}) {
  const [text, setText] = useState(String(value));
  useEffect(() => setText(String(value)), [value]);
  const commit = () => {
    let v = Number(text);
    if (text.trim() === '' || Number.isNaN(v)) v = value;
    if (min !== undefined) v = Math.max(min, v);
    if (max !== undefined) v = Math.min(max, v);
    setText(String(v));
    if (v !== value) onChange(v);
  };
  return (
    <InputShell prefix={prefix} className={className}>
      <input
        type="number"
        inputMode="decimal"
        min={min}
        max={max}
        step={step}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => e.key === 'Enter' && commit()}
        className={cx(bareInput, 'font-mono text-xs tabular')}
      />
    </InputShell>
  );
}
