# Démarrage rapide

Dix minutes, depuis les sources. Il n'y a pas encore d'installeur.

## 1. Prérequis

```sh
# macOS
brew install go node ollama
brew install poppler ffmpeg whisper.cpp tesseract   # PDF par page, audio, vidéo, OCR (inclus avec la formule Homebrew de Lorag)
ollama serve &                                       # si Ollama ne tourne pas déjà
ollama pull bge-m3          # modèle d'embedding multilingue (français, anglais, arabe…), 1,2 Go
```

## 2. Compiler

```sh
git clone https://github.com/raucheacho/lorag
cd lorag
make build
```

`make build` télécharge le moteur VectixDB (précompilé), compile le Studio, puis `bin/lorag` et `bin/loragctl`.

## 3. Lancer

```sh
./bin/lorag server
```

Ouvrez http://localhost:8080 (ou `lorag open`).

- Les données vont dans `~/.lorag/data`. Pour un autre dossier : `./bin/lorag server -d ~/lorag-data`.
- Pour un autre port : `-p 8090`.

## 4. Indexer un dossier

Sur **Overview**, le guide de démarrage vérifie le modèle d'embedding, puis demande un dossier.
- **Index** lance l'indexation.
- *Watch* réindexe ensuite les fichiers modifiés.

La page **Ingestion** montre l'avancement et les éventuelles erreurs. Vous pouvez aussi y glisser des fichiers.

## 5. Chercher

Dans **Playground**, tapez une question puis `⌘↵`. Chaque résultat indique :
- son fichier, et sa page, ses lignes ou sa minute ;
- son rang dans la recherche par le sens (violet) et dans la recherche par les mots (orange).

*Compare routes* montre ce que chaque recherche trouve seule.

## 6. Connecter un agent

Dans **Connect**, choisissez votre client (client HTTP générique, Cursor, VS Code, passerelle stdio), copiez la configuration et cliquez sur **Test**. L'URL est `http://localhost:8080/mcp`.

L'agent reçoit sept outils, dont :
- `memory_recall` : chercher dans ce qui est déjà connu avant de chercher ailleurs ;
- `memory_save` : garder un correctif ou une réponse utile ;
- `search` : chercher dans vos fichiers.

## 7. Suivre

- **Agent logs** : chaque recherche et chaque rappel de vos agents, en direct.
- **Memory** : ce qu'ils ont appris, ce qui a servi, les tokens économisés.

## Problèmes courants

| Symptôme | Cause et solution |
| --- | --- |
| `address already in use` au lancement | Le port 8080 est pris : `./bin/lorag server -p 8090` |
| *Embedder unavailable* dans la barre latérale | Ollama ne tourne pas, ou le modèle manque : `ollama serve`, `ollama pull bge-m3` |
| `database locked by another process` | Un autre `lorag server` utilise le même dossier de données |
| Un PDF n'a pas de numéro de page | `pdftotext` n'est pas installé |
| Un audio ou une vidéo échoue avec `TRANSCRIPTION_DISABLED` | La transcription a été désactivée : Settings → Images and audio → Audio transcription, puis redémarrer |
| Aucun résultat | Vérifiez dans Explorer que le fichier est indexé ; pour un identifiant exact, essayez le mode *Exact* |
| `max chunks limit reached (5000)` | Sans clé de licence, l'index est limité à 5 000 chunks (voir [docs/STATUS.md](docs/STATUS.md)) |

La suite : [USER_GUIDE.md](USER_GUIDE.md).
