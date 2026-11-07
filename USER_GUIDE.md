# Guide utilisateur

Lorag est la mémoire locale de vos agents IA. Il fait deux choses :

- **Il indexe vos fichiers** (documents, code, PDF, présentations, tableurs, audio, vidéo, images) pour que vos agents y cherchent par le sens ou par les mots exacts. Chaque résultat indique la page, les lignes ou la minute.
- **Il garde ce que vos agents ont déjà trouvé** :
  - un **correctif** à un problème qui revient ;
  - une **réponse** cherchée sur le web, avec sa source.

  La fois suivante, l'agent la retrouve en quelques dizaines de tokens au lieu de recommencer.

Tout tourne sur votre machine : embeddings par Ollama, transcription par whisper.cpp, lecture de texte par tesseract.

Installation : [QUICKSTART.md](QUICKSTART.md).

## Le Studio

Ouvrez http://localhost:8080, ou lancez `lorag open` si vous avez oublié l'adresse.

- **Raccourcis :** `⌘K` (ou `Ctrl+K`) ouvre la palette de commandes, qui sert à changer de page, lancer une recherche ou une action ; `/` place le curseur dans le champ de recherche de la page.
- **Thème :** le sélecteur est en bas de la barre latérale (système, clair, sombre).
- **État :** la barre latérale indique celui du moteur, du modèle d'embedding et du serveur.

### Overview

Volumes (documents, chunks, taille de l'index), latence des recherches, dernières requêtes des agents, dernières indexations, état de l'index et de la mémoire. Tant que rien n'est indexé, un guide en trois étapes vérifie le modèle d'embedding, indexe un premier dossier et mène à la connexion d'un agent.

### Ingestion

- **Un chemin.** Indiquez un dossier ou un fichier présent sur la machine qui fait tourner Lorag.
  - *Subfolders* inclut les sous-dossiers.
  - *Keep watching* réindexe automatiquement les fichiers modifiés.
- **Glisser-déposer.** Les fichiers sont copiés dans le dossier de données (`imports/`), puis indexés.
- **Suivi.** La file d'attente, les fichiers en cours et le journal (indexé, inchangé, en échec avec la raison).

Un fichier inchangé n'est pas retraité. Un fichier modifié remplace d'un coup son ancienne version : une recherche ne voit jamais un fichier à moitié indexé.

Types de fichiers et outils nécessaires : [docs/MEDIA_INDEXING.md](docs/MEDIA_INDEXING.md).

### Explorer

La liste des documents (filtre par nom, type, tri). Un clic ouvre le détail :
- **Métadonnées** du document.
- **Chunks** : chacun avec sa position (page, lignes, minute), sa section, et le badge *indexed* s'il est bien dans l'index de recherche.
- **Source** :
  - *Open original file* ouvre le fichier ;
  - pour un PDF, le lien ouvre la bonne page ;
  - pour un audio ou une vidéo, *Play* lit le passage.
- **Actions** : réindexer, supprimer, chercher dans ce seul fichier.

### Playground

Une requête exactement comme un agent la ferait.

- **Modes.**

  | Mode | Cherche |
  | --- | --- |
  | Hybrid (par défaut) | le sens et les mots |
  | Vector | le sens seul |
  | Keyword | les mots seuls |
  | Exact | le texte mot pour mot, avec toutes les occurrences : utile pour un message d'erreur ou un identifiant |

- **Options.**
  - `top_k` : le nombre de résultats ;
  - `context` : ajoute les chunks voisins ;
  - `type` et `path` : filtres.
- **Résultats.** Pour chaque résultat, le rang par le sens (violet), le rang par les mots (orange) et le score fusionné. Un clic explique le calcul et montre le texte exact renvoyé à l'agent.
- **Compare routes.** Lance les trois recherches côte à côte et marque ce qu'une seule trouve.
- **Copy as.** Donne la requête en `curl` ou en appel MCP.

### Agent logs

Chaque recherche et chaque rappel de mémoire, avec :
- la source (MCP, API, Studio) et le nom du client MCP (Cursor, Codex…) ;
- la question, le nombre de résultats et la latence.

Les réponses vides sont signalées : souvent un document manque, ou la question demandait le mode exact. *Replay* rejoue une requête dans le Playground.

Ce journal garde les 1 000 dernières requêtes depuis le démarrage.

### Memory

Ce que vos agents (et vous) savent déjà. Deux types d'entrées :

| Type | Contient | Exemple |
| --- | --- | --- |
| **Fix** | un problème qui revient, et ce qui l'a résolu | « port 3000 déjà utilisé » → `lsof -ti :3000 \| xargs kill -9` |
| **Knowledge** | une question et sa réponse vérifiée, avec la source | « taille max d'upload de nginx » → « 1 Mo, `client_max_body_size` », nginx.org |

Une entrée est une information, pas une tâche : l'agent ne s'en sert que si elle correspond à sa situation.

Les onglets :
- **Trends** : rappels réussis et ratés par jour, problèmes les plus fréquents, entrées les plus utiles, tokens économisés (estimation expliquée sur la page).
- **Entries** : filtres par type, statut et espace, et recherche.
  - Le panneau de détail montre le correctif ou la réponse, l'erreur d'origine, la source, les versions, les compteurs et tout l'historique.
  - Vous pouvez corriger une entrée (nouvelle révision, l'ancienne reste dans l'historique), changer son statut, indiquer si elle était juste, ou la supprimer.
- **Timeline** : ce qui a été appris, utilisé, jugé faux ou oublié, jour par jour.
- **Health** :
  - entrées à revérifier ;
  - entrées obsolètes ;
  - entrées non vérifiées depuis six mois ;
  - doublons probables.

Deux outils en haut de la page :
- **Test a recall** montre exactement ce que l'agent recevrait pour une erreur ou une question, et combien de tokens cela coûte.
- **New entry** ajoute une entrée à la main ; elle part avec plus de confiance qu'une entrée d'agent.

Statuts d'une entrée :
- **valid** : la valeur de départ ;
- **recheck** : elle a été jugée fausse une fois, ou elle n'a pas été vérifiée depuis six mois, ou l'agent utilise une autre version majeure que celle de l'entrée (« expo 51≠52 ») ;
- **obsolete** : jugée fausse plus souvent que juste. Elle n'est plus rappelée.

Les espaces : une mémoire **globale** et une par **projet**. Un agent qui travaille sur un projet voit les entrées de ce projet et les entrées globales.

Détails techniques : [docs/MEMORY.md](docs/MEMORY.md).

### Connect

- **Connexion.** L'URL MCP, la configuration à copier pour chaque client et un bouton **Test** qui vérifie la connexion.
- **Outils.** La liste de ce que l'agent peut faire, lue sur le serveur.

### Settings

| Groupe | Contenu | Prise en compte |
| --- | --- | --- |
| Embeddings | fournisseur, modèle, URL, dimension ; *Test embedder* ; *Use bge-m3* quand un autre modèle Ollama est choisi | au redémarrage. Un nouveau modèle ou une nouvelle dimension reconstruit l'index et recalcule la mémoire automatiquement |
| Search index | distance, vecteurs int8, paramètres HNSW et BM25 | au redémarrage, puis reconstruction de l'index en arrière-plan ; les recherches continuent sur l'ancien index pendant ce temps |
| Chunking and indexing | taille des chunks, chevauchement, travailleurs, limite | au redémarrage, pour les fichiers indexés ensuite |
| MCP | présentation des résultats de recherche aux agents | immédiatement |
| Memory | seuils de similarité du rappel (0 = valeurs calibrées pour le modèle) | au redémarrage |
| Images and audio | description des images, transcription, modèle Whisper (*Download now* le télécharge et affiche la progression ; sinon il est téléchargé au premier fichier audio) | au redémarrage |
| Maintenance | checkpoint, compactage, reconstruction de l'index | immédiatement |

Une barre en bas de page signale les changements non enregistrés, et prévient quand un changement entraînera une reconstruction de l'index.

## Connecter vos agents

Le serveur MCP est sur `http://localhost:8080/mcp` (Streamable HTTP). La page Connect donne la configuration de chaque client, par exemple :

```json
{ "mcpServers": { "lorag": { "type": "http", "url": "http://localhost:8080/mcp" } } }
```

Pour un client qui ne sait lancer que des programmes locaux (stdio), passez par la passerelle `npx -y mcp-remote http://localhost:8080/mcp`.

Les outils et leur usage sont décrits dans [docs/MCP.md](docs/MCP.md). À la connexion, l'agent reçoit aussi ses consignes :
- chercher dans la mémoire avant de chercher sur le web ;
- n'enregistrer que ce qui est vérifié et réutilisable ;
- dire si une entrée l'a aidé.

## Ligne de commande

`loragctl` parle au serveur (variables `LORAG_HOST` et `LORAG_PORT` pour un autre serveur).

```sh
loragctl index ~/notes --recursive                 # indexer
loragctl search "procédure de rollback"            # chercher
loragctl status                                     # état
loragctl watch list | add <dossier> | remove <id>   # dossiers surveillés
loragctl doc list | delete <id>                     # documents

loragctl mem recall "Error: listen EADDRINUSE: address already in use :::3000"
npm run dev 2>&1 | loragctl mem recall -            # rappel depuis un log
loragctl mem save --kind fix --error "$ERR" --fix "lsof -ti :3000 | xargs kill -9" --project shop
loragctl mem save --kind knowledge --problem "…" --answer "…" --source https://…
loragctl mem feedback <id> --worked | --failed
loragctl mem list --kind knowledge --status recheck
loragctl mem stats
loragctl mem export > memoire.jsonl
```

## Vos données

| Emplacement | Contenu |
| --- | --- |
| `<données>/lorag.db` | documents, chunks, réglages, dossiers surveillés |
| `<données>/vectix/` | index de recherche et mémoire des agents |
| `<données>/imports/` | fichiers déposés dans le Studio |
| `~/.lorag/models/` | modèles Whisper téléchargés |

`<données>` vaut `~/.lorag/data` par défaut (`-d` ou `LORAG_DATA_DIR` pour changer).

Les versions précédentes rangeaient les données dans `./data`, relatif au dossier de lancement. Si ce dossier existe et que `~/.lorag/data` est vide, Lorag continue d'utiliser `./data` et l'indique au démarrage. Pour passer au nouvel emplacement : arrêtez le serveur, puis `mkdir -p ~/.lorag && mv data ~/.lorag/data`.

- **Sauvegarde :** arrêtez le serveur et copiez le dossier.
- **Export de la mémoire :** Memory → bouton d'export, ou `loragctl mem export`. Le format est un JSON par ligne.
- **Clés et mots de passe :** ceux qui se trouvent dans une entrée de mémoire sont masqués avant l'écriture.

## Dépannage

| Symptôme | Que faire |
| --- | --- |
| Le serveur ne démarre pas : `database locked by another process` | Un autre `lorag server` utilise ce dossier de données ; arrêtez-le |
| Port déjà utilisé | `./bin/lorag server -p 8090` |
| *Embedder unavailable* | Lancez Ollama et installez le modèle configuré ; Settings → *Test embedder* |
| *out of sync* sur Overview | L'index est en reconstruction, ou des chunks manquent : Settings → Rebuild index |
| Un fichier échoue | Ingestion → Failed donne la raison (outil manquant, transcription désactivée, fichier illisible) |
| Un rappel de mémoire ne trouve pas une entrée existante | Memory → Test a recall avec la même question : en cas d'échec, il montre l'entrée la plus proche et sa similarité face au seuil. Avec un modèle peu multilingue (`nomic-embed-text`), passez à `bge-m3` ; sinon ajustez les seuils dans Settings → Memory |
| `max chunks limit reached (5000)` | Limite sans clé de licence, voir [docs/STATUS.md](docs/STATUS.md) |
