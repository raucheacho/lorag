import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { 
  FileText, ArrowLeft, Layers, Calendar, 
  HardDrive, Hash, ChevronDown, ChevronUp 
} from 'lucide-react';
import { showToast } from '../components/Toaster';

interface Chunk {
  id: string;
  content: string;
  position: number;
  start_byte: number;
  end_byte: number;
  tokens: number;
}

interface Document {
  id: string;
  filename: string;
  path: string;
  extension: string;
  size: number;
  chunk_count: number;
  indexed_at?: string;
  created_at: string;
}

export function DocumentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [document, setDocument] = useState<Document | null>(null);
  const [chunks, setChunks] = useState<Chunk[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedChunks, setExpandedChunks] = useState<Set<number>>(new Set([0]));

  useEffect(() => {
    if (id) {
      fetchDocument(id);
      fetchChunks(id);
    }
  }, [id]);

  const fetchDocument = async (docId: string) => {
    try {
      const res = await fetch(`/api/v1/documents/${docId}`);
      if (res.ok) {
        const data = await res.json();
        setDocument(data);
      } else {
        showToast('error', 'Document non trouvé');
        navigate('/documents');
      }
    } catch (err) {
      showToast('error', 'Erreur de chargement');
      navigate('/documents');
    }
  };

  const fetchChunks = async (docId: string) => {
    try {
      const res = await fetch(`/api/v1/documents/${docId}/chunks`);
      if (res.ok) {
        const data = await res.json();
        setChunks(data.chunks || []);
      }
    } catch (err) {
      showToast('error', 'Erreur de chargement des chunks');
    } finally {
      setLoading(false);
    }
  };

  const toggleChunk = (index: number) => {
    const newExpanded = new Set(expandedChunks);
    if (newExpanded.has(index)) {
      newExpanded.delete(index);
    } else {
      newExpanded.add(index);
    }
    setExpandedChunks(newExpanded);
  };

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return 'Non indexé';
    return new Date(dateStr).toLocaleString('fr-FR');
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-text-secondary">Chargement...</div>
      </div>
    );
  }

  if (!document) {
    return (
      <div className="text-center py-12">
        <p className="text-text-secondary">Document non trouvé</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <button
          onClick={() => navigate('/documents')}
          className="p-2 hover:bg-bg-secondary rounded-lg text-text-secondary hover:text-text"
        >
          <ArrowLeft size={20} />
        </button>
        <div>
          <h2 className="text-2xl font-semibold text-text">{document.filename}</h2>
          <p className="text-text-tertiary text-sm">{document.path}</p>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="bg-bg-secondary border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-tertiary mb-1">
            <HardDrive size={16} />
            <span className="text-xs uppercase">Taille</span>
          </div>
          <div className="text-lg font-medium text-text">{formatSize(document.size)}</div>
        </div>
        
        <div className="bg-bg-secondary border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-tertiary mb-1">
            <Layers size={16} />
            <span className="text-xs uppercase">Chunks</span>
          </div>
          <div className="text-lg font-medium text-text">{document.chunk_count}</div>
        </div>
        
        <div className="bg-bg-secondary border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-tertiary mb-1">
            <FileText size={16} />
            <span className="text-xs uppercase">Format</span>
          </div>
          <div className="text-lg font-medium text-text uppercase">{document.extension}</div>
        </div>
        
        <div className="bg-bg-secondary border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-tertiary mb-1">
            <Calendar size={16} />
            <span className="text-xs uppercase">Indexé le</span>
          </div>
          <div className="text-lg font-medium text-text">{formatDate(document.indexed_at)}</div>
        </div>
      </div>

      {/* Chunks */}
      <div>
        <h3 className="text-lg font-medium text-text mb-4 flex items-center gap-2">
          <Hash size={18} className="text-accent" />
          Contenu indexé ({chunks.length} chunks)
        </h3>
        
        <div className="space-y-3">
          {chunks.map((chunk, index) => (
            <div 
              key={chunk.id}
              className="bg-bg-secondary border border-border rounded-lg overflow-hidden"
            >
              <button
                onClick={() => toggleChunk(index)}
                className="w-full flex items-center justify-between p-4 hover:bg-bg-tertiary/50 transition-colors"
              >
                <div className="flex items-center gap-3">
                  <span className="text-xs font-mono text-text-tertiary bg-bg-tertiary px-2 py-1 rounded">
                    #{index + 1}
                  </span>
                  <span className="text-sm text-text-secondary">
                    Position {chunk.position} • Octets {chunk.start_byte}-{chunk.end_byte}
                  </span>
                  <span className="text-xs text-text-tertiary">
                    ({chunk.tokens} tokens)
                  </span>
                </div>
                {expandedChunks.has(index) ? (
                  <ChevronUp size={18} className="text-text-tertiary" />
                ) : (
                  <ChevronDown size={18} className="text-text-tertiary" />
                )}
              </button>
              
              {expandedChunks.has(index) && (
                <div className="px-4 pb-4">
                  <div className="bg-bg border border-border rounded-lg p-4">
                    <pre className="text-sm text-text-secondary whitespace-pre-wrap font-mono leading-relaxed">
                      {chunk.content}
                    </pre>
                  </div>
                </div>
              )}
            </div>
          ))}
          
          {chunks.length === 0 && (
            <div className="text-center py-8 text-text-tertiary">
              Aucun chunk trouvé pour ce document
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
