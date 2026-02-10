interface ProgressBarProps {
  current: number;
  total: number;
  running: number;
  label?: string;
}

export function ProgressBar({ current, total, running, label }: ProgressBarProps) {
  const percentage = total > 0 ? Math.round((current / total) * 100) : 0;
  const isActive = running > 0 || current < total;

  return (
    <div className="bg-bg-secondary border border-border rounded-lg p-5">
      <div className="flex items-center justify-between mb-3">
        <div>
          <h3 className="font-medium text-text">{label || 'Indexing Progress'}</h3>
          <p className="text-text-tertiary text-sm mt-0.5">
            {running > 0 ? `${running} files processing` : 'Ready'}
          </p>
        </div>
        <div className="text-right">
          <span className="text-2xl font-semibold text-accent">{percentage}%</span>
        </div>
      </div>

      {/* Progress bar */}
      <div className="h-2 bg-bg-tertiary rounded-full overflow-hidden">
        <div 
          className={`h-full rounded-full transition-all duration-500 ease-out
            ${isActive ? 'bg-accent animate-pulse' : 'bg-accent'}`}
          style={{ width: `${percentage}%` }}
        />
      </div>

      {/* Stats */}
      <div className="grid grid-cols-4 gap-4 mt-4 pt-4 border-t border-border">
        <div className="text-center">
          <div className="text-lg font-semibold text-text">{running}</div>
          <div className="text-xs text-text-tertiary uppercase tracking-wide">Running</div>
        </div>
        <div className="text-center">
          <div className="text-lg font-semibold text-status-success">{current}</div>
          <div className="text-xs text-text-tertiary uppercase tracking-wide">Completed</div>
        </div>
        <div className="text-center">
          <div className="text-lg font-semibold text-text">{total - current}</div>
          <div className="text-xs text-text-tertiary uppercase tracking-wide">Pending</div>
        </div>
        <div className="text-center">
          <div className="text-lg font-semibold text-text">{total}</div>
          <div className="text-xs text-text-tertiary uppercase tracking-wide">Total</div>
        </div>
      </div>
    </div>
  );
}
