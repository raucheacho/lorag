import { useState, useEffect } from 'react';
import { 
  Save, RotateCcw, AlertTriangle, Check, Database, Brain, 
  FolderOpen, Shield, Server, Sliders 
} from 'lucide-react';

interface Settings {
  // Server
  host: string;
  port: number;
  
  // Embeddings
  embeddingProvider: string;
  embeddingModel: string;
  embeddingUrl: string;
  embeddingDim: number;
  
  // Vector Index
  vectorIndexType: string;
  vectorM: number;
  vectorEf: number;
  
  // Indexing
  maxChunks: number;
  chunkSize: number;
  chunkOverlap: number;
  indexWorkers: number;
  
  // OCR & Vision
  ocrEnabled: boolean;
  ocrModel: string;
  transcriptionEnabled: boolean;
  whisperModel: 'tiny' | 'base' | 'small';

  // Features
  encryptionEnabled: boolean;
}

const defaultSettings: Settings = {
  host: 'localhost',
  port: 8080,
  embeddingProvider: 'ollama',
  embeddingModel: 'nomic-embed-text',
  embeddingUrl: 'http://localhost:11434',
  embeddingDim: 768,
  vectorIndexType: 'hnsw',
  vectorM: 16,
  vectorEf: 200,
  maxChunks: 50000,
  chunkSize: 512,
  chunkOverlap: 50,
  indexWorkers: 3,
  ocrEnabled: false,
  ocrModel: 'llava:7b',
  transcriptionEnabled: false,
  whisperModel: 'tiny',
  encryptionEnabled: false,
};

export function SettingsPage() {
  const [settings, setSettings] = useState<Settings>(defaultSettings);
  const [activeTab, setActiveTab] = useState('general');
  const [isSaving, setIsSaving] = useState(false);
  const [notification, setNotification] = useState<{type: 'success'|'error', message: string} | null>(null);
  const [testStatus, setTestStatus] = useState<Record<string, string>>({});

  useEffect(() => {
    // Load settings from API and convert snake_case to camelCase
    fetch('/api/v1/settings')
      .then(r => r.json())
      .then(data => {
        const converted = {
          ...defaultSettings,
          host: data.host || defaultSettings.host,
          port: data.port || defaultSettings.port,
          embeddingProvider: data.embedding_provider || defaultSettings.embeddingProvider,
          embeddingModel: data.embedding_model || defaultSettings.embeddingModel,
          embeddingUrl: data.embedding_url || defaultSettings.embeddingUrl,
          embeddingDim: data.embedding_dim || defaultSettings.embeddingDim,
          vectorIndexType: data.vector_index_type || defaultSettings.vectorIndexType,
          vectorM: data.vector_m || defaultSettings.vectorM,
          vectorEf: data.vector_ef || defaultSettings.vectorEf,
          maxChunks: data.max_chunks || defaultSettings.maxChunks,
          chunkSize: data.chunk_size || defaultSettings.chunkSize,
          chunkOverlap: data.chunk_overlap || defaultSettings.chunkOverlap,
          indexWorkers: data.index_workers || defaultSettings.indexWorkers,
          ocrEnabled: data.ocr_enabled ?? defaultSettings.ocrEnabled,
          ocrModel: data.ocr_model || defaultSettings.ocrModel,
          transcriptionEnabled: data.transcription_enabled ?? defaultSettings.transcriptionEnabled,
          whisperModel: data.whisper_model || defaultSettings.whisperModel,
          encryptionEnabled: data.encryption_enabled ?? defaultSettings.encryptionEnabled,
        };
        setSettings(converted);
      })
      .catch(() => setNotification({ type: 'error', message: 'Impossible de charger les paramètres' }));
  }, []);

  const handleSave = async () => {
    setIsSaving(true);
    try {
      // Convert camelCase to snake_case for backend compatibility
      const payload = {
        host: settings.host,
        port: settings.port,
        embedding_provider: settings.embeddingProvider,
        embedding_model: settings.embeddingModel,
        embedding_url: settings.embeddingUrl,
        embedding_dim: settings.embeddingDim,
        vector_index_type: settings.vectorIndexType,
        vector_m: settings.vectorM,
        vector_ef: settings.vectorEf,
        max_chunks: settings.maxChunks,
        chunk_size: settings.chunkSize,
        chunk_overlap: settings.chunkOverlap,
        index_workers: settings.indexWorkers,
        ocr_enabled: settings.ocrEnabled,
        ocr_model: settings.ocrModel,
        transcription_enabled: settings.transcriptionEnabled,
        whisper_model: settings.whisperModel,
        encryption_enabled: settings.encryptionEnabled,
      };
      
      const resp = await fetch('/api/v1/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      if (resp.ok) {
        setNotification({ type: 'success', message: 'Paramètres sauvegardés' });
      } else {
        const err = await resp.text();
        console.error('Save error:', err);
        setNotification({ type: 'error', message: 'Erreur lors de la sauvegarde' });
      }
    } catch (err) {
      console.error('Connection error:', err);
      setNotification({ type: 'error', message: 'Erreur de connexion' });
    } finally {
      setIsSaving(false);
      setTimeout(() => setNotification(null), 3000);
    }
  };

  const handleReset = () => {
    if (confirm('Réinitialiser tous les paramètres?')) {
      setSettings(defaultSettings);
    }
  };

  const testOllamaConnection = async () => {
    setTestStatus({ ...testStatus, ollama: 'Test en cours...' });
    try {
      const resp = await fetch(`${settings.embeddingUrl}/api/tags`);
      if (resp.ok) {
        setTestStatus({ ...testStatus, ollama: '✓ Connecté' });
      } else {
        setTestStatus({ ...testStatus, ollama: '✗ Erreur de réponse' });
      }
    } catch {
      setTestStatus({ ...testStatus, ollama: '✗ Non accessible' });
    }
  };

  const tabs = [
    { id: 'general', label: 'Général', icon: Server },
    { id: 'embeddings', label: 'Embeddings', icon: Brain },
    { id: 'vector', label: 'Vector DB', icon: Database },
    { id: 'indexing', label: 'Indexation', icon: FolderOpen },
    { id: 'advanced', label: 'Avancé', icon: Sliders },
  ];

  const renderTabContent = () => {
    switch (activeTab) {
      case 'general':
        return (
          <div className="space-y-6">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text mb-1">
                  Hôte
                </label>
                <input
                  type="text"
                  value={settings.host}
                  onChange={(e) => setSettings({ ...settings, host: e.target.value })}
                  className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
                />
                <p className="text-xs text-text-tertiary mt-1">
                  localhost pour accès local uniquement
                </p>
              </div>
              <div>
                <label className="block text-sm font-medium text-text mb-1">
                  Port
                </label>
                <input
                  type="number"
                  value={settings.port}
                  onChange={(e) => setSettings({ ...settings, port: parseInt(e.target.value) })}
                  className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
                />
                <p className="text-xs text-text-tertiary mt-1">
                  Redémarrage requis après modification
                </p>
              </div>
            </div>
          </div>
        );

      case 'embeddings':
        return (
          <div className="space-y-6">
            <div>
              <label className="block text-sm font-medium text-text mb-2">
                Provider d'Embeddings
              </label>
              <div className="grid grid-cols-2 gap-3">
                {['ollama', 'openai'].map((provider) => (
                  <button
                    key={provider}
                    onClick={() => setSettings({ ...settings, embeddingProvider: provider })}
                    className={`p-3 border rounded-lg text-left capitalize ${
                      settings.embeddingProvider === provider
                        ? 'border-accent bg-accent/5'
                        : 'border-border hover:border-accent/50'
                    }`}
                  >
                    <div className="font-medium text-text">{provider}</div>
                    <div className="text-xs text-text-tertiary">
                      {provider === 'ollama' ? 'Local & gratuit' : 'API cloud'}
                    </div>
                  </button>
                ))}
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-text mb-1">
                URL du Service
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={settings.embeddingUrl}
                  onChange={(e) => setSettings({ ...settings, embeddingUrl: e.target.value })}
                  className="flex-1 px-3 py-2 bg-bg border border-border rounded-lg text-text"
                />
                <button
                  onClick={testOllamaConnection}
                  className="px-3 py-2 border border-border rounded-lg text-text hover:bg-bg-secondary"
                >
                  Tester
                </button>
              </div>
              {testStatus.ollama && (
                <p className={`text-xs mt-1 ${testStatus.ollama.includes('✓') ? 'text-status-success' : 'text-status-error'}`}>
                  {testStatus.ollama}
                </p>
              )}
            </div>

            <div>
              <label className="block text-sm font-medium text-text mb-1">
                Modèle
              </label>
              <input
                type="text"
                value={settings.embeddingModel}
                onChange={(e) => setSettings({ ...settings, embeddingModel: e.target.value })}
                className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
              />
              <p className="text-xs text-text-tertiary mt-1">
                nomic-embed-text recommandé pour Ollama
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-text mb-1">
                Dimensions
              </label>
              <select
                value={settings.embeddingDim}
                onChange={(e) => setSettings({ ...settings, embeddingDim: parseInt(e.target.value) })}
                className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
              >
                <option value={384}>384 (petit, rapide)</option>
                <option value={768}>768 (standard)</option>
                <option value={1024}>1024 (précis)</option>
                <option value={1536}>1536 (OpenAI)</option>
              </select>
            </div>
          </div>
        );

      case 'vector':
        return (
          <div className="space-y-6">
            <div className="bg-accent/5 border border-accent/20 rounded-lg p-4">
              <h4 className="font-medium text-text mb-1">À propos de HNSW</h4>
              <p className="text-sm text-text-secondary">
                HNSW (Hierarchical Navigable Small World) est un algorithme de recherche 
                approximative rapide. Plus M et Ef sont élevés, plus la recherche est précise 
                mais consomme de mémoire.
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-text mb-1">
                Type d'Index
              </label>
              <select
                value={settings.vectorIndexType}
                onChange={(e) => setSettings({ ...settings, vectorIndexType: e.target.value })}
                className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
              >
                <option value="hnsw">HNSW (recommandé)</option>
                <option value="flat">Flat (précis mais lent)</option>
              </select>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text mb-1">
                  M (Connexions)
                </label>
                <input
                  type="number"
                  min={4}
                  max={64}
                  value={settings.vectorM}
                  onChange={(e) => setSettings({ ...settings, vectorM: parseInt(e.target.value) })}
                  className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
                />
                <p className="text-xs text-text-tertiary mt-1">16 par défaut</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-text mb-1">
                  Ef Construction
                </label>
                <input
                  type="number"
                  min={50}
                  max={500}
                  value={settings.vectorEf}
                  onChange={(e) => setSettings({ ...settings, vectorEf: parseInt(e.target.value) })}
                  className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
                />
                <p className="text-xs text-text-tertiary mt-1">200 par défaut</p>
              </div>
            </div>
          </div>
        );

      case 'indexing':
        return (
          <div className="space-y-6">
            <div>
              <label className="block text-sm font-medium text-text mb-1">
                Taille des Chunks
              </label>
              <input
                type="number"
                min={100}
                max={2000}
                value={settings.chunkSize}
                onChange={(e) => setSettings({ ...settings, chunkSize: parseInt(e.target.value) })}
                className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
              />
              <p className="text-xs text-text-tertiary mt-1">
                Nombre de caractères par segment (512 recommandé)
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-text mb-1">
                Chevauchement
              </label>
              <input
                type="number"
                min={0}
                max={500}
                value={settings.chunkOverlap}
                onChange={(e) => setSettings({ ...settings, chunkOverlap: parseInt(e.target.value) })}
                className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
              />
              <p className="text-xs text-text-tertiary mt-1">
                Caractères de chevauchement entre chunks (50 recommandé)
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-text mb-1">
                Workers d'Indexation
              </label>
              <input
                type="number"
                min={1}
                max={10}
                value={settings.indexWorkers}
                onChange={(e) => setSettings({ ...settings, indexWorkers: parseInt(e.target.value) })}
                className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
              />
              <p className="text-xs text-text-tertiary mt-1">
                Processus parallèles pour l'indexation
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-text mb-1">
                Limite de Chunks
              </label>
              <input
                type="number"
                min={1000}
                max={1000000}
                step={1000}
                value={settings.maxChunks}
                onChange={(e) => setSettings({ ...settings, maxChunks: parseInt(e.target.value) })}
                className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
              />
              <p className="text-xs text-text-tertiary mt-1">
                Nombre maximum de chunks stockés
              </p>
            </div>
          </div>
        );

      case 'advanced':
        return (
          <div className="space-y-6">
            <div className="flex items-center justify-between p-4 border border-border rounded-lg">
              <div className="flex items-center gap-3">
                <Shield size={20} className="text-accent" />
                <div>
                  <div className="font-medium text-text">OCR (Reconnaissance de texte)</div>
                  <div className="text-sm text-text-secondary">
                    Extraire le texte des images et PDF scannés avec un modèle IA
                  </div>
                </div>
              </div>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  checked={settings.ocrEnabled}
                  onChange={(e) => setSettings({ ...settings, ocrEnabled: e.target.checked })}
                  className="sr-only peer"
                />
                <div className="w-11 h-6 bg-bg-tertiary peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-accent"></div>
              </label>
            </div>

            {settings.ocrEnabled && (
              <div className="ml-4 p-4 bg-bg-tertiary/30 rounded-lg">
                <label className="block text-sm font-medium text-text mb-1">
                  Modèle OCR (Ollama)
                </label>
                <input
                  type="text"
                  value={settings.ocrModel}
                  onChange={(e) => setSettings({ ...settings, ocrModel: e.target.value })}
                  placeholder="llava:7b"
                  className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
                />
                <p className="text-xs text-text-tertiary mt-1">
                  Modèle Ollama vision (ex: llava:7b, llava:latest)
                </p>
              </div>
            )}

            {settings.ocrEnabled && (
              <div className="ml-4 p-4 bg-bg-tertiary/30 rounded-lg text-sm text-text-secondary">
                <p className="mb-2"><strong>Prérequis pour l'OCR :</strong></p>
                <ul className="list-disc list-inside space-y-1 text-xs">
                  <li>Ollama doit être démarré : <code>ollama serve</code></li>
                  <li>Installer un modèle vision : <code>ollama pull llava:7b</code></li>
                </ul>
              </div>
            )}

            <div className="flex items-center justify-between p-4 border border-border rounded-lg">
              <div className="flex items-center gap-3">
                <Shield size={20} className="text-accent" />
                <div>
                  <div className="font-medium text-text">Transcription Audio/Vidéo</div>
                  <div className="text-sm text-text-secondary">
                    Transcrire automatiquement avec Whisper local
                  </div>
                </div>
              </div>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  checked={settings.transcriptionEnabled}
                  onChange={(e) => setSettings({ ...settings, transcriptionEnabled: e.target.checked })}
                  className="sr-only peer"
                />
                <div className="w-11 h-6 bg-bg-tertiary peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-accent"></div>
              </label>
            </div>

            {settings.transcriptionEnabled && (
              <>
                <div className="ml-4 p-4 bg-bg-tertiary/30 rounded-lg">
                  <label className="block text-sm font-medium text-text mb-2">
                    Modèle Whisper
                  </label>
                  <select
                    value={settings.whisperModel}
                    onChange={(e) => setSettings({ ...settings, whisperModel: e.target.value as 'tiny' | 'base' | 'small' })}
                    className="w-full px-3 py-2 bg-bg border border-border rounded-lg text-text"
                  >
                    <option value="tiny">Tiny (39 MB) - Rapide, qualité standard</option>
                    <option value="base">Base (74 MB) - Bon équilibre</option>
                    <option value="small">Small (244 MB) - Haute qualité</option>
                  </select>
                  <p className="text-xs text-text-tertiary mt-1">
                    Le modèle sera téléchargé automatiquement au premier usage
                  </p>
                </div>

                <div className="ml-4 p-4 bg-bg-tertiary/30 rounded-lg text-sm text-text-secondary">
                  <p className="mb-2"><strong>Prérequis pour la transcription :</strong></p>
                  <ul className="list-disc list-inside space-y-1 text-xs">
                    <li>whisper.cpp doit être installé : <code>make whisper-cli</code> dans whisper.cpp</li>
                    <li>Pour les vidéos : <code>ffmpeg</code> doit être installé</li>
                  </ul>
                </div>
              </>
            )}

            <div className="flex items-center justify-between p-4 border border-border rounded-lg">
              <div className="flex items-center gap-3">
                <Shield size={20} className="text-accent" />
                <div>
                  <div className="font-medium text-text">Chiffrement</div>
                  <div className="text-sm text-text-secondary">
                    Chiffrer les données sur disque (AES-256)
                  </div>
                </div>
              </div>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  checked={settings.encryptionEnabled}
                  onChange={(e) => setSettings({ ...settings, encryptionEnabled: e.target.checked })}
                  className="sr-only peer"
                />
                <div className="w-11 h-6 bg-bg-tertiary peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-accent"></div>
              </label>
            </div>

            {settings.encryptionEnabled && (
              <div className="bg-status-error/10 border border-status-error/30 rounded-lg p-4 flex items-start gap-3">
                <AlertTriangle size={20} className="text-status-error flex-shrink-0 mt-0.5" />
                <div className="text-sm text-text-secondary">
                  <p className="font-medium text-status-error">Attention</p>
                  <p>La perte de la clé de chiffrement entraînera la perte irréversible de vos données.</p>
                </div>
              </div>
            )}
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-text">Paramètres</h2>
          <p className="text-text-tertiary mt-1">
            Configurez Lorag selon vos besoins
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={handleReset}
            className="flex items-center gap-2 px-4 py-2 border border-border text-text rounded-lg hover:bg-bg-secondary"
          >
            <RotateCcw size={18} />
            Réinitialiser
          </button>
          <button
            onClick={handleSave}
            disabled={isSaving}
            className="flex items-center gap-2 px-4 py-2 bg-accent text-bg rounded-lg hover:bg-accent-hover disabled:opacity-50"
          >
            <Save size={18} />
            {isSaving ? 'Sauvegarde...' : 'Sauvegarder'}
          </button>
        </div>
      </div>

      {/* Notification */}
      {notification && (
        <div className={`fixed top-4 right-4 px-4 py-2 rounded-lg shadow-lg animate-fade-in flex items-center gap-2 ${
          notification.type === 'success' ? 'bg-status-success text-bg' : 'bg-status-error text-bg'
        }`}>
          {notification.type === 'success' ? <Check size={18} /> : <AlertTriangle size={18} />}
          {notification.message}
        </div>
      )}

      <div className="flex gap-6">
        {/* Sidebar */}
        <div className="w-48 flex-shrink-0 space-y-1">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-left transition-colors ${
                activeTab === tab.id
                  ? 'bg-accent/10 text-accent'
                  : 'text-text-secondary hover:bg-bg-secondary hover:text-text'
              }`}
            >
              <tab.icon size={18} />
              {tab.label}
            </button>
          ))}
        </div>

        {/* Content */}
        <div className="flex-1 bg-bg-secondary border border-border rounded-xl p-6">
          <h3 className="text-lg font-medium text-text mb-6 capitalize">
            {tabs.find(t => t.id === activeTab)?.label}
          </h3>
          {renderTabContent()}
        </div>
      </div>
    </div>
  );
}
