import { cloneElement, ReactElement, ReactNode, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

// Tooltip shows a short label above (or below) its child after a brief
// hover or on keyboard focus. Positioned in a portal so overflow never clips it.
export function Tooltip({ label, children, side = 'top' }: { label: ReactNode; children: ReactElement; side?: 'top' | 'bottom' }) {
  const [pos, setPos] = useState<{ x: number; y: number; below: boolean }>();
  const timer = useRef<ReturnType<typeof setTimeout>>();
  const anchor = useRef<HTMLElement | null>(null);

  const show = (delay: number) => {
    clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      const r = anchor.current?.getBoundingClientRect();
      if (!r) return;
      const below = side === 'bottom' || r.top < 40;
      setPos({ x: r.left + r.width / 2, y: below ? r.bottom + 6 : r.top - 6, below });
    }, delay);
  };
  const hide = () => {
    clearTimeout(timer.current);
    setPos(undefined);
  };
  useEffect(() => () => clearTimeout(timer.current), []);

  const child = cloneElement(children, {
    ref: (el: HTMLElement | null) => (anchor.current = el),
    onMouseEnter: () => show(350),
    onMouseLeave: hide,
    onFocus: (e: React.FocusEvent) => e.currentTarget.matches(':focus-visible') && show(0),
    onBlur: hide,
    onMouseDown: hide,
    'aria-label': typeof label === 'string' ? label : undefined,
  });

  return (
    <>
      {child}
      {pos &&
        createPortal(
          <div
            role="tooltip"
            style={{
              left: Math.min(Math.max(pos.x, 80), window.innerWidth - 80),
              top: pos.y,
              transform: `translate(-50%, ${pos.below ? '0' : '-100%'})`,
            }}
            className="fade-in pointer-events-none fixed z-[60] whitespace-nowrap rounded-md border border-line-strong bg-raised px-2 py-1 text-2xs font-medium text-fg shadow-lg"
          >
            {label}
          </div>,
          document.body,
        )}
    </>
  );
}
