import { FileText, Image, Film, Music, FileCode, Trash2, Eye, RefreshCw } from 'lucide-react';
import type { Document } from '../types';

interface DocumentTableProps {
  documents: Document[];
  onDelete?: (id: string) => void;
  onView?: (id: string) => void;
  onReindex?: (id: string) => void;
}

const iconMap: Record<string, React.ReactNode> = {
  '.pdf': <FileText size={16} className="text-red-400" />,
  '.docx': <FileText size={16} className="text-blue-400" />,
  '.pptx': <FileText size={16} className="text-orange-400" />,
  '.md': <FileCode size={16} className="text-purple-400" />,
  '.jpg': <Image size={16} className="text-green-400" />,
  '.jpeg': <Image size={16} className="text-green-400" />,
  '.png': <Image size={16} className="text-green-400" />,
  '.mp4': <Film size={16} className="text-yellow-400" />,
  '.wav': <Music size={16} className="text-cyan-400" />,
};

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatDate(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleDateString('en-US', { 
    month: 'short', 
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
}

export function DocumentTable({ documents, onDelete, onView, onReindex }: DocumentTableProps) {
  if (documents.length === 0) {
    return (
      <div className="bg-bg-secondary border border-border rounded-lg p-8 text-center">
        <FileText size={48} className="mx-auto text-text-tertiary mb-4" />
        <h3 className="text-lg font-medium text-text">No documents</h3>
        <p className="text-text-tertiary text-sm mt-1">
          Start by indexing some files or directories
        </p>
      </div>
    );
  }

  return (
    <div className="bg-bg-secondary border border-border rounded-lg overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border bg-bg-tertiary/50">
              <th className="text-left py-3 px-4 text-xs font-medium text-text-tertiary uppercase tracking-wider">
                Name
              </th>
              <th className="text-left py-3 px-4 text-xs font-medium text-text-tertiary uppercase tracking-wider">
                Type
              </th>
              <th className="text-left py-3 px-4 text-xs font-medium text-text-tertiary uppercase tracking-wider">
                Size
              </th>
              <th className="text-left py-3 px-4 text-xs font-medium text-text-tertiary uppercase tracking-wider">
                Chunks
              </th>
              <th className="text-left py-3 px-4 text-xs font-medium text-text-tertiary uppercase tracking-wider">
                Indexed
              </th>
              <th className="text-right py-3 px-4 text-xs font-medium text-text-tertiary uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {documents.map((doc) => (
              <tr 
                key={doc.id} 
                className="hover:bg-bg-tertiary/30 transition-colors cursor-pointer"
                onClick={() => onView && onView(doc.id)}
              >
                <td className="py-3 px-4">
                  <div className="flex items-center gap-3">
                    <span className="flex-shrink-0">
                      {iconMap[doc.extension.toLowerCase()] || <FileText size={16} className="text-text-tertiary" />}
                    </span>
                    <span className="font-medium text-sm text-text truncate max-w-[200px]">
                      {doc.filename}
                    </span>
                  </div>
                </td>
                <td className="py-3 px-4 text-sm text-text-secondary">
                  {doc.extension.toUpperCase().replace('.', '')}
                </td>
                <td className="py-3 px-4 text-sm text-text-secondary font-mono">
                  {formatBytes(doc.size)}
                </td>
                <td className="py-3 px-4">
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-accent-muted text-accent">
                    {doc.chunk_count}
                  </span>
                </td>
                <td className="py-3 px-4 text-sm text-text-tertiary font-mono">
                  {doc.indexed_at ? formatDate(doc.indexed_at) : '-'}
                </td>
                <td className="py-3 px-4 text-right">
                  <div className="flex items-center justify-end gap-2">
                    {onReindex && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          onReindex(doc.id);
                        }}
                        className="text-text-tertiary hover:text-accent transition-colors"
                        title="Réindexer"
                      >
                        <RefreshCw size={16} />
                      </button>
                    )}
                    {onView && (
                      <button
                        onClick={() => onView(doc.id)}
                        className="text-text-tertiary hover:text-accent transition-colors"
                        title="Voir les détails"
                      >
                        <Eye size={16} />
                      </button>
                    )}
                    {onDelete && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          if (confirm('Supprimer ce document ?')) {
                            onDelete(doc.id);
                          }
                        }}
                        className="text-text-tertiary hover:text-status-error transition-colors"
                        title="Supprimer"
                      >
                        <Trash2 size={16} />
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
