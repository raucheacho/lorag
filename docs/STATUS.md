# État du projet

Mis à jour le 17 octobre 2026. Ce document sert à reprendre le développement : où en sont les phases de la fiche technique, ce qui a été décidé, ce qui reste à faire et les problèmes connus.

## Les deux projets

| Projet | Rôle | Dépôt | Licence |
| --- | --- | --- | --- |
| **VectixDB** | Moteur de recherche embarqué générique (Zig, API C, binding Go) | `github.com/raucheacho/vectixdb` (public), version 0.1.0 publiée | Apache-2.0 |
| **Lorag** | L'application : démon Go, MCP, Studio, mémoire des agents | `github.com/raucheacho/lorag` (public) | MIT |

Lorag utilise VectixDB comme une bibliothèque liée dans le démon : pas de second processus. Il dépend d'une version publiée du module Go, qui contient les bibliothèques précompilées : Zig n'est pas nécessaire pour compiler Lorag. Le moteur ne connaît rien des agents ni de la mémoire ; Lorag construit tout ça avec des champs génériques.

## Phases de la fiche

| Phase | État | Critère de la fiche | Résultat |
| --- | --- | --- | --- |
| 0. Contrat | fait | spécification figée, benchmark exécutable | `vectixdb/docs/SPEC.md`, `vectix bench` |
| 1. Noyau du moteur | fait | rappel@5 et latence mesurés, tests de pannes en simulation | rappel@10 0,93 (int8) et 0,99 (f32) sur 100 000 vecteurs ; plus de 2 000 scénarios de plantage ; voir `vectixdb/docs/BENCH.md` |
| 2. Différenciation et publication | partiel | benchmark public face à Zvec, publication | comparaison avec Zvec faite ; primitives différenciantes (budget, validité par contexte, modulation du score) pas encore dans le moteur ; dépôt pas encore public |
| 3. Orchestrateur et mémoire | fait, à valider à l'usage | tokens économisés sur au moins 20 problèmes répétés | mémoire fix/knowledge, MCP, `loragctl mem`, vue Mémoire. Le critère se mesure sur l'usage réel (Mémoire → Tendances) |
| 4. Documents, audio, vidéo, interface | fait | un résultat renvoie à la bonne page ou minute | vérifié avec un PDF de 3 pages, un audio, une vidéo (transcription et texte à l'écran), du Markdown et du code |
| 5. Plusieurs appareils | pas commencé | | réplication par envoi du journal |

## Décisions prises

- **Architecture.** Moteur générique en bibliothèque dans le démon (pas de protocole binaire entre deux processus, contrairement à la fiche) : un seul binaire, pas de supervision à écrire.
- **Durabilité.** Écriture `sync` par défaut, `F_FULLFSYNC` sur macOS ; un `fsync` en échec empoisonne la poignée (leçons des issues de Zvec).
- **Mémoire.** Deux types d'entrées :
  - `fix` : problème → correctif ;
  - `knowledge` : question → réponse, avec sa source.

  Une entrée est une information à peser, jamais une tâche ; on n'enregistre que ce qui est vérifié et réutilisable, pas chaque bug.
- **Historique.** Rien n'est écrasé : toute modification est un événement (`memory_events`).
- **Espaces.** Une mémoire globale et une par projet.
- **Studio.** Pour développeurs, en anglais ; documentation utilisateur en français.
- **Docker.** Image Debian, pas Alpine : la bibliothèque VectixDB est liée à la glibc.

## Problèmes connus

### Résolu le 16 octobre : rappel raté en français

Une entrée `knowledge` enregistrée en français (« Je dois manger chaque jour avant midi ») n'est pas retrouvée par « moment de la journée où je mange ». Trois causes, mesurées :

1. **Le vecteur d'une entrée ne contient que le problème** (et la ligne d'erreur), pas la réponse. Ici l'agent a donné un titre vague (« Rappel personnel de l'utilisateur ») et tout le sens est dans la réponse. Similarité : 0,70 avec le problème seul, 0,77 avec problème + réponse.
2. **Les seuils (0,80 seul, 0,705 avec des mots communs) ont été calibrés sur des erreurs en anglais.** Avec nomic-embed-text, le français est tassé : vrais rappels de 0,69 à 0,77, questions sans rapport jusqu'à 0,60. De plus, l'anglais ne retrouve pas une entrée en français (0,37).
3. **Pas de racinisation dans le BM25** : « mange » ne correspond pas à « manger », donc l'aide des mots communs ne joue pas.

Corrections faites :
- **Vecteur des entrées.** Celui des entrées `knowledge` couvre la question **et** la réponse, et la recherche par mots inclut la réponse. Les entrées existantes sont recalculées automatiquement (`schemaVersion` 3).
- **Modèle par défaut.** C'est maintenant `bge-m3` (1 024 dimensions, multilingue).
- **Seuils.** Ils sont calibrés par modèle (bge-m3 : 0,55 et 0,50 ; nomic : 0,80 et 0,705) et réglables dans Settings → Memory.
- **Rappel raté.** Le testeur du Studio montre le candidat rejeté le plus proche.
- **Titres.** La description de `memory_save` demande un titre formulé comme on le chercherait, dans la langue de l'utilisateur.

Vérifié sur le vrai serveur avec bge-m3 :
- ta requête exacte retrouve l'entrée, en français comme en anglais ;
- les questions sans rapport ne renvoient rien.

Détails et mesures : `docs/MEMORY.md`.

Reste : le BM25 n'a toujours pas de racinisation. bge-m3 compense par le sens.

### Résolu le 16 octobre : changement de modèle à dimension égale

Le nom du modèle fait maintenant partie de l'empreinte de l'index et de celle de la mémoire. Changer de modèle, même à dimension égale, reconstruit l'index et recalcule la mémoire. Conséquence : au premier démarrage avec cette version, l'index est reconstruit une fois.

### Autres

- **Limite de 5 000 chunks.** Sans `LORAG_LICENSE_KEY`, `config.MaxChunksAllowed` plafonne l'index à 5 000 chunks. C'est un reste de l'ancien modèle « gratuit / payant » : à décider.
- **Activity log.** Il est en mémoire : les Agent logs repartent de zéro au redémarrage. Les statistiques de la mémoire, elles, viennent du journal persistant.
- **Préfixes nomic.** `nomic-embed-text` est utilisé sans ses préfixes `search_query:` et `search_document:`. Mesuré : pas de gain en français, donc laissé ainsi.
- **Vidéo, détection de scène.** Deux diapositives qui ne diffèrent que par du texte ne déclenchent pas la détection de changement de scène. Une image est donc prise au moins toutes les 5 secondes (ou durée/120). La position du texte à l'écran est précise à cet intervalle près.
- **Whisper.** Le modèle `tiny` comprend mal les termes techniques (« hell and roll back » pour « helm rollback »). Le texte à l'écran compense pour les présentations ; `base` ou `small` sont à essayer.
- **Fichiers non formatés.** Plusieurs anciens fichiers Go ne sont pas passés par `gofmt` (`indexer.go`, `queue.go`, `handlers.go`…).
- **Makefile.** `make lint` ne fonctionne pas (pas de script `lint` dans l'UI).
- **Exemples.** `examples/` utilise encore l'ancien nom d'outil `index_file` : il fonctionne comme alias de `ingest`.

## À faire

### Bloquant pour les releases
1. **Première release.** Le workflow est prêt (`release.yml`, `scripts/dist.sh`) mais n'a pas encore tourné sur GitHub ; les archives ont été construites et vérifiées en local (macOS arm64 et Intel, Linux amd64 et arm64 sur Debian 10 et Ubuntu 20.04). Il manque :
   - lister les licences des dépendances Go dans les archives (`go-licenses`) ;
   - l'image Docker arm64 (runner arm64 natif) ;
   - une commande `lorag service install` pour lancer Lorag avec la session sans Homebrew ni `install.sh`.

### Mémoire
2. Recalibrer les seuils de bge-m3 sur tes vraies entrées après quelques jours d'usage (Memory → Test a recall).
3. Mesurer le critère de la phase 3 : 20 problèmes répétés sur un usage réel, tokens économisés relevés dans Mémoire → Tendances.
4. Décider si une entrée enregistrée par un agent doit être validée par toi avant d'être rappelée (statut « proposée »).

### Studio et documentation
5. **Tests UI.** Mettre les tests d'interface dans le dépôt (`ui/e2e/`, Playwright) avec un script qui crée des données de test.
6. **Docs utilisateur.** Relire `USER_GUIDE.md` et `QUICKSTART.md` après chaque changement d'écran.

### Moteur (VectixDB)
7. **Primitives de la phase 2** dans le moteur : modulation du score par un champ ou par le temps, diversité MMR, budget de tokens, validité par plage de versions. Lorag les fait aujourd'hui de son côté.
8. **Racinisation optionnelle** par champ texte.
9. **Windows.** Le moteur ne compile pas pour Windows (`src/fs.zig` suppose des descripteurs de fichier entiers ; la CLI utilise `getrusage`). La correction de compilation est petite ; le travail est de valider la résistance aux plantages (remplacement atomique, synchronisation du dossier, verrou) sur un runner Windows.
10. **Reste de la feuille de route** dans `vectixdb/docs/DEVELOPMENT.md` : mmap, group commit, vecteurs épars, fuzzing, binding Python.

### Fiche technique
11. **Phase 5 : plusieurs appareils.** Le journal de VectixDB et celui de la mémoire sont la base de la réplication.
12. **Sécurité.** Chiffrement au repos, avec la clé dans le trousseau du système : prévu par la fiche, pas fait.
