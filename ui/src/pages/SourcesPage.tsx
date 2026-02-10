import { useState } from 'react';
import { FolderOpen, Plus, Trash2, Loader2 } from 'lucide-react';
import { useWatches } from '../hooks/useWatches';

export function SourcesPage() {
  const { watches, loading, addWatch, removeWatch } = useWatches();
  const [newPath, setNewPath] = useState('');
  const [recursive, setRecursive] = useState(true);
  const [isAdding, setIsAdding] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  const handleAdd = async () => {
    if (!newPath.trim()) return;
    
    setIsAdding(true);
    setMessage(null);
    
    const success = await addWatch(newPath, recursive);
    
    if (success) {
      setMessage('Watch added successfully');
      setNewPath('');
    } else {
      setMessage('Failed to add watch - check if path exists');
    }
    
    setIsAdding(false);
    setTimeout(() => setMessage(null), 3000);
  };

  const handleRemove = async (id: string) => {
    await removeWatch(id);
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-text">Sources</h2>
        <p className="text-text-tertiary mt-1">
          Watched directories for auto-indexing
        </p>
      </div>

      {/* Add Watch */}
      <div className="bg-bg-secondary border border-border rounded-lg p-5">
        <h3 className="font-medium text-text mb-3">Add Watch</h3>
        <div className="flex gap-3">
          <div className="flex-1">
            <input
              type="text"
              value={newPath}
              onChange={(e) => setNewPath(e.target.value)}
              placeholder="Directory path to watch..."
              className="w-full px-4 py-2.5 bg-bg border border-border rounded-lg 
                text-text placeholder-text-tertiary focus:outline-none focus:border-accent
                transition-colors"
            />
          </div>
          <label className="flex items-center gap-2 px-4 py-2.5 bg-bg-tertiary 
            border border-border rounded-lg cursor-pointer hover:border-border-strong">
            <input
              type="checkbox"
              checked={recursive}
              onChange={(e) => setRecursive(e.target.checked)}
              className="rounded border-border bg-bg text-accent focus:ring-accent"
            />
            <span className="text-sm text-text-secondary">Recursive</span>
          </label>
          <button
            onClick={handleAdd}
            disabled={isAdding || !newPath.trim()}
            className="flex items-center gap-2 px-4 py-2.5 bg-accent text-bg 
              font-medium rounded-lg hover:bg-accent-hover transition-colors
              disabled:opacity-50"
          >
            {isAdding ? (
              <Loader2 size={16} className="animate-spin" />
            ) : (
              <Plus size={16} />
            )}
            Add Watch
          </button>
        </div>
        {message && (
          <p className={`mt-3 text-sm ${message.includes('success') ? 'text-status-success' : 'text-status-error'}`}>
            {message}
          </p>
        )}
      </div>

      {/* Watches List */}
      <div className="bg-bg-secondary border border-border rounded-lg overflow-hidden">
        {loading ? (
          <div className="p-8 text-center">
            <Loader2 size={32} className="mx-auto text-text-tertiary animate-spin mb-4" />
            <p className="text-text-secondary">Loading watches...</p>
          </div>
        ) : watches.length === 0 ? (
          <div className="p-8 text-center">
            <FolderOpen size={48} className="mx-auto text-text-tertiary mb-4" />
            <h3 className="text-lg font-medium text-text">No watches configured</h3>
            <p className="text-text-tertiary text-sm mt-1">
              Add directories to watch for automatic indexing
            </p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b border-border bg-bg-tertiary/50">
                <th className="text-left py-3 px-4 text-xs font-medium text-text-tertiary uppercase">
                  Path
                </th>
                <th className="text-left py-3 px-4 text-xs font-medium text-text-tertiary uppercase">
                  Recursive
                </th>
                <th className="text-left py-3 px-4 text-xs font-medium text-text-tertiary uppercase">
                  Added
                </th>
                <th className="text-right py-3 px-4 text-xs font-medium text-text-tertiary uppercase">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {watches.map((watch) => (
                <tr key={watch.id} className="hover:bg-bg-tertiary/30">
                  <td className="py-3 px-4 text-sm text-text font-mono">
                    {watch.path}
                  </td>
                  <td className="py-3 px-4">
                    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs
                      ${watch.recursive ? 'bg-accent-muted text-accent' : 'bg-bg-tertiary text-text-tertiary'}`}>
                      {watch.recursive ? 'Yes' : 'No'}
                    </span>
                  </td>
                  <td className="py-3 px-4 text-sm text-text-tertiary">
                    {new Date(watch.created_at).toLocaleDateString()}
                  </td>
                  <td className="py-3 px-4 text-right">
                    <button
                      onClick={() => handleRemove(watch.id)}
                      className="text-text-tertiary hover:text-status-error transition-colors"
                    >
                      <Trash2 size={16} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
