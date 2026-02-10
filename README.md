# Lorag — Local RAG + MCP Engine

<p align="center">
  <h1 align="center">🧠 Lorag</h1>
  <p align="center"><strong>La mémoire locale universelle pour agents IA</strong></p>
  <p align="center">
    <a href="#installation">Installation</a> •
    <a href="#démarrage-rapide">Démarrage Rapide</a> •
    <a href="#utilisation">Utilisation</a> •
    <a href="#documentation">Documentation</a>
  </p>
</p>

---

## 🎯 Qu'est-ce que Lorag ?

**Lorag** est un moteur RAG (Retrieval-Augmented Generation) 100% local qui permet à n'importe quelle IA (Claude, GPT, etc.) d'accéder à vos documents personnels de manière sécurisée et privée.

### ✨ Fonctionnalités clés

- 🔒 **100% Local & Privé** - Vos données ne quittent jamais votre machine
- 🤖 **Compatible MCP** - Fonctionne avec Claude, ChatGPT, VS Code, et plus
- 📄 **Multi-format** - PDF, Word, Markdown, code source (20+ formats)
- 🔄 **Indexation automatique** - Surveillance en temps réel de vos dossiers
- ⏰ **Planification** - Indexation automatique programmée
- 📝 **Templates de prompts** - Personnalisez les réponses de vos IA
- ⚡ **Ultra rapide** - Recherche sémantique en < 50ms
- 🌐 **UI Web moderne** - Interface React intuitive et accessible

## 📦 Installation

### Option 1 : Application Desktop (Recommandé)

**macOS :**
```bash
curl -fsSL https://lorag.dev/install-macos.sh | bash
```

**Windows :**
```powershell
# Téléchargez l'installeur
https://github.com/lorag/lorag/releases/latest/download/lorag-windows.exe
```

**Linux :**
```bash
curl -fsSL https://lorag.dev/install.sh | bash
```

### Option 2 : Depuis les sources

```bash
git clone https://github.com/lorag/lorag.git
cd lorag
make install
make build
```

## 🚀 Démarrage Rapide

### 1. Lancer Lorag

```bash
# Méthode simple
./scripts/launch.sh

# Ou avec Make
make launch

# Ou directement
lorag server
```

L'interface s'ouvre automatiquement à `http://localhost:8080`

### 2. Assistant de configuration

Au premier démarrage, suivez l'assistant :

1. **Choisissez l'emplacement des données**
2. **Ajoutez vos dossiers** à indexer
3. **Configurez les embeddings** (Ollama recommandé)
4. **Laissez indexer** - Ça prend quelques minutes

### 3. Commencer à utiliser

- 🔍 **Recherchez** dans vos documents via l'onglet "Recherche"
- 📁 **Ajoutez des dossiers** dans "Sources"
- ⏰ **Planifiez** l'indexation automatique
- 🤖 **Connectez vos IA** via l'endpoint MCP

## 📖 Utilisation

### Pour les utilisateurs non-techniques

Lorag s'utilise principalement via son **interface web** :

1. Ouvrez `http://localhost:8080` dans votre navigateur
2. Utilisez le **Wizard** pour la configuration initiale
3. Explorez les différentes sections :
   - **Tableau de bord** - Statistiques et aperçu
   - **Recherche** - Recherche sémantique naturelle
   - **Sources** - Gestion des dossiers surveillés
   - **Planification** - Tâches d'indexation automatiques
   - **Templates** - Personnalisation des prompts

### Pour les utilisateurs avancés (CLI)

```bash
# Indexer un dossier
loragctl index /path/to/documents --recursive

# Rechercher
loragctl search "comment configurer un serveur"

# Voir le statut
loragctl status

# Gérer les tâches planifiées
loragctl schedule add --path /docs --cron "0 2 * * *"
```

### Connexion avec les IA

**Claude (MCP) :**
```
Paramètres > Extensions > MCP
URL: http://localhost:8080/mcp
```

**API REST :**
```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query": "votre question", "top_k": 5}'
```

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        CLIENTS                              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │  Claude  │ │ ChatGPT  │ │ VS Code  │ │   CLI    │       │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘       │
└───────┼────────────┼────────────┼────────────┼─────────────┘
        │            │            │            │
        └────────────┴──────┬─────┴────────────┘
                            │
                    ┌───────▼───────┐
                    │   MCP/REST    │
                    │    API        │
                    └───────┬───────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
   ┌────▼────┐       ┌─────▼─────┐      ┌──────▼──────┐
   │ Watcher │       │  Indexer  │      │    Query    │
   │(fsnotify│       │ +Chunker  │      │   Engine    │
   └────┬────┘       └─────┬─────┘      └──────┬──────┘
        │                  │                   │
        └──────────────────┼───────────────────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
        ┌─────▼────┐ ┌────▼────┐ ┌─────▼─────┐
        │ BoltDB   │ │  HNSW   │ │  Parsers  │
        │(Metadata)│ │(Vectors)│ │(PDF/DOCX) │
        └──────────┘ └─────────┘ └───────────┘
```

## 📚 Documentation

- **[Guide Utilisateur](USER_GUIDE.md)** - Documentation complète pour les utilisateurs
- **[Démarrage Rapide](QUICKSTART.md)** - Premiers pas en 3 minutes
- **[API Reference](docs/API.md)** - Documentation de l'API REST
- **[MCP Protocol](docs/MCP.md)** - Guide du protocole MCP
- **[Contributing](CONTRIBUTING.md)** - Guide de contribution

## 🛠️ Technologies

| Composant | Technologie |
|-----------|-------------|
| Backend | Go 1.22 |
| API | Chi Router + CORS |
| Vector DB | HNSW (implémentation Go) |
| Metadata | BoltDB |
| Frontend | React 18 + TypeScript |
| UI | TailwindCSS |
| Build | Vite |
| Embeddings | Ollama / OpenAI |

## 🗺️ Roadmap

- [x] Core RAG avec HNSW
- [x] UI Web React
- [x] Support MCP
- [x] Planification d'indexation
- [x] Templates de prompts
- [ ] OCR pour images et PDF scannés
- [ ] Synchronisation chiffrée multi-appareils
- [ ] Plugins et extensions
- [ ] Mode collaboratif (Enterprise)

## 🤝 Contribution

Les contributions sont les bienvenues ! Voir [CONTRIBUTING.md](CONTRIBUTING.md)

```bash
# Fork et clone
git clone https://github.com/votre-username/lorag.git

# Installer les déps
make install

# Lancer en dev
make dev-server  # Terminal 1
make dev-ui      # Terminal 2
```

## 📄 Licence

Lorag est sous licence MIT. Voir [LICENSE](LICENSE)

---

<p align="center">
  <strong>Fait avec ❤️ pour une IA plus utile et privée</strong><br>
  <a href="https://github.com/lorag/lorag">GitHub</a> •
  <a href="https://discord.gg/lorag">Discord</a> •
  <a href="https://twitter.com/lorag">Twitter</a>
</p>
