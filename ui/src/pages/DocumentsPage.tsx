import { useState, useEffect } from 'react';
import { RefreshCw, FolderOpen, FileText, Film, Music, Image as ImageIcon, Download } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { DocumentTable } from '../components/DocumentTable';
import { useDocuments, indexFile, indexDirectory, deleteDocument, reindexDocument, reindexMedia, getModelStatus } from '../hooks/useApi';

interface ModelStatus {
  ocr: { enabled: boolean; model: string };
  transcription: { 
    enabled: boolean; 
    model: string; 
    model_status: Record<string, string>;
  };
}

export function DocumentsPage() {
  const navigate = useNavigate();
  const { documents, loading, refetch } = useDocuments(100);
  const [path, setPath] = useState('');
  const [isIndexing, setIsIndexing] = useState(false);
  const [isReindexingMedia, setIsReindexingMedia] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [modelStatus, setModelStatus] = useState<ModelStatus | null>(null);

  // Load model status
  useEffect(() => {
    loadModelStatus();
  }, []);

  const loadModelStatus = async () => {
    try {
      const status = await getModelStatus();
      setModelStatus(status);
    } catch {
      // Ignore error
    }
  };

  // Refresh model status periodically
  useEffect(() => {
    const interval = setInterval(loadModelStatus, 5000);
    return () => clearInterval(interval);
  }, []);

  const handleViewDocument = (id: string) => {
    navigate(`/documents/${id}`);
  };

  const handleDeleteDocument = async (id: string) => {
    try {
      await deleteDocument(id);
      setMessage('Document supprimé');
      refetch();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Failed to delete document');
    }
    setTimeout(() => setMessage(null), 3000);
  };

  const handleReindexDocument = async (id: string) => {
    try {
      await reindexDocument(id);
      setMessage('Document en cours de réindexation');
      setTimeout(refetch, 2000);
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Failed to reindex document');
    }
    setTimeout(() => setMessage(null), 3000);
  };

  const handleReindexMedia = async (types: string[]) => {
    setIsReindexingMedia(true);
    try {
      const result = await reindexMedia(types);
      setMessage(`${result.count} fichiers en cours de réindexation (${types.join(', ')})`);
      setTimeout(refetch, 3000);
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Failed to reindex media');
    } finally {
      setIsReindexingMedia(false);
      setTimeout(() => setMessage(null), 5000);
    }
  };

  const handleIndexFile = async () => {
    if (!path.trim()) return;
    setIsIndexing(true);
    try {
      await indexFile(path);
      setMessage('File indexed successfully');
      setPath('');
      refetch();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Failed to index file');
    } finally {
      setIsIndexing(false);
      setTimeout(() => setMessage(null), 3000);
    }
  };

  const handleIndexDirectory = async () => {
    if (!path.trim()) return;
    setIsIndexing(true);
    try {
      await indexDirectory(path);
      setMessage('Directory indexing started');
      setPath('');
      setTimeout(refetch, 2000);
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Failed to index directory');
    } finally {
      setIsIndexing(false);
      setTimeout(() => setMessage(null), 3000);
    }
  };

  // Count media files
  const mediaCounts = documents.reduce((acc, doc) => {
    const ext = doc.extension.toLowerCase();
    if (['.jpg', '.jpeg', '.png', '.gif', '.webp'].includes(ext)) acc.images++;
    else if (['.wav', '.mp3', '.m4a', '.ogg', '.flac'].includes(ext)) acc.audio++;
    else if (['.mp4', '.avi', '.mkv', '.mov', '.webm'].includes(ext)) acc.video++;
    return acc;
  }, { images: 0, audio: 0, video: 0 });

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-text">Documents</h2>
          <p className="text-text-tertiary mt-1">
            {documents.length} files indexed
          </p>
        </div>
        <button
          onClick={refetch}
          disabled={loading}
          className="flex items-center gap-2 px-4 py-2 bg-bg-secondary border border-border 
            rounded-lg text-sm font-medium text-text-secondary hover:text-text 
            hover:border-border-strong transition-all disabled:opacity-50"
        >
          <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {/* Model Status & Media Reindex */}
      {modelStatus && (
        <div className="bg-bg-secondary border border-border rounded-lg p-5">
          <h3 className="font-medium text-text mb-4">AI Models Status</h3>
          
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
            {/* OCR Status */}
            <div className="flex items-center gap-3 p-3 bg-bg-tertiary/30 rounded-lg">
              <ImageIcon size={20} className={modelStatus.ocr.enabled ? 'text-status-success' : 'text-text-tertiary'} />
              <div className="flex-1">
                <div className="text-sm font-medium text-text">OCR (Images)</div>
                <div className="text-xs text-text-secondary">
                  {modelStatus.ocr.enabled ? `Enabled (${modelStatus.ocr.model})` : 'Disabled'}
                </div>
              </div>
              {mediaCounts.images > 0 && modelStatus.ocr.enabled && (
                <button
                  onClick={() => handleReindexMedia(['image'])}
                  disabled={isReindexingMedia}
                  className="text-xs px-3 py-1.5 bg-accent text-bg rounded hover:bg-accent-hover transition-colors disabled:opacity-50"
                >
                  Réindexer {mediaCounts.images} images
                </button>
              )}
            </div>

            {/* Transcription Status */}
            <div className="flex items-center gap-3 p-3 bg-bg-tertiary/30 rounded-lg">
              <div className="flex items-center gap-2">
                <Music size={18} className={modelStatus.transcription.enabled ? 'text-status-success' : 'text-text-tertiary'} />
                <Film size={18} className={modelStatus.transcription.enabled ? 'text-status-success' : 'text-text-tertiary'} />
              </div>
              <div className="flex-1">
                <div className="text-sm font-medium text-text">Transcription</div>
                <div className="text-xs text-text-secondary">
                  {modelStatus.transcription.enabled 
                    ? `Enabled (${modelStatus.transcription.model})` 
                    : 'Disabled'}
                </div>
                {/* Model download status */}
                {modelStatus.transcription.enabled && (
                  <div className="flex gap-2 mt-1">
                    {Object.entries(modelStatus.transcription.model_status).map(([model, status]) => (
                      <span 
                        key={model}
                        className={`text-[10px] px-1.5 py-0.5 rounded ${
                          status === 'ready' ? 'bg-status-success/20 text-status-success' :
                          status === 'downloading' ? 'bg-accent/20 text-accent animate-pulse' :
                          'bg-text-tertiary/20 text-text-tertiary'
                        }`}
                        title={status === 'not_downloaded' ? 'Will download on first use' : status}
                      >
                        {status === 'downloading' && <Download size={8} className="inline mr-1" />}
                        {model}
                      </span>
                    ))}
                  </div>
                )}
              </div>
              {(mediaCounts.audio > 0 || mediaCounts.video > 0) && modelStatus.transcription.enabled && (
                <div className="flex gap-1">
                  {mediaCounts.audio > 0 && (
                    <button
                      onClick={() => handleReindexMedia(['audio'])}
                      disabled={isReindexingMedia}
                      className="text-xs px-2 py-1.5 bg-accent text-bg rounded hover:bg-accent-hover transition-colors disabled:opacity-50"
                      title="Réindexer audio"
                    >
                      {mediaCounts.audio} audio
                    </button>
                  )}
                  {mediaCounts.video > 0 && (
                    <button
                      onClick={() => handleReindexMedia(['video'])}
                      disabled={isReindexingMedia}
                      className="text-xs px-2 py-1.5 bg-accent text-bg rounded hover:bg-accent-hover transition-colors disabled:opacity-50"
                      title="Réindexer vidéos"
                    >
                      {mediaCounts.video} vidéos
                    </button>
                  )}
                </div>
              )}
            </div>
          </div>

          {/* Global reindex button */}
          {(mediaCounts.images > 0 || mediaCounts.audio > 0 || mediaCounts.video > 0) && (
            <div className="flex items-center justify-between pt-3 border-t border-border">
              <span className="text-sm text-text-secondary">
                Les paramètres AI ont changé ? Réindexez vos médias pour appliquer les nouveaux réglages.
              </span>
              <button
                onClick={() => handleReindexMedia(['image', 'audio', 'video'])}
                disabled={isReindexingMedia}
                className="flex items-center gap-2 px-4 py-2 bg-accent text-bg font-medium 
                  rounded-lg hover:bg-accent-hover transition-colors disabled:opacity-50"
              >
                <RefreshCw size={16} className={isReindexingMedia ? 'animate-spin' : ''} />
                {isReindexingMedia ? 'Réindexation...' : 'Tout réindexer'}
              </button>
            </div>
          )}
        </div>
      )}

      {/* Add Document */}
      <div className="bg-bg-secondary border border-border rounded-lg p-5">
        <h3 className="font-medium text-text mb-3">Add Documents</h3>
        <div className="flex gap-3">
          <div className="flex-1 relative">
            <input
              type="text"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="Enter file or directory path..."
              className="w-full px-4 py-2.5 bg-bg border border-border rounded-lg 
                text-text placeholder-text-tertiary focus:outline-none focus:border-accent
                transition-colors"
            />
          </div>
          <button
            onClick={handleIndexFile}
            disabled={isIndexing || !path.trim()}
            className="flex items-center gap-2 px-4 py-2.5 bg-accent text-bg font-medium 
              rounded-lg hover:bg-accent-hover transition-colors disabled:opacity-50"
          >
            <FileText size={16} />
            Index File
          </button>
          <button
            onClick={handleIndexDirectory}
            disabled={isIndexing || !path.trim()}
            className="flex items-center gap-2 px-4 py-2.5 bg-bg-tertiary border border-border
              text-text font-medium rounded-lg hover:border-border-strong 
              transition-colors disabled:opacity-50"
          >
            <FolderOpen size={16} />
            Index Directory
          </button>
        </div>
        {message && (
          <p className={`mt-3 text-sm ${message.includes('success') || message.includes('cours') ? 'text-status-success' : 'text-status-error'}`}>
            {message}
          </p>
        )}
      </div>

      {/* Documents Table */}
      <DocumentTable 
        documents={documents} 
        onView={handleViewDocument}
        onDelete={handleDeleteDocument}
        onReindex={handleReindexDocument}
      />
    </div>
  );
}
