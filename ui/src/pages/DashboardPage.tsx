import { 
  Database, 
  FileText, 
  Layers, 
  Activity,
  Cpu,
  Clock
} from 'lucide-react';
import { StatCard } from '../components/StatCard';
import { ProgressBar } from '../components/ProgressBar';
import { useStats, useQueueStats } from '../hooks/useApi';

export function DashboardPage() {
  const { stats } = useStats(3000);
  const { stats: queueStats } = useQueueStats(500);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-2xl font-semibold text-text">Dashboard</h2>
        <p className="text-text-tertiary mt-1">
          Overview of your local RAG system
        </p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Documents"
          value={stats?.documents || 0}
          subtitle="Total indexed files"
          icon={FileText}
        />
        <StatCard
          title="Chunks"
          value={stats?.chunks || 0}
          subtitle="Vector segments"
          icon={Layers}
        />
        <StatCard
          title="Vector DB"
          value={stats?.vector?.elements || 0}
          subtitle={`${stats?.vector?.dimension || 768} dimensions`}
          icon={Database}
        />
        <StatCard
          title="Workers"
          value={queueStats?.workers || 0}
          subtitle="Parallel processors"
          icon={Cpu}
        />
      </div>

      {/* Progress Section */}
      {queueStats && (
        <ProgressBar
          current={queueStats.completed}
          total={queueStats.completed + queueStats.running + queueStats.queue_depth}
          running={queueStats.running}
          label="Indexing Progress"
        />
      )}

      {/* Quick Stats */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Recent Activity */}
        <div className="bg-bg-secondary border border-border rounded-lg p-5">
          <div className="flex items-center gap-2 mb-4">
            <Activity size={18} className="text-accent" />
            <h3 className="font-medium text-text">System Status</h3>
          </div>
          <div className="space-y-3">
            <div className="flex justify-between items-center py-2 border-b border-border">
              <span className="text-text-secondary">Queue Status</span>
              <span className={`text-sm font-medium ${queueStats?.enabled ? 'text-status-success' : 'text-text-tertiary'}`}>
                {queueStats?.enabled ? 'Active' : 'Disabled'}
              </span>
            </div>
            <div className="flex justify-between items-center py-2 border-b border-border">
              <span className="text-text-secondary">Max Chunks</span>
              <span className="text-sm font-mono text-text">{stats?.maxChunks || 5000}</span>
            </div>
            <div className="flex justify-between items-center py-2 border-b border-border">
              <span className="text-text-secondary">Active Watches</span>
              <span className="text-sm font-mono text-text">{stats?.watches || 0}</span>
            </div>
            <div className="flex justify-between items-center py-2">
              <span className="text-text-secondary">Vector Layers</span>
              <span className="text-sm font-mono text-text">{stats?.vector?.max_layer || 0}</span>
            </div>
          </div>
        </div>

        {/* Performance */}
        <div className="bg-bg-secondary border border-border rounded-lg p-5">
          <div className="flex items-center gap-2 mb-4">
            <Clock size={18} className="text-accent" />
            <h3 className="font-medium text-text">Performance</h3>
          </div>
          <div className="space-y-4">
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-text-secondary">Queue Depth</span>
                <span className="text-text font-mono">{queueStats?.queue_depth || 0}</span>
              </div>
              <div className="h-1.5 bg-bg-tertiary rounded-full overflow-hidden">
                <div 
                  className="h-full bg-accent rounded-full transition-all"
                  style={{ width: `${Math.min(((queueStats?.queue_depth || 0) / (queueStats?.max_queue || 100)) * 100, 100)}%` }}
                />
              </div>
            </div>
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-text-secondary">Chunks Usage</span>
                <span className="text-text font-mono">
                  {stats?.chunks || 0} / {stats?.maxChunks || 5000}
                </span>
              </div>
              <div className="h-1.5 bg-bg-tertiary rounded-full overflow-hidden">
                <div 
                  className="h-full bg-accent rounded-full transition-all"
                  style={{ width: `${Math.min(((stats?.chunks || 0) / (stats?.maxChunks || 5000)) * 100, 100)}%` }}
                />
              </div>
            </div>
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-text-secondary">Vector Capacity</span>
                <span className="text-text font-mono">
                  {stats?.vector?.elements || 0} vectors
                </span>
              </div>
              <div className="h-1.5 bg-bg-tertiary rounded-full overflow-hidden">
                <div 
                  className="h-full bg-status-success rounded-full transition-all"
                  style={{ width: `${Math.min(((stats?.vector?.elements || 0) / (stats?.maxChunks || 5000)) * 100, 100)}%` }}
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
