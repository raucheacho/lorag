# Lorag

**La mémoire locale des agents IA.** Lorag indexe vos fichiers (documents, code, PDF, présentations, audio, vidéo) et garde ce que vos agents ont déjà trouvé ou appris : un correctif à une erreur, une réponse cherchée sur le web, ce que vous leur avez dit de vous. Un agent les retrouve en quelques tokens au lieu de chercher ou de redemander. Tout reste sur votre machine.

- **Mémoire des agents.** Correctifs et réponses déjà trouvés, faits sur vous et vos proches (une personne, une préférence, un engagement), avec leur source et leur historique, rappelés en 20 à 60 tokens ([docs/MEMORY.md](docs/MEMORY.md)). Pour le code comme pour un assistant vocal comme Benom.
- **Recherche hybride.** Le sens (embeddings) et les mots exacts (BM25) fusionnés. Les messages d'erreur sont cherchés mot pour mot.
- **Positions exactes.** Chaque résultat renvoie à la page d'un PDF, aux lignes d'un fichier ou à la minute d'un audio ou d'une vidéo ([docs/MEDIA_INDEXING.md](docs/MEDIA_INDEXING.md)).
- **MCP.** Cursor, VS Code, Codex et tout client MCP se branchent sur la même mémoire ([docs/MCP.md](docs/MCP.md)).
- **Studio.** Une interface web pour voir ce que les agents demandent, tester une requête et comprendre pourquoi un résultat sort, et corriger la mémoire.
- **Local.** Embeddings par Ollama, transcription par whisper.cpp, OCR par tesseract. Aucune donnée ne sort sans votre action.
- **Fiable.** Le moteur de recherche, [VectixDB](https://github.com/raucheacho/vectixdb), est écrit en Zig, résiste aux plantages et est lié dans le démon : un seul binaire.

Statut : en développement actif, pas encore de version publiée. Voir [docs/STATUS.md](docs/STATUS.md).

## Installation

Lorag est un seul binaire pour macOS et Linux (amd64, arm64) : le Studio et le moteur sont dedans. Il lui faut [Ollama](https://ollama.com) pour les embeddings. À partir de la première version publiée :

```sh
# macOS (et Linux) avec Homebrew : Lorag tourne en fond avec la session
brew install raucheacho/tap/lorag
brew services start lorag

# Linux : binaires dans ~/.local/bin, --service pour un service systemd utilisateur
curl -fsSL https://raw.githubusercontent.com/raucheacho/lorag/main/scripts/install.sh | sh -s -- --service

ollama pull bge-m3
```

Studio sur http://localhost:8080, données dans `~/.lorag/data`.

## Installation depuis les sources

Prérequis :
- Go 1.24 ;
- Node.js 20 ;
- [Ollama](https://ollama.com).

```sh
git clone https://github.com/raucheacho/lorag
cd lorag
ollama pull bge-m3   # modèle d'embedding multilingue (1,2 Go)
make build            # Studio, bin/lorag et bin/loragctl
./bin/lorag server    # Studio sur http://localhost:8080
```

Avec Homebrew, les outils pour les PDF, l'audio, la vidéo et les images sont installés avec Lorag ; sur Linux : [docs/MEDIA_INDEXING.md](docs/MEDIA_INDEXING.md). Docker et compilation détaillée : [BUILD_INSTRUCTIONS.md](BUILD_INSTRUCTIONS.md).

## Démarrage

1. Ouvrez http://localhost:8080. La page **Overview** vérifie le modèle d'embedding et propose d'indexer un premier dossier.
2. **Ingestion** : indiquez un dossier ou glissez des fichiers. Cochez *Keep watching* pour réindexer les fichiers modifiés.
3. **Connect** : copiez la configuration MCP de votre client (Cursor, VS Code, client HTTP) et testez la connexion.
4. Travaillez avec votre agent. **Agent logs** montre ses recherches et ses rappels, **Memory** ce qu'il a appris.

Guide pas à pas : [QUICKSTART.md](QUICKSTART.md). Guide complet : [USER_GUIDE.md](USER_GUIDE.md).

## Le Studio

| Page | |
| --- | --- |
| Overview | état du moteur et de l'embedder, volumes, dernières requêtes et indexations, mémoire |
| Memory | tendances (rappels, tokens économisés), entrées avec historique, chronologie, santé, testeur de rappel |
| Explorer | documents, chunks avec leur position, ouverture de la source à la bonne page ou minute |
| Playground | une requête comme un agent la ferait, avec la part du sens et des mots dans le score, comparaison des routes, export `curl` ou MCP |
| Agent logs | chaque recherche et chaque rappel, par client : question, résultats, latence |
| Ingestion | indexer, glisser-déposer, dossiers surveillés, file d'attente, erreurs |
| Connect | URL MCP, configuration par client, test, outils exposés |
| Settings | embeddings, index, découpage, audio et images, maintenance |

Palette de commandes `⌘K`, thème clair ou sombre.

## Ligne de commande

```sh
loragctl index ~/notes --recursive
loragctl search "procédure de rollback"
loragctl mem recall "Error: listen EADDRINUSE: address already in use :::3000"
npm run dev 2>&1 | loragctl mem recall -
loragctl mem save --kind knowledge --problem "taille max d'upload par défaut de nginx" \
  --answer "1 Mo (client_max_body_size 1m)" --source https://nginx.org/en/docs/http/ngx_http_core_module.html
loragctl mem stats
loragctl status
```

## API

- **MCP :** `POST http://localhost:8080/mcp`. Outils : `memory_recall`, `memory_save`, `memory_feedback`, `search`, `get_document`, `ingest`, `get_stats` ([docs/MCP.md](docs/MCP.md)).
- **REST :** `http://localhost:8080/api/v1` ([docs/API.md](docs/API.md)).

```sh
curl -s localhost:8080/api/v1/search -H 'Content-Type: application/json' \
  -d '{"query": "procédure de rollback", "top_k": 5}'
```

## Architecture

```
agents MCP · loragctl · Studio
          │
   lorag server (Go) ── parsers, indexeur, mémoire, MCP, REST, Studio embarqué
          │ cgo
   VectixDB (Zig) ── index vectoriel, BM25, filtres, journal d'écriture
          │
   <data>/lorag.db (documents)   <data>/vectix/ (index et mémoire)
```

Détails : [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Documentation

Tout est listé dans [docs/README.md](docs/README.md). Pour contribuer : [CONTRIBUTING.md](CONTRIBUTING.md) et [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Licence

MIT, voir [LICENSE](LICENSE). Le moteur VectixDB est sous licence Apache-2.0.
