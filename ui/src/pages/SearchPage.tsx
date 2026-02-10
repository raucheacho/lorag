import { useState, useEffect } from 'react';
import { Search, Loader2, FileText, SlidersHorizontal, Sparkles } from 'lucide-react';
import { searchDocuments } from '../hooks/useApi';
import { showToast } from '../components/Toaster';
import type { SearchResult } from '../types';

interface Template {
  id: string;
  name: string;
  content: string;
}

export function SearchPage() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const [showFilters, setShowFilters] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<string>('default');
  const [topK, setTopK] = useState(10);
  const [minScore, setMinScore] = useState(0);
  const [templates, setTemplates] = useState<Template[]>([]);

  useEffect(() => {
    // Load templates from localStorage
    const saved = localStorage.getItem('lorag-templates');
    if (saved) {
      setTemplates(JSON.parse(saved));
    }
  }, []);

  const handleSearch = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!query.trim()) return;

    setLoading(true);
    setSearched(true);
    try {
      const data = await searchDocuments(query, topK, minScore);
      setResults(data);
      if (data.length === 0 && searched) {
        showToast('info', 'Aucun résultat trouvé');
      }
    } catch (err) {
      console.error('Search failed:', err);
      showToast('error', 'Erreur lors de la recherche');
    } finally {
      setLoading(false);
    }
  };

  const copyWithTemplate = () => {
    const template = templates.find(t => t.id === selectedTemplate);
    if (!template || results.length === 0) return;

    const context = results.map((r, i) => 
      `[${i + 1}] ${r.source.filename}:\n${r.content}`
    ).join('\n\n---\n\n');

    const prompt = template.content
      .replace(/{{query}}/g, query)
      .replace(/{{context}}/g, context);

    navigator.clipboard.writeText(prompt);
    showToast('success', 'Prompt copié dans le presse-papiers');
  };

  // Score is already similarity (0-1), no need to transform
  const formatScore = (score: number) => {
    return score.toFixed(3);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-2xl font-semibold text-text">Recherche Sémantique</h2>
        <p className="text-text-tertiary mt-1">
          Recherchez dans vos documents en langage naturel
        </p>
      </div>

      {/* Search Input */}
      <form onSubmit={handleSearch} className="relative">
        <div className="relative">
          <Search 
            size={20} 
            className="absolute left-4 top-1/2 -translate-y-1/2 text-text-tertiary" 
          />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Posez une question sur vos documents..."
            className="w-full pl-12 pr-32 py-4 bg-bg-secondary border border-border rounded-xl
              text-lg text-text placeholder-text-tertiary focus:outline-none focus:border-accent
              transition-colors"
          />
          <button
            type="submit"
            disabled={loading || !query.trim()}
            className="absolute right-3 top-1/2 -translate-y-1/2 px-6 py-2 bg-accent 
              text-bg font-medium rounded-lg hover:bg-accent-hover transition-colors
              disabled:opacity-50 flex items-center gap-2"
          >
            {loading ? (
              <>
                <Loader2 size={16} className="animate-spin" />
                Recherche...
              </>
            ) : (
              'Rechercher'
            )}
          </button>
        </div>
      </form>

      {/* Filters Bar */}
      <div className="flex items-center justify-between">
        <button
          onClick={() => setShowFilters(!showFilters)}
          className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm transition-colors ${
            showFilters ? 'bg-accent/10 text-accent' : 'text-text-secondary hover:bg-bg-secondary'
          }`}
        >
          <SlidersHorizontal size={16} />
          Filtres
        </button>

        {results.length > 0 && templates.length > 0 && (
          <div className="flex items-center gap-3">
            <select
              value={selectedTemplate}
              onChange={(e) => setSelectedTemplate(e.target.value)}
              className="px-3 py-1.5 bg-bg-secondary border border-border rounded-lg text-sm text-text"
            >
              {templates.map(t => (
                <option key={t.id} value={t.id}>{t.name}</option>
              ))}
            </select>
            <button
              onClick={copyWithTemplate}
              className="flex items-center gap-2 px-3 py-1.5 bg-accent/10 text-accent 
                rounded-lg text-sm hover:bg-accent/20"
            >
              <Sparkles size={14} />
              Copier pour IA
            </button>
          </div>
        )}
      </div>

      {/* Filters Panel */}
      {showFilters && (
        <div className="bg-bg-secondary border border-border rounded-lg p-4 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-text-secondary mb-1">
                Nombre de résultats
              </label>
              <select
                value={topK}
                onChange={(e) => setTopK(parseInt(e.target.value))}
                className="w-full px-3 py-2 bg-bg border border-border rounded text-text"
              >
                <option value={5}>5</option>
                <option value={10}>10</option>
                <option value={20}>20</option>
                <option value={50}>50</option>
              </select>
            </div>
            <div>
              <label className="block text-sm text-text-secondary mb-1">
                Score minimum
              </label>
              <select
                value={minScore}
                onChange={(e) => setMinScore(parseFloat(e.target.value))}
                className="w-full px-3 py-2 bg-bg border border-border rounded text-text"
              >
                <option value={0}>Aucun</option>
                <option value={0.5}>0.5 (faible)</option>
                <option value={0.7}>0.7 (moyen)</option>
                <option value={0.85}>0.85 (fort)</option>
              </select>
            </div>
          </div>
        </div>
      )}

      {/* Results */}
      {searched && !loading && results.length === 0 && (
        <div className="text-center py-12">
          <FileText size={48} className="mx-auto text-text-tertiary mb-4" />
          <h3 className="text-lg font-medium text-text">Aucun résultat</h3>
          <p className="text-text-tertiary mt-1">
            Essayez avec d'autres mots-clés ou vérifiez vos documents indexés
          </p>
        </div>
      )}

      <div className="space-y-4">
        {results.map((result, index) => (
          <div 
            key={result.chunk_id}
            className="bg-bg-secondary border border-border rounded-lg p-5 
              hover:border-border-strong transition-colors"
          >
            <div className="flex items-start justify-between mb-3">
              <div className="flex items-center gap-3">
                <span className="text-xs font-mono text-text-tertiary bg-bg-tertiary px-2 py-1 rounded">
                  #{index + 1}
                </span>
                <span className="text-sm font-medium text-text">
                  {result.source.filename}
                </span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-xs text-text-tertiary uppercase tracking-wide">
                  Pertinence
                </span>
                <span className={`text-sm font-mono font-medium ${
                  result.score > 0.8 ? 'text-status-success' : 
                  result.score > 0.6 ? 'text-accent' : 'text-text-tertiary'
                }`}>
                  {formatScore(result.score)}
                </span>
              </div>
            </div>
            
            <p className="text-text-secondary leading-relaxed">
              {result.content}
            </p>
            
            <div className="mt-4 pt-3 border-t border-border flex items-center gap-4 text-xs text-text-tertiary">
              <span>Position: {result.source.position}</span>
              <span>Octets: {result.source.start_byte}-{result.source.end_byte}</span>
              <span className="truncate max-w-[300px]" title={result.source.path}>
                {result.source.path}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* Help text */}
      {!searched && (
        <div className="bg-accent/5 border border-accent/20 rounded-lg p-6 text-center">
          <Sparkles size={32} className="text-accent mx-auto mb-3" />
          <h3 className="font-medium text-text mb-2">Comment ça marche?</h3>
          <p className="text-text-secondary text-sm max-w-md mx-auto">
            Tapez votre question en langage naturel. Lorag comprend le sens de votre recherche 
            et trouve les passages les plus pertinents dans vos documents, même si les mots 
            sont différents.
          </p>
        </div>
      )}
    </div>
  );
}
