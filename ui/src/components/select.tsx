import { ReactNode, useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Check, ChevronsUpDown } from 'lucide-react';
import { cx } from './ui';

export interface Option<T extends string | number> {
  value: T;
  label: ReactNode;
  // Text used for type-ahead when label is not a string.
  text?: string;
  hint?: ReactNode;
}

interface Props<T extends string | number> {
  value: T;
  options: Option<T>[];
  onChange: (v: T) => void;
  className?: string;
  // Content shown before the selected label, e.g. a field name.
  prefix?: ReactNode;
  'aria-label'?: string;
}

// Select is a listbox styled like the rest of the Studio: the native menu
// ignores the theme and its chevron differs per OS.
export function Select<T extends string | number>({ value, options, onChange, className, prefix, ...aria }: Props<T>) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const [pos, setPos] = useState<{ left: number; top: number; width: number; up: boolean }>();
  const button = useRef<HTMLButtonElement>(null);
  const list = useRef<HTMLDivElement>(null);
  const typed = useRef({ text: '', at: 0 });
  const id = useId();
  const current = options.find((o) => o.value === value);

  const show = () => {
    const r = button.current!.getBoundingClientRect();
    const height = Math.min(options.length * 32 + 8, 280);
    const up = r.bottom + height + 8 > window.innerHeight && r.top > height + 8;
    setPos({ left: r.left, top: up ? r.top - 4 : r.bottom + 4, width: r.width, up });
    setActive(Math.max(0, options.findIndex((o) => o.value === value)));
    setOpen(true);
  };
  const close = (focus = true) => {
    setOpen(false);
    if (focus) button.current?.focus();
  };
  const pick = (i: number) => {
    const o = options[i];
    if (o) onChange(o.value);
    close();
  };

  // Close on outside click, scroll or resize (the menu is positioned once).
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (!list.current?.contains(e.target as Node) && !button.current?.contains(e.target as Node)) close(false);
    };
    const onScroll = (e: Event) => !list.current?.contains(e.target as Node) && close(false);
    const onResize = () => close(false);
    document.addEventListener('mousedown', onDown);
    window.addEventListener('scroll', onScroll, true);
    window.addEventListener('resize', onResize);
    return () => {
      document.removeEventListener('mousedown', onDown);
      window.removeEventListener('scroll', onScroll, true);
      window.removeEventListener('resize', onResize);
    };
  }, [open]);

  useLayoutEffect(() => {
    if (open) list.current?.querySelector(`[data-i="${active}"]`)?.scrollIntoView({ block: 'nearest' });
  }, [open, active]);

  const onKey = (e: React.KeyboardEvent) => {
    const last = options.length - 1;
    switch (e.key) {
      case 'ArrowDown':
      case 'ArrowUp':
        e.preventDefault();
        if (!open) return show();
        setActive((a) => (e.key === 'ArrowDown' ? Math.min(a + 1, last) : Math.max(a - 1, 0)));
        return;
      case 'Home':
      case 'End':
        if (!open) return;
        e.preventDefault();
        setActive(e.key === 'Home' ? 0 : last);
        return;
      case 'Enter':
      case ' ':
        e.preventDefault();
        if (open) pick(active);
        else show();
        return;
      case 'Escape':
      case 'Tab':
        if (open) {
          if (e.key === 'Escape') e.preventDefault();
          close(e.key === 'Escape');
        }
        return;
    }
    // Type-ahead: jump to the first option starting with the typed text.
    if (e.key.length === 1 && !e.metaKey && !e.ctrlKey) {
      const now = Date.now();
      typed.current = { text: (now - typed.current.at < 600 ? typed.current.text : '') + e.key.toLowerCase(), at: now };
      const i = options.findIndex((o) => (o.text ?? String(o.label)).toLowerCase().startsWith(typed.current.text));
      if (i < 0) return;
      if (open) setActive(i);
      else onChange(options[i].value);
    }
  };

  return (
    <>
      <button
        ref={button}
        type="button"
        role="combobox"
        aria-expanded={open}
        aria-controls={id}
        aria-haspopup="listbox"
        {...aria}
        onClick={() => (open ? close() : show())}
        onKeyDown={onKey}
        className={cx(
          'inline-flex h-8 min-w-0 items-center gap-2 rounded-md border bg-surface pl-2.5 pr-2 text-left text-[13px] text-fg transition-colors',
          open ? 'border-accent/70 ring-[3px] ring-accent/15' : 'border-line-strong hover:border-faint/60',
          className,
        )}
      >
        {prefix && <span className="shrink-0 text-faint">{prefix}</span>}
        <span className="min-w-0 flex-1 truncate">{current?.label ?? <span className="text-faint">Select…</span>}</span>
        <ChevronsUpDown size={13} className="shrink-0 text-faint" />
      </button>
      {open &&
        pos &&
        createPortal(
          <div
            ref={list}
            id={id}
            role="listbox"
            style={{
              left: pos.left,
              minWidth: Math.max(pos.width, 160),
              ...(pos.up ? { bottom: window.innerHeight - pos.top } : { top: pos.top }),
            }}
            className={cx(
              'pop fixed z-50 max-h-[280px] overflow-y-auto rounded-lg border border-line-strong bg-surface p-1 shadow-xl shadow-black/20',
              pos.up ? 'origin-bottom' : 'origin-top',
            )}
          >
            {options.map((o, i) => (
              <div
                key={String(o.value)}
                data-i={i}
                role="option"
                aria-selected={o.value === value}
                onMouseMove={() => setActive(i)}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => pick(i)}
                className={cx(
                  'flex h-8 cursor-pointer items-center gap-2 rounded-md pl-2 pr-2.5 text-[13px]',
                  i === active ? 'bg-raised text-fg' : 'text-muted',
                )}
              >
                <Check size={13} className={cx('shrink-0 text-accent', o.value !== value && 'invisible')} />
                <span className="truncate">{o.label}</span>
                {o.hint && <span className="ml-auto pl-4 text-2xs text-faint">{o.hint}</span>}
              </div>
            ))}
          </div>,
          document.body,
        )}
    </>
  );
}
