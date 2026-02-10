# Nom du Produit

Lorag — Local RAG + MCP Engine

Tagline : “La mémoire locale universelle pour agents IA”

---

## 🚀

## Vision du Produit

Permettre à n’importe quelle IA ou agent (Claude, GPT, open-source, voice agents, IDE, etc.) de :

- indexer les données locales de l’utilisateur,
- interroger ces données avec un moteur RAG hyper performant,
- tout en restant 100% local, private by default,
- fournir une UI web locale pour visualiser, administrer et suivre l’état des indexations.

---

# 📦

# Architecture Globale

```
                             +------------------+
                             |   Clients/Apps   |
                             |------------------|
                             | Agent IA         |
                             | IDE (VSCode)     |
                             | Voice Assistants |
                             | CLI              |
                             +------------------+
                                       |
                                       |   MCP (Model Context Protocol)
                                       |
                             +------------------+
                             |  API Gateway     |  (http/grpc)
                             +------------------+
                                       |
                             +------------------+
                             |    lorag Core  |    << RAG + Vector + Watcher
                             | (Go Binary)      |
                             +------------------+
                                       |
         +----------------------+------+-------------------+
         |                      |                       |
 +---------------+    +--------------------+     +----------------+
 | File Watcher  |    | Doc Parsers/OCR     |     | Embedding     |
 | (fsnotify)    |    | PDF/DOCX/MD/TXT     |     | Provider      |
 +---------------+    +--------------------+     +----------------+
         |                      |                       |
         +-------------+--------------------+------------+
                       |
                +--------------------+
                | Vector DB Engine   |
                | (hnswlib / custom) |
                +--------------------+
                       |
                +-----------------------+
                | Storage (Local)       |
                | (BoltDB)              |
                +-----------------------+
                       |
                +-----------------------+
                | Local UI (Web UI)     |
                +-----------------------+
```

---

# 🛠️

# Stack Technologique

Partie

Tech

Pourquoi

Core Engine

Go

Performance, déploiement binaire, concurreny

API

Go + chi

Léger, rapide, facile à exposer HTTP/REST

MCP Protocol

JSON/HTTP + gRPC

Standard ouvert, compatible agents

Vector search

hnswlib-go

Pour embeddings rapides

Local storage

BoltDB

Persist local, ultra stable

Embeddings

Ollama / ggml / ONNX

Option local y compris offline

OCR

Tesseract (CGO)

Détection texte images locales

Parsers docs

unidoc

PDF, docx, md

UI

Go serve + React + Vite

UI locale rapide

UI design system

TailwindCSS

Léger, moderne

CLI

Cobra

UX ligne de commande

Packaging

Go Releaser

Binaires multiplateformes

---

# 📁

# Arborescence du Projet (RAG Engine)

```
lorag/
├── cmd/
│   ├── lorag/           # serveur principal
│   └── loragctl/        # CLI
│
├── internal/            # tout le code privé
│   ├── api/
│   │   ├── rest/
│   │   └── mcp/
│   ├── core/
│   │   ├── indexer/
│   │   ├── query/
│   │   ├── vector/
│   │   └── store/
│   ├── watcher/
│   ├── parsers/
│   ├── embeddings/
│   ├── config/
│   └── server/
│
├── web/                 # React UI source
├── ui/                  # embedded assets
├── docs/
├── scripts/
└── examples/                  # UI code (React/Tailwind)
```

---

# 🔑

# Features Complètes

## 🧠 Core / RAG Capabilities

### 🚀 Indexation

- Indexation multi-format :
  - PDF
  - DOCX
  - Markdown
  - Texte
  - Code (.js .go .py …)
  - Images (OCR)
- Détection automatique de nouveaux documents (watcher)
- Chunking intelligent par contexte
- Indexation incrémentale / delta
- Scheduling d’indexation

### 📊 Vector Search

- Index vectoriel local (HNSW)
- Metrics configurables (cosine, dot)
- Fast search / multi-threaded
- Persistence sur disque

### 🧠 Query Engine

- API REST / MCP
- Contexte + embeddings
- Prompt templates (configurable)
- Pagination de résultats
- Résultats avec trace (log)

---

## 🤝 MCP Support (clé stratégique)

- Endpoint MCP standard
- Support JSON + gRPC
- Documentation / SDK
- Support plug & play avec :
  - ChatGPT plugins
  - Agents open
  - Claude / GPT via proxy
  - VSCode extensions

---

## 🧑‍💻 UI Web Locale

UI inspirée de Drizzle/RedisInsight :

### Dashboard

- Statistiques d’indexation
- Taille de l’index
- Activité récente
- Logs d’erreur

### Search Console

- Barre de recherche
- Résultats vectoriels
- Affichage contextuel
- Filtres & tags

### Sources

- Voir les fichiers indexés
- Voir les metadata
- Réindexer / supprimer

### Settings

- API Keys (local)
- Embeddings config
- Watch folders
- Permissions locales

---

## 📦 CLI local

```
loragctl index /path/to/docs
loragctl query "ma question"
loragctl status
loragctl watch add /path
```

---

# 🔐 Sécurité & Local-First

- 100% local par défaut
- Chiffrement optionnel (AES-GCM)
- ACL locales
- Pas de cloud par défaut
- Consentement explicite pour synchronisation externe

---

# 📈 Modèle de Prix (B2B SaaS / Licence)

Free

- 1 dossier indexé
- 5k chunks max
- UI Web locale
- API MCP limité

### 💼

### Pro — $99/utilisateur/mois

- multi-dossiers
- 100k chunks
- OCR
- Logs / traces
- API MCP illimité
- Support email

### 🏢

### Enterprise — Sur devis

- multi-serveurs
- découpes RBAC
- audit & compliance
- support SLA
- intégrations personnalisées

---

## 👉 Pourquoi c’est super vendeable

✔ Résout une vraie douleur (local-first RAG)

✔ Privé & compliant (RGPD)

✔ Standard ouvert (MCP) = adoption rapide

✔ UI moderne = facile onboarding

✔ B2B = argent réel dès le début

✔ Fixe une barrière d’entrée tech durable

✔ Language bas niveau = performance et fiabilité
