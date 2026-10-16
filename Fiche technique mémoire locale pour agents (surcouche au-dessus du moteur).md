# Fiche technique : mémoire locale pour agents (surcouche au-dessus du moteur)

Oct 16, 2026 · @Rauche Acho

## Vision et objectifs

Le produit est une mémoire locale que n'importe quel agent peut interroger en quelques tokens, et à laquelle la personne ajoute ses propres documents, sons et vidéos. Il se place au-dessus d'un moteur de stockage et de recherche écrit en bas niveau.

- **Réutiliser le savoir :** un problème déjà résolu revient avec sa solution éprouvée en 30 tokens ou moins, sans nouvelle recherche.
- **Documentation personnelle :** la personne importe documents, notes, audio et vidéo, et les retrouve par le sens ou par le texte exact.
- **Agnostique :** Claude Code, un modèle local via Ollama ou tout autre agent se branche sur la même mémoire (MCP, ligne de commande, HTTP local).
- **Local et possédé :** index, modèles et fichiers restent sur la machine, avec un export ouvert.
- **Observable :** une interface montre ce qui est appris, utilisé, oublié, et les tendances.

**Hors périmètre de la v1 :** synchronisation cloud, plusieurs utilisateurs sur un même serveur, recherche sémantique exhaustive.

## Inspiration : Zvec et zvec-grep (zg)

On reprend l'idée d'un moteur embarqué sous une couche de recherche unifiée, et on ajoute ce qu'ils ne couvrent pas : la mémoire problème-solution, l'ingestion multimodale et l'interface d'observation.

| Projet | Ce que c'est | Ce qu'on reprend | Ce qu'on ajoute ou change |
| --- | --- | --- | --- |
| [Zvec](https://zvec.org/en/docs/db/) (Alibaba, Apache 2.0) | Base vectorielle qui s'exécute dans le processus de l'application, sans serveur | Moteur embarqué, journal d'écriture (WAL) pour la durabilité, vecteurs denses et épars, recherche hybride avec filtres, index qui vont de la mémoire au disque | Écrivain unique géré par notre orchestrateur : Zvec autorise plusieurs lecteurs mais un seul processus écrivain |
| [zvec-grep (zg)](https://github.com/zvec-ai/zvec-grep) | Couche de recherche locale qui unifie ripgrep, BM25 et recherche vectorielle, pour humains et agents | Quatre routes (hybride, texte BM25, vectorielle, exacte), fusion RRF, index local incrémental, intégration MCP, résultats compacts | Mémoire problème-solution, audio, vidéo, PDF et Office (non couverts par zg), interface d'observation |

Limite à retenir de zg : les routes indexées renvoient des échantillons classés, seule la route exacte est exhaustive. La règle de travail en découle : le sens sert à découvrir, le texte exact sert à prouver.

## Principes et périmètre

Tout fonctionne sans réseau par défaut, et le journal d'événements est la seule source de vérité.

1. **Local d'abord :** index, modèles d'embedding, transcription et vision tournent sur la machine ; aucune donnée ne sort sans action explicite.
2. **Journal d'abord :** chaque écriture est un événement ajouté à un journal ; les index sont dérivés et reconstructibles.
3. **Modèles interchangeables :** chaque vecteur garde l'identifiant du modèle qui l'a produit, ce qui permet de recalculer plus tard sans casser la base.
4. **Exports ouverts :** JSONL pour les entrées, fichiers d'origine conservés tels quels.
5. **Espaces :** une mémoire globale (savoir transversal) et des espaces par projet, avec des filtres à la recherche.
6. **Réponses compactes :** tout résultat est calibré pour un budget de tokens que l'agent peut fixer.

## Architecture en couches

Les agents et la personne parlent à l'orchestrateur Go, qui pilote l'ingestion locale ; seul le moteur Zig écrit sur le disque, par un journal à écrivain unique.

&#91;embedded content: architecture en couches · agents, orchestrateur Go, moteur Zig, disque\]

Lecture de haut en bas : requêtes et résultats circulent par le protocole binaire entre l'orchestrateur et le moteur ; l'ingestion transforme les fichiers en chunks que l'orchestrateur dépose dans le moteur.

## Ingestion multimodale

Tout fichier devient une suite de morceaux de texte (chunks) qui gardent leur position d'origine, pour que chaque résultat renvoie à l'endroit exact : page, ligne ou minute.

| Type | Extraction locale | Découpage | Position conservée |
| --- | --- | --- | --- |
| Texte, Markdown, code | Lecture directe | Par structure (titres, fonctions) | Fichier, ligne |
| PDF, Word, Excel, PowerPoint | Extraction du texte et des tableaux ; OCR local pour les scans | Par section et par page | Fichier, page |
| Audio | Transcription locale (modèle de type Whisper) avec horodatage | Par segment de parole | Fichier, minute début-fin |
| Vidéo | Transcription de la piste audio, plus images clés décrites par un modèle de vision local ou lues par OCR | Par scène ou par segment de parole | Fichier, minute début-fin |
| Images | OCR et légende par modèle de vision local | Une image = un chunk | Fichier |

Pipeline : détecter le type, extraire, découper, calculer les embeddings, écrire les événements. L'import est idempotent grâce au hash du contenu, et les mises à jour sont incrémentales (seuls les fichiers modifiés sont retraités).

Les modèles précis (transcription, vision, embedding) restent à choisir et à mesurer sur la machine cible, en français comme en arabe. L'ingestion vidéo est la plus coûteuse en calcul et passe en tâche de fond.

## Modèle de données

Cinq objets suffisent. Le journal d'événements est la vérité ; sources, chunks et entrées de mémoire en sont des vues indexées.

| Objet | Champs principaux |
| --- | --- |
| Événement | numéro de séquence, type, contenu, horodatage, auteur (agent ou personne) |
| Source | identifiant, type, chemin ou URL, hash du contenu, date d'import, métadonnées |
| Chunk | identifiant, source, texte, position (page, ligne ou minutes), vecteur, identifiant du modèle d'embedding, nombre de tokens |
| Entrée de mémoire | signature du problème, problème en une ligne, solution compressée, contexte (versions, plateforme, projet), statut, date de dernière vérification, compteurs de succès et d'échec, origine, niveau de confiance |
| Espace | global ou projet, avec ses règles d'accès |

Statuts d'une entrée : valide, à revérifier, obsolète. Une entrée n'est jamais écrasée : une nouvelle version s'ajoute et l'historique reste consultable.

## Recherche

Quatre routes, comme dans zg, avec une fusion par rang pour la route par défaut.

| Route | Sert à | Moteur |
| --- | --- | --- |
| Exacte | Identifiants, messages d'erreur, chaînes précises ; seule route exhaustive | Recherche textuelle de type ripgrep |
| Texte pondéré | Mots-clés classés par pertinence | BM25 sur l'index plein texte |
| Vectorielle | Retrouver par le sens quand on ignore les mots | Index vectoriel (HNSW) |
| Hybride (défaut) | Sens et mots-clés ensemble | Fusion RRF des routes texte et vectorielle |

La fusion RRF additionne, pour chaque résultat, l'inverse de son rang dans chaque liste (k vaut typiquement 60) :

```latex
score(d) = \sum_{i} \frac{1}{k + rang_i(d)}
```

Toutes les routes acceptent des filtres (espace, type de source, version, statut). Un résultat compact contient l'identifiant, un extrait court, la source avec sa position et le score, dans un budget de tokens fixé par l'agent. Pour un message d'erreur, la route exacte passe en premier ; l'hybride prend le relais si rien n'est trouvé.

## Mémoire problème-solution

C'est le cœur de l'économie de tokens : un « Stack Overflow personnel » où chaque solution est éprouvée, datée et liée à son contexte.

1. **Capture :** quand un agent résout un problème, il appelle `memory.save` avec le problème, la solution et le contexte. Le moteur calcule une signature à partir de l'erreur normalisée (chemins, numéros de ligne et versions retirés).
2. **Dédoublonnage :** même signature, on ajoute une variante ou on met à jour avec historique, jamais d'écrasement silencieux.
3. **Rappel :** avant toute recherche sur le web, l'agent appelle `memory.recall` avec l'erreur. La réponse tient en 30 tokens ou moins.
4. **Retour d'expérience :** l'agent indique si la solution a fonctionné ; les compteurs de succès et d'échec évoluent.
5. **Vieillissement :** si les versions du projet diffèrent de celles de l'entrée, elle passe à revérifier ; des échecs répétés la rendent obsolète.

Format de réponse visé pour un rappel :

```text
ERR expo-metro-resolve | valide, vérifiée 2026-10-02, 4 succès/0 échec
Fix: supprimer .expo et node_modules, puis `npx expo start -c`
Ctx: expo 52, rn 0.76
```

Cet exemple est illustratif : le format exact se fixe avec des cas réels.

## Moteur (Zig) et orchestrateur (Go)

Deux processus séparés par un protocole binaire versionné : le moteur possède le fichier de données et n'a qu'un écrivain, l'orchestrateur reçoit les requêtes en parallèle.

**Moteur de stockage (Zig)**

- Journal d'écriture (WAL) comme source de vérité, avec un `fsync` par lot d'écritures (group commit).
- Table en mémoire, puis segments immuables sur disque (mmap), chacun avec son index vectoriel (HNSW), son index plein texte (BM25) et ses métadonnées ; compactage en arrière-plan.
- Un thread écrivain unique ; les lecteurs travaillent en parallèle sur des instantanés.

**Orchestrateur (Go)**

- Une goroutine par requête entrante : validation, calcul des embeddings via une file de travail, puis dépôt de l'écriture dans un channel vers le moteur.
- Politique de conflits : clé stable par signature de problème, versionnage, règle de choix entre variantes (dernière écriture, ou la plus validée par l'usage).
- Supervision du moteur (relance après plantage), sessions d'agents, flux d'événements pour l'interface.

**Protocole**

- Messages binaires préfixés par leur longueur, sur socket Unix (tube nommé sous Windows), version du protocole dans chaque message.
- Opérations : `put`, `get`, `search`, `delete`, `snapshot`, `subscribe`.
- Ce protocole est la pièce à figer en premier : il isole le moteur, qui est un projet open source indépendant, développé directement en Zig sans étape intermédiaire par SQLite. Le binaire du moteur peut être embarqué dans l'exécutable Go (`go:embed`) pour livrer un seul fichier.

## Interface et intégration des agents

Une interface locale (page servie par le démon) pour voir et piloter la mémoire, et trois portes d'entrée pour les agents.

| Vue | Ce qu'on y voit |
| --- | --- |
| Tableau de bord | Volume, croissance, répartition par type de source |
| Chronologie | Ce qui a été appris, utilisé, oublié, dans le temps |
| Tendances | Problèmes récurrents, solutions les plus utiles, tokens économisés |
| Explorer | Recherche, ouverture de la source à la bonne position (page, minute) |
| Importer | Glisser-déposer de documents, audio, vidéo, avec l'avancement du traitement |
| Santé | Entrées à revérifier ou obsolètes, doublons |

**Intégration**

- **MCP :** outils `memory.save`, `memory.recall`, `search`, `ingest` ; configuration automatique pour Claude Code et les autres clients MCP, comme zg le fait pour Codex, Claude Code, Cursor et OpenCode.
- **Ligne de commande :** `mem index`, `mem search`, `mem save`.
- **HTTP local :** pour les applications et les modèles locaux (Ollama), sur la même API que le MCP.

## Sécurité, vie privée et critères de réussite

Une mémoire que les agents lisent sans réfléchir est une cible : le contenu importé ne doit jamais devenir une instruction.

- **Local :** aucun appel réseau par défaut ; modèles en cache sur la machine.
- **Chiffrement au repos**, avec la clé dans le trousseau du système.
- **Anti-empoisonnement :** chaque entrée garde son origine (quel agent, quel projet, quelle source) et un niveau de confiance ; le contenu importé est marqué non fiable ; une entrée n'est jamais exécutée automatiquement.
- **Secrets :** détection et masquage des clés API et jetons avant écriture.
- **Droits :** seuls les agents autorisés écrivent ; la personne peut tout voir, corriger, supprimer.

Cibles proposées, à valider sur des cas réels :

| Mesure | Cible proposée |
| --- | --- |
| Taille d'un rappel de solution | 30 tokens ou moins |
| Tokens économisés sur des problèmes répétés | Mesurés contre une recherche web à chaque fois, sur au moins 20 cas |
| Qualité du rappel | Rappel@5 d'au moins 90 % sur un jeu de test annoté |
| Latence de recherche hybride | Moins de 50 ms sur 100 000 chunks |
| Reprise après plantage | Reconstruction des index depuis le journal, sans perte |

## Moteur open source et différenciation

Le moteur est un projet à part, publié en open source, et l'application mémoire le consomme comme une dépendance.

- **Dépôt du moteur :** cœur en Zig, API C stable avec son en-tête, exécutable en ligne de commande, clients (Go en premier), spécification du protocole et du format de fichier, benchmarks reproductibles.
- **Dépôt de l'application :** orchestrateur, ingestion, interface ; elle inclut le moteur comme sous-module Git ou dépendance Zig (`build.zig.zon`), et lui parle par le protocole.
- **Licence :** à décider ; Apache-2.0 (celle de Zvec) ajoute une clause de brevets, MIT est plus simple.

Pistes de différenciation, à départager par un état de l'art plus complet avant de les annoncer :

| Piste | Ce que ça apporte | Ce qui existe déjà |
| --- | --- | --- |
| Requête unique exact + BM25 + vectoriel | Un seul plan d'exécution sur un seul index, au lieu de trois moteurs assemblés | zg assemble ripgrep, BM25 et Zvec derrière une interface ; Zvec fait déjà hybride avec filtres |
| Validité par contexte | Chaque entrée porte un prédicat (par exemple une plage de versions) indexé ; la requête passe le contexte courant et seules les entrées valides remontent | Non trouvé comme fonction native d'un moteur |
| Recherche sous budget de tokens | Demander le meilleur ensemble de résultats qui tient en N tokens, sans doublons de sens | Non trouvé |
| Dynamique de mémoire native | Renforcement par l'usage, vieillissement et oubli dans le score et le compactage | Surtout au niveau applicatif (couches de mémoire pour agents), pas dans le moteur |
| Versions, branches, voyage dans le temps | Instantanés et branches sans copie grâce aux segments immuables | Déjà fait : [Proximum](https://cljdoc.org/d/org.replikativ/proximum/0.1.35) (JVM) ; seule l'implémentation Zig sans dépendances serait nouvelle |

La combinaison la plus défendable est celle des trois lignes du milieu : un moteur de rappel pensé pour les agents, où le contexte et le budget de tokens font partie de la requête. Une affirmation d'innovation ne se publie qu'après un benchmark public face à Zvec et aux autres.

## Feuille de route, risques et questions ouvertes

Chaque phase se termine par un critère à atteindre avant de passer à la suivante.

1. **Phase 0, contrat :** protocole, format de fichier, schéma des entrées, jeu de test et benchmark. *Critère :* spécification figée, benchmark exécutable.
2. **Phase 1, noyau du moteur (Zig) :** journal, segments immuables, index plein texte et vectoriel, recherche hybride, API C et exécutable. En parallèle, l'orchestrateur Go se développe contre un simulateur du protocole. *Critère :* rappel@5 et latence mesurés, tests de pannes en simulation déterministe réussis.
3. **Phase 2, différenciation et publication :** les fonctions retenues dans la section sur le moteur open source. *Critère :* benchmark public face à Zvec et aux autres, puis publication du dépôt du moteur.
4. **Phase 3, orchestrateur et mémoire problème-solution :** MCP et ligne de commande, branchés sur ton propre usage. *Critère :* tokens économisés mesurés sur au moins 20 problèmes répétés.
5. **Phase 4, documents, audio, vidéo et interface.** *Critère :* un résultat renvoie à la bonne page ou à la bonne minute.
6. **Phase 5, plusieurs appareils :** réplication par envoi du journal.

**Risques**

- Un mauvais rappel coûte plus de tokens qu'il n'en économise.
- Une solution périmée après une mise à jour de version induit l'agent en erreur.
- L'ingestion vidéo locale demande beaucoup de calcul.
- Un moteur maison est une charge de maintenance : correctifs, formats, migrations.

**Questions ouvertes**

- Quel modèle d'embedding local, multilingue (français, arabe) et de quelle dimension ?
- Quelles plateformes cibles : macOS, Linux, Windows ?
- Interface web locale ou application native ?
- Licence du projet.

**Sources**

- [Zvec, documentation](https://zvec.org/en/docs/db/)
- [Zvec, dépôt GitHub](https://github.com/alibaba/zvec)
- [zvec-grep, README](https://github.com/zvec-ai/zvec-grep)
- [zg, guide complet (AgentPedia)](https://agentpedia.codes/blog/zg-local-search-complete-guide)
- [zvec-grep, présentation (AlphaSignal)](https://alphasignal.ai/news/alibaba-s-zvec-grep-gives-coding-agents-one-search-layer-to-replace-endless)
