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
- 📝 **Templates de prompts** - Personnalisez les réponses de vos IA
- ⚡ **Ultra rapide** - Recherche sémantique en < 50ms
- 🧪 **Studio** - Explorer, playground avec décomposition du score, logs des agents

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

- 📁 **Ajoutez des dossiers** dans "Ingestion"
- 🔍 **Testez vos requêtes** dans "Playground"
- 🤖 **Connectez vos agents** depuis "Connect" (endpoint MCP)
- 📈 **Suivez ce qu'ils demandent** dans "Agent logs"

## 📖 Utilisation

### Pour les utilisateurs non-techniques

Lorag s'utilise via son **Studio**, servi par le démon sur `http://localhost:8080` (thème clair ou sombre, palette de commandes `⌘K`) :

- **Overview** - état du moteur et de l'embedder, volumes, dernières requêtes et indexations ; guide de démarrage quand l'index est vide
- **Explorer** - documents → chunks, avec le texte exact de chaque chunk et sa présence dans l'index
- **Playground** - une requête comme un agent la ferait : rang vectoriel, rang BM25 et score fusionné de chaque résultat, comparaison des trois routes, export en `curl` ou en appel MCP
- **Agent logs** - chaque recherche servie, par client MCP : question, résultats renvoyés, latence, réponses vides
- **Ingestion** - indexer un fichier ou un dossier, dossiers surveillés, file d'attente, erreurs
- **Connect** - URL MCP, configuration par client, test de connexion, outils exposés
- **Settings** - embeddings, paramètres de l'index VectixDB, découpage, médias, maintenance

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
        ┌─────▼────┐ ┌────▼─────┐ ┌─────▼─────┐
        │ BoltDB   │ │ VectixDB │ │  Parsers  │
        │(Metadata)│ │ (Search) │ │(PDF/DOCX) │
        └──────────┘ └──────────┘ └───────────┘
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
| Backend | Go 1.24 |
| API | Chi Router + CORS |
| Recherche | [VectixDB](../vectixdb) (Zig, lié par cgo) : HNSW, BM25, filtres, fusion hybride, WAL |
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
