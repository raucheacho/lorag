import { useState } from 'react';
import { Check, ChevronRight, ChevronLeft, FolderOpen, Brain, Settings, Sparkles } from 'lucide-react';

interface WizardProps {
  onComplete: () => void;
}

interface WizardData {
  dataDir: string;
  folders: string[];
  embeddingProvider: 'ollama' | 'openai';
  embeddingModel: string;
  ollamaUrl: string;
}

export function Wizard({ onComplete }: WizardProps) {
  const [step, setStep] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [data, setData] = useState<WizardData>({
    dataDir: './lorag-data',
    folders: [],
    embeddingProvider: 'ollama',
    embeddingModel: 'nomic-embed-text',
    ollamaUrl: 'http://localhost:11434',
  });
  const [newFolder, setNewFolder] = useState('');
  const [status, setStatus] = useState<string>('');

  const steps = [
    { title: 'Bienvenue', icon: Sparkles },
    { title: 'Stockage', icon: FolderOpen },
    { title: 'Sources', icon: Settings },
    { title: 'Embeddings', icon: Brain },
  ];

  const handleAddFolder = () => {
    if (newFolder && !data.folders.includes(newFolder)) {
      setData({ ...data, folders: [...data.folders, newFolder] });
      setNewFolder('');
    }
  };

  const handleRemoveFolder = (folder: string) => {
    setData({ ...data, folders: data.folders.filter(f => f !== folder) });
  };

  const handleTestOllama = async () => {
    setStatus('Test de connexion...');
    try {
      const resp = await fetch(`${data.ollamaUrl}/api/tags`);
      if (resp.ok) {
        setStatus('✓ Connexion réussie à Ollama!');
      } else {
        setStatus('✗ Ollama ne répond pas correctement');
      }
    } catch {
      setStatus('✗ Impossible de se connecter à Ollama. Assurez-vous qu\'il est démarré: ollama serve');
    }
  };

  const handleComplete = async () => {
    setIsLoading(true);
    setStatus('Configuration en cours...');
    
    try {
      // Save configuration
      await fetch('/api/v1/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          dataDir: data.dataDir,
          embeddingProvider: data.embeddingProvider,
          embeddingModel: data.embeddingModel,
          embeddingUrl: data.ollamaUrl,
        }),
      });

      // Add watched folders
      for (const folder of data.folders) {
        await fetch('/api/v1/watches', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ path: folder, recursive: true }),
        });
      }

      setStatus('✓ Configuration terminée!');
      setTimeout(onComplete, 1000);
    } catch (err) {
      setStatus('✗ Erreur lors de la configuration');
    } finally {
      setIsLoading(false);
    }
  };

  const renderStep = () => {
    switch (step) {
      case 0:
        return (
          <div className="text-center space-y-6">
            <div className="w-20 h-20 bg-accent/10 rounded-full flex items-center justify-center mx-auto">
              <Sparkles size={40} className="text-accent" />
            </div>
            <h2 className="text-3xl font-bold text-text">Bienvenue sur Lorag!</h2>
            <p className="text-text-secondary max-w-md mx-auto">
              Lorag est votre mémoire locale universelle pour agents IA. 
              Vos documents restent 100% privés sur votre ordinateur.
            </p>
            <div className="bg-bg-secondary rounded-lg p-4 text-left max-w-md mx-auto">
              <h3 className="font-medium text-text mb-2">Ce qui va se passer:</h3>
              <ul className="space-y-2 text-sm text-text-secondary">
                <li className="flex items-center gap-2">
                  <Check size={16} className="text-status-success" />
                  Configuration du stockage des données
                </li>
                <li className="flex items-center gap-2">
                  <Check size={16} className="text-status-success" />
                  Sélection des dossiers à indexer
                </li>
                <li className="flex items-center gap-2">
                  <Check size={16} className="text-status-success" />
                  Configuration du moteur d'embeddings
                </li>
                <li className="flex items-center gap-2">
                  <Check size={16} className="text-status-success" />
                  Indexation automatique de vos documents
                </li>
              </ul>
            </div>
          </div>
        );

      case 1:
        return (
          <div className="space-y-6">
            <div className="text-center">
              <FolderOpen size={48} className="text-accent mx-auto mb-4" />
              <h2 className="text-2xl font-bold text-text">Où stocker les données?</h2>
              <p className="text-text-secondary mt-2">
                Choisissez où Lorag stockera son index et ses métadonnées.
              </p>
            </div>
            
            <div className="max-w-md mx-auto space-y-4">
              <div>
                <label className="block text-sm font-medium text-text mb-2">
                  Dossier de données
                </label>
                <input
                  type="text"
                  value={data.dataDir}
                  onChange={(e) => setData({ ...data, dataDir: e.target.value })}
                  className="w-full px-4 py-3 bg-bg-secondary border border-border rounded-lg text-text"
                  placeholder="./lorag-data"
                />
                <p className="text-xs text-text-tertiary mt-1">
                  ~500 Mo par 1000 documents indexés
                </p>
              </div>
            </div>
          </div>
        );

      case 2:
        return (
          <div className="space-y-6">
            <div className="text-center">
              <Settings size={48} className="text-accent mx-auto mb-4" />
              <h2 className="text-2xl font-bold text-text">Quels dossiers indexer?</h2>
              <p className="text-text-secondary mt-2">
                Sélectionnez les dossiers à surveiller. Les nouveaux fichiers seront automatiquement indexés.
              </p>
            </div>
            
            <div className="max-w-md mx-auto space-y-4">
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newFolder}
                  onChange={(e) => setNewFolder(e.target.value)}
                  placeholder="/chemin/vers/dossier"
                  className="flex-1 px-4 py-3 bg-bg-secondary border border-border rounded-lg text-text"
                />
                <button
                  onClick={handleAddFolder}
                  className="px-4 py-2 bg-accent text-bg rounded-lg hover:bg-accent-hover"
                >
                  Ajouter
                </button>
              </div>
              
              <div className="space-y-2">
                {data.folders.map((folder) => (
                  <div key={folder} className="flex items-center justify-between p-3 bg-bg-secondary rounded-lg">
                    <span className="text-sm text-text truncate">{folder}</span>
                    <button
                      onClick={() => handleRemoveFolder(folder)}
                      className="text-status-error hover:text-status-error/80"
                    >
                      Supprimer
                    </button>
                  </div>
                ))}
                {data.folders.length === 0 && (
                  <p className="text-center text-text-tertiary py-4">
                    Aucun dossier sélectionné. Vous pourrez en ajouter plus tard.
                  </p>
                )}
              </div>
            </div>
          </div>
        );

      case 3:
        return (
          <div className="space-y-6">
            <div className="text-center">
              <Brain size={48} className="text-accent mx-auto mb-4" />
              <h2 className="text-2xl font-bold text-text">Configuration des Embeddings</h2>
              <p className="text-text-secondary mt-2">
                Choisissez comment transformer vos documents en vecteurs.
              </p>
            </div>
            
            <div className="max-w-md mx-auto space-y-4">
              <div className="grid grid-cols-2 gap-3">
                <button
                  onClick={() => setData({ ...data, embeddingProvider: 'ollama' })}
                  className={`p-4 rounded-lg border text-left transition-colors ${
                    data.embeddingProvider === 'ollama'
                      ? 'border-accent bg-accent/10'
                      : 'border-border hover:border-accent/50'
                  }`}
                >
                  <div className="font-medium text-text">Ollama (Local)</div>
                  <div className="text-xs text-text-tertiary mt-1">
                    100% offline, gratuit
                  </div>
                </button>
                <button
                  onClick={() => setData({ ...data, embeddingProvider: 'openai' })}
                  className={`p-4 rounded-lg border text-left transition-colors ${
                    data.embeddingProvider === 'openai'
                      ? 'border-accent bg-accent/10'
                      : 'border-border hover:border-accent/50'
                  }`}
                >
                  <div className="font-medium text-text">OpenAI</div>
                  <div className="text-xs text-text-tertiary mt-1">
                    Nécessite une clé API
                  </div>
                </button>
              </div>

              {data.embeddingProvider === 'ollama' && (
                <div className="space-y-3">
                  <div>
                    <label className="block text-sm font-medium text-text mb-1">
                      URL Ollama
                    </label>
                    <input
                      type="text"
                      value={data.ollamaUrl}
                      onChange={(e) => setData({ ...data, ollamaUrl: e.target.value })}
                      className="w-full px-4 py-2 bg-bg-secondary border border-border rounded-lg text-text"
                    />
                  </div>
                  <button
                    onClick={handleTestOllama}
                    className="text-sm text-accent hover:underline"
                  >
                    Tester la connexion
                  </button>
                  {status && (
                    <p className={`text-sm ${status.includes('✓') ? 'text-status-success' : 'text-status-error'}`}>
                      {status}
                    </p>
                  )}
                </div>
              )}

              <div className="bg-bg-secondary rounded-lg p-4 text-sm text-text-secondary">
                <p className="font-medium text-text mb-1">💡 Conseil</p>
                <p>
                  Ollama est recommandé pour la confidentialité. Installez-le avec:{' '}
                  <code className="bg-bg-tertiary px-1 rounded">curl -fsSL https://ollama.com/install.sh | sh</code>
                </p>
              </div>
            </div>
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <div className="fixed inset-0 bg-bg/95 backdrop-blur-sm flex items-center justify-center z-50">
      <div className="bg-bg-secondary border border-border rounded-2xl shadow-2xl w-full max-w-2xl mx-4 overflow-hidden">
        {/* Progress */}
        <div className="bg-bg-tertiary px-6 py-4 border-b border-border">
          <div className="flex items-center justify-between">
            {steps.map((s, i) => (
              <div key={i} className="flex items-center">
                <div className={`w-8 h-8 rounded-full flex items-center justify-center ${
                  i <= step ? 'bg-accent text-bg' : 'bg-bg-secondary text-text-tertiary'
                }`}>
                  <s.icon size={16} />
                </div>
                {i < steps.length - 1 && (
                  <div className={`w-12 h-0.5 mx-2 ${
                    i < step ? 'bg-accent' : 'bg-border'
                  }`} />
                )}
              </div>
            ))}
          </div>
        </div>

        {/* Content */}
        <div className="p-8 min-h-[400px]">
          {renderStep()}
        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-border flex justify-between">
          <button
            onClick={() => setStep(Math.max(0, step - 1))}
            disabled={step === 0}
            className="flex items-center gap-2 px-4 py-2 text-text-secondary hover:text-text disabled:opacity-50"
          >
            <ChevronLeft size={18} />
            Précédent
          </button>
          
          {step < steps.length - 1 ? (
            <button
              onClick={() => setStep(step + 1)}
              className="flex items-center gap-2 px-6 py-2 bg-accent text-bg rounded-lg hover:bg-accent-hover"
            >
              Suivant
              <ChevronRight size={18} />
            </button>
          ) : (
            <button
              onClick={handleComplete}
              disabled={isLoading}
              className="flex items-center gap-2 px-6 py-2 bg-accent text-bg rounded-lg hover:bg-accent-hover disabled:opacity-50"
            >
              {isLoading ? 'Configuration...' : 'Terminer'}
              <Check size={18} />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
