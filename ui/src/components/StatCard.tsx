import { LucideIcon } from 'lucide-react';

interface StatCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  icon: LucideIcon;
  trend?: {
    value: number;
    positive: boolean;
  };
}

export function StatCard({ title, value, subtitle, icon: Icon, trend }: StatCardProps) {
  return (
    <div className="bg-bg-secondary border border-border rounded-lg p-5 
      hover:border-border-strong transition-colors duration-200">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-text-tertiary text-sm font-medium">{title}</p>
          <h3 className="text-2xl font-semibold mt-1 text-text">{value}</h3>
          {subtitle && (
            <p className="text-text-tertiary text-xs mt-1">{subtitle}</p>
          )}
          {trend && (
            <div className={`flex items-center gap-1 mt-2 text-xs
              ${trend.positive ? 'text-status-success' : 'text-status-error'}`}>
              <span>{trend.positive ? '↑' : '↓'} {trend.value}%</span>
            </div>
          )}
        </div>
        <div className="p-2 bg-bg-tertiary rounded-md">
          <Icon size={20} className="text-accent" />
        </div>
      </div>
    </div>
  );
}
