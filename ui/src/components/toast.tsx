import { createContext, ReactNode, useCallback, useContext, useState } from 'react';
import { CheckCircle2, XCircle } from 'lucide-react';

interface Toast {
  id: number;
  kind: 'ok' | 'error';
  text: string;
}

const Ctx = createContext<(kind: Toast['kind'], text: string) => void>(() => {});

export function useToast() {
  const push = useContext(Ctx);
  return {
    ok: (text: string) => push('ok', text),
    error: (e: unknown) => push('error', e instanceof Error ? e.message : String(e)),
  };
}

let next = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Toast[]>([]);
  const push = useCallback((kind: Toast['kind'], text: string) => {
    const id = ++next;
    setItems((xs) => [...xs.slice(-3), { id, kind, text }]);
    setTimeout(() => setItems((xs) => xs.filter((x) => x.id !== id)), kind === 'error' ? 6000 : 3000);
  }, []);
  return (
    <Ctx.Provider value={push}>
      {children}
      <div className="pointer-events-none fixed bottom-4 right-4 z-50 flex w-80 flex-col gap-2" aria-live="polite">
        {items.map((t) => (
          <div
            key={t.id}
            className="pointer-events-auto flex items-start gap-2 rounded-md border border-line-strong bg-surface px-3 py-2.5 text-xs shadow-lg"
          >
            {t.kind === 'ok' ? <CheckCircle2 size={14} className="mt-px shrink-0 text-ok" /> : <XCircle size={14} className="mt-px shrink-0 text-bad" />}
            <span className="break-words">{t.text}</span>
          </div>
        ))}
      </div>
    </Ctx.Provider>
  );
}
