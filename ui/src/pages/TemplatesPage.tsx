import { useState, useEffect } from 'react';
import { FileText, Plus, Trash2, Edit2, Check, X, Copy, Sparkles } from 'lucide-react';

interface Template {
  id: string;
  name: string;
  description: string;
  content: string;
  isDefault?: boolean;
}

const defaultTemplates: Template[] = [
  {
    id: 'default',
    name: 'Standard',
    description: 'Réponse équilibrée avec contexte',
    content: `Basé sur les documents suivants:

{{context}}

Question: {{query}}

Réponds de manière claire et précise en te basant uniquement sur le contexte fourni.`,
    isDefault: true,
  },
  {
    id: 'concise',
    name: 'Concis',
    description: 'Réponse courte et directe',
    content: `Contexte:
{{context}}

Question: {{query}}

Réponds en 2-3 phrases maximum.`,
    isDefault: true,
  },
  {
    id: 'detailed',
    name: 'Détaillé',
    description: 'Réponse complète avec citations',
    content: `Documents de référence:
{{context}}

Question: {{query}}

Fournis une réponse détaillée en citant les sources [1], [2], etc. Structure ta réponse avec des paragraphes clairs.`,
    isDefault: true,
  },
  {
    id: 'academic',
    name: 'Académique',
    description: 'Style formel et structuré',
    content: `Sources:
{{context}}

Requête: {{query}}

Rédige une réponse académique structurée avec:
1. Introduction
2. Points clés
3. Conclusion

Cite explicitement les sources.`,
    isDefault: true,
  },
  {
    id: 'bullet-points',
    name: 'Liste à puces',
    description: 'Points clés formatés',
    content: `Contexte:
{{context}}

Question: {{query}}

Réponds sous forme de liste à puces.`,
    isDefault: true,
  },
];

export function TemplatesPage() {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [editing, setEditing] = useState<string | null>(null);
  const [editContent, setEditContent] = useState('');
  const [editName, setEditName] = useState('');
  const [editDescription, setEditDescription] = useState('');
  const [showNewForm, setShowNewForm] = useState(false);
  const [previewQuery, setPreviewQuery] = useState('Comment configurer un serveur?');
  const [previewContext, setPreviewContext] = useState('Source 1: Pour configurer un serveur, vous devez d\'abord installer Nginx...\n\nSource 2: La configuration recommandée inclut SSL...');
  const [selectedTemplate, setSelectedTemplate] = useState<string>('default');
  const [notification, setNotification] = useState<string>('');
  const [defaultTemplateId, setDefaultTemplateId] = useState<string>('default');

  useEffect(() => {
    // Load templates from localStorage or use defaults
    const saved = localStorage.getItem('lorag-templates');
    if (saved) {
      setTemplates(JSON.parse(saved));
    } else {
      setTemplates(defaultTemplates);
    }
    
    // Load default template from server settings
    fetch('/api/v1/settings')
      .then(r => r.json())
      .then(data => {
        if (data.default_template) {
          setDefaultTemplateId(data.default_template);
        }
      })
      .catch(() => {});
  }, []);

  const saveTemplates = (newTemplates: Template[]) => {
    setTemplates(newTemplates);
    localStorage.setItem('lorag-templates', JSON.stringify(newTemplates));
  };

  const handleStartEdit = (t: Template) => {
    setEditing(t.id);
    setEditName(t.name);
    setEditDescription(t.description);
    setEditContent(t.content);
  };

  const handleSaveEdit = () => {
    if (!editing) return;
    
    const updated = templates.map(t => 
      t.id === editing 
        ? { ...t, name: editName, description: editDescription, content: editContent }
        : t
    );
    saveTemplates(updated);
    setEditing(null);
    showNotification('Template mis à jour');
  };

  const handleCancelEdit = () => {
    setEditing(null);
    setEditName('');
    setEditDescription('');
    setEditContent('');
  };

  const handleDelete = (id: string) => {
    const updated = templates.filter(t => t.id !== id);
    saveTemplates(updated);
    showNotification('Template supprimé');
  };

  const handleCreate = () => {
    const newTemplate: Template = {
      id: `custom-${Date.now()}`,
      name: editName || 'Nouveau template',
      description: editDescription || 'Template personnalisé',
      content: editContent || 'Contexte:\n{{context}}\n\nQuestion: {{query}}\n\n',
    };
    saveTemplates([...templates, newTemplate]);
    setShowNewForm(false);
    setEditName('');
    setEditDescription('');
    setEditContent('');
    showNotification('Template créé');
  };

  const handleDuplicate = (t: Template) => {
    const copy: Template = {
      ...t,
      id: `custom-${Date.now()}`,
      name: `${t.name} (copie)`,
      isDefault: false,
    };
    saveTemplates([...templates, copy]);
    showNotification('Template dupliqué');
  };

  const handleSetDefault = async (templateId: string) => {
    setDefaultTemplateId(templateId);
    
    // Save to server
    try {
      const currentSettings = await fetch('/api/v1/settings').then(r => r.json());
      const updatedSettings = { ...currentSettings, default_template: templateId };
      
      await fetch('/api/v1/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updatedSettings),
      });
      
      showNotification('Template par défaut mis à jour');
    } catch (err) {
      showNotification('Erreur lors de la sauvegarde');
    }
  };

  const showNotification = (msg: string) => {
    setNotification(msg);
    setTimeout(() => setNotification(''), 3000);
  };

  const getPreview = (template: Template) => {
    return template.content
      .replace(/{{query}}/g, previewQuery)
      .replace(/{{context}}/g, previewContext);
  };

  const currentTemplate = templates.find(t => t.id === selectedTemplate) || templates[0];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-text">Templates de Prompts</h2>
          <p className="text-text-tertiary mt-1">
            Personnalisez comment les résultats sont présentés aux IA
          </p>
        </div>
        <button
          onClick={() => {
            setShowNewForm(true);
            setEditName('');
            setEditDescription('');
            setEditContent('');
          }}
          className="flex items-center gap-2 px-4 py-2 bg-accent text-bg rounded-lg hover:bg-accent-hover"
        >
          <Plus size={18} />
          Nouveau Template
        </button>
      </div>

      {/* Notification */}
      {notification && (
        <div className="fixed top-4 right-4 bg-status-success text-bg px-4 py-2 rounded-lg shadow-lg animate-fade-in">
          {notification}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Templates List */}
        <div className="space-y-4">
          {templates.map((template) => (
            <div
              key={template.id}
              onClick={() => setSelectedTemplate(template.id)}
              className={`p-4 border rounded-lg cursor-pointer transition-colors ${
                selectedTemplate === template.id
                  ? 'border-accent bg-accent/5'
                  : 'border-border hover:border-accent/50'
              }`}
            >
              {editing === template.id ? (
                <div className="space-y-3">
                  <input
                    type="text"
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    className="w-full px-3 py-2 bg-bg border border-border rounded text-text font-medium"
                    placeholder="Nom du template"
                  />
                  <input
                    type="text"
                    value={editDescription}
                    onChange={(e) => setEditDescription(e.target.value)}
                    className="w-full px-3 py-2 bg-bg border border-border rounded text-sm text-text-secondary"
                    placeholder="Description"
                  />
                  <textarea
                    value={editContent}
                    onChange={(e) => setEditContent(e.target.value)}
                    rows={6}
                    className="w-full px-3 py-2 bg-bg border border-border rounded text-sm text-text font-mono"
                    placeholder="Contenu du template..."
                  />
                  <div className="flex gap-2">
                    <button
                      onClick={handleSaveEdit}
                      className="flex items-center gap-1 px-3 py-1.5 bg-accent text-bg rounded text-sm"
                    >
                      <Check size={14} />
                      Sauvegarder
                    </button>
                    <button
                      onClick={handleCancelEdit}
                      className="flex items-center gap-1 px-3 py-1.5 border border-border text-text rounded text-sm"
                    >
                      <X size={14} />
                      Annuler
                    </button>
                  </div>
                </div>
              ) : (
                <div className="flex items-start justify-between">
                  <div className="flex items-start gap-3">
                    <FileText size={20} className="text-accent mt-0.5" />
                    <div>
                      <h3 className="font-medium text-text">{template.name}</h3>
                      <p className="text-sm text-text-secondary">{template.description}</p>
                      <div className="flex gap-2 mt-2">
                        {defaultTemplateId === template.id && (
                          <span className="text-2xs bg-accent text-bg px-2 py-0.5 rounded font-medium">
                            Défaut actuel
                          </span>
                        )}
                        <span className="text-2xs text-text-tertiary">
                          {template.content.length} caractères
                        </span>
                      </div>
                    </div>
                  </div>
                  <div className="flex gap-1">
                    {defaultTemplateId !== template.id && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleSetDefault(template.id);
                        }}
                        className="p-1.5 text-accent hover:bg-accent/10 rounded"
                        title="Définir comme défaut"
                      >
                        <Check size={16} />
                      </button>
                    )}
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDuplicate(template);
                      }}
                      className="p-1.5 text-text-tertiary hover:text-text hover:bg-bg rounded"
                      title="Dupliquer"
                    >
                      <Copy size={16} />
                    </button>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleStartEdit(template);
                      }}
                      className="p-1.5 text-text-tertiary hover:text-text hover:bg-bg rounded"
                      title="Modifier"
                    >
                      <Edit2 size={16} />
                    </button>
                    {defaultTemplateId !== template.id && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDelete(template.id);
                        }}
                        className="p-1.5 text-status-error hover:bg-status-error/10 rounded"
                        title="Supprimer"
                      >
                        <Trash2 size={16} />
                      </button>
                    )}
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>

        {/* Preview Panel */}
        <div className="space-y-4">
          <div className="bg-bg-secondary border border-border rounded-lg p-4">
            <h3 className="font-medium text-text mb-4 flex items-center gap-2">
              <Sparkles size={18} className="text-accent" />
              Aperçu
            </h3>
            
            <div className="space-y-3 mb-4">
              <div>
                <label className="text-xs text-text-tertiary uppercase">Question exemple</label>
                <input
                  type="text"
                  value={previewQuery}
                  onChange={(e) => setPreviewQuery(e.target.value)}
                  className="w-full px-3 py-2 bg-bg border border-border rounded text-sm text-text mt-1"
                />
              </div>
              <div>
                <label className="text-xs text-text-tertiary uppercase">Contexte exemple</label>
                <textarea
                  value={previewContext}
                  onChange={(e) => setPreviewContext(e.target.value)}
                  rows={3}
                  className="w-full px-3 py-2 bg-bg border border-border rounded text-sm text-text mt-1"
                />
              </div>
            </div>

            <div className="bg-bg rounded-lg p-4">
              <div className="text-xs text-text-tertiary uppercase mb-2">Résultat</div>
              <pre className="text-sm text-text whitespace-pre-wrap font-mono">
                {currentTemplate ? getPreview(currentTemplate) : 'Sélectionnez un template'}
              </pre>
            </div>
          </div>

          <div className="bg-accent/5 border border-accent/20 rounded-lg p-4">
            <h4 className="font-medium text-text mb-2">Variables disponibles</h4>
            <div className="space-y-1 text-sm">
              <code className="text-accent bg-accent/10 px-1.5 py-0.5 rounded">{'{{query}}'}</code>
              <span className="text-text-secondary"> - La question de l'utilisateur</span>
            </div>
            <div className="space-y-1 text-sm mt-1">
              <code className="text-accent bg-accent/10 px-1.5 py-0.5 rounded">{'{{context}}'}</code>
              <span className="text-text-secondary"> - Le contenu trouvé dans les documents</span>
            </div>
          </div>
        </div>
      </div>

      {/* New Template Modal */}
      {showNewForm && (
        <div className="fixed inset-0 bg-bg/80 backdrop-blur-sm flex items-center justify-center z-50">
          <div className="bg-bg-secondary border border-border rounded-xl w-full max-w-lg mx-4 p-6">
            <h3 className="text-lg font-semibold text-text mb-4">Nouveau Template</h3>
            <div className="space-y-4">
              <input
                type="text"
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                placeholder="Nom du template"
                className="w-full px-4 py-2 bg-bg border border-border rounded-lg text-text"
              />
              <input
                type="text"
                value={editDescription}
                onChange={(e) => setEditDescription(e.target.value)}
                placeholder="Description"
                className="w-full px-4 py-2 bg-bg border border-border rounded-lg text-text"
              />
              <textarea
                value={editContent}
                onChange={(e) => setEditContent(e.target.value)}
                rows={8}
                placeholder="Contenu du template...&#10;&#10;Exemple:&#10;Basé sur:&#10;{{context}}&#10;&#10;Question: {{query}}"
                className="w-full px-4 py-2 bg-bg border border-border rounded-lg text-text font-mono"
              />
              <div className="flex justify-end gap-2">
                <button
                  onClick={() => setShowNewForm(false)}
                  className="px-4 py-2 text-text-secondary hover:text-text"
                >
                  Annuler
                </button>
                <button
                  onClick={handleCreate}
                  className="px-4 py-2 bg-accent text-bg rounded-lg hover:bg-accent-hover"
                >
                  Créer
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
