# Compiler Lorag

## Prérequis

| Outil | Version | Rôle |
| --- | --- | --- |
| Go | 1.24 ou plus | démon et CLI (cgo activé) |
| Node.js et npm | 20 | le Studio |
| Compilateur C | | macOS : `xcode-select --install` ; Linux : `build-essential` |
| Zig | 0.16.0 | seulement pour les archives Linux (`zig cc` comme compilateur C croisé) |

VectixDB est un module Go privé. Une fois pour toutes :

```sh
go env -w GOPRIVATE='github.com/raucheacho/*'   # ni proxy ni base de sommes publics pour ce module
gh auth setup-git                               # ou une clé SSH : git doit pouvoir lire raucheacho/vectixdb
```

Outils facultatifs pour l'extraction locale (PDF par page, audio, vidéo, OCR) : voir [docs/MEDIA_INDEXING.md](docs/MEDIA_INDEXING.md). Sans eux, les fichiers concernés échouent avec un message clair dans Ingestion, et le reste s'indexe normalement.

## Comment le moteur est lié

- **Bibliothèque statique.** Le moteur de recherche est `libvectixdb.a`, liée dans `lorag` par cgo.
- **Module Go.** `go.mod` dépend d'une version publiée du binding (`github.com/raucheacho/vectixdb/bindings/go v0.1.0`). Le module contient la bibliothèque précompilée de chaque plateforme : ni Zig ni dépôt VectixDB pour compiler.
- **Changer de version.** `go get github.com/raucheacho/vectixdb/bindings/go@v0.2.0`, puis `go mod tidy`.
- **Tester un VectixDB local** (non publié) : un `go.work`, ignoré par git, avec le binding et le module de la plateforme ; compiler d'abord la bibliothèque avec `../vectixdb/scripts/go-libs.sh`.

  ```sh
  go work init . ../vectixdb/bindings/go ../vectixdb/bindings/go/lib/darwin_arm64
  ```

## Compiler

```sh
make build            # Studio (ui/dist), bin/lorag, bin/loragctl
./bin/lorag server
```

Étape par étape :

```sh
(cd ui && npm ci && npm run build)  # ui/dist, embarqué dans le binaire
go build -o bin/lorag ./cmd/lorag
go build -o bin/loragctl ./cmd/loragctl
```

Le dossier `ui/dist` doit exister au moment de compiler le serveur : le Studio y est embarqué (`go:embed`).

## Développement

```sh
make dev-server     # serveur sur :8080
make dev-ui         # Studio sur :5173 avec rechargement à chaud, /api relayé vers :8080
go test -race ./...
```

Détails : [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Docker

L'image est construite sur Debian (la bibliothèque VectixDB est liée à la glibc ; Alpine ne convient pas). Le module VectixDB étant privé, la construction reçoit un jeton GitHub comme secret, qui ne reste pas dans l'image :

```sh
GITHUB_TOKEN=$(gh auth token) make docker-build
# ou
GITHUB_TOKEN=$(gh auth token) docker build --secret id=github_token,env=GITHUB_TOKEN -t lorag .

docker run -p 8080:8080 -v "$PWD/data:/data" \
  -e LORAG_EMBEDDING_URL=http://host.docker.internal:11434 lorag
```

Dans le conteneur, Ollama tourne sur la machine hôte : d'où `host.docker.internal`.

## Archives de release

`scripts/dist.sh` produit les archives distribuées, une par plateforme, dans `dist/` :

```sh
(cd ui && npm ci && npm run build)
./scripts/dist.sh 0.1.0 darwin_arm64 darwin_amd64 linux_amd64 linux_arm64
# ou, pour la machine courante : make dist
```

Chaque archive `lorag_<version>_<os>_<arch>.tar.gz` contient `lorag`, `loragctl`, le README, la licence de Lorag, ainsi que la licence et la notice de VectixDB (Apache-2.0, à livrer avec le binaire). Le script écrit aussi `dist/SHA256SUMS`.

| Plateforme | Compilateur C pour cgo | Où |
| --- | --- | --- |
| `darwin_arm64`, `darwin_amd64` | clang d'Apple (`-arch`, macOS 13 minimum) | sur macOS seulement : le binaire lie des frameworks Apple |
| `linux_amd64`, `linux_arm64` | `zig cc`, contre la glibc 2.28 | depuis n'importe quelle machine. Le binaire tourne sur Debian 10, Ubuntu 20.04, RHEL 8 et plus récents |

`loragctl` est en Go pur. Windows n'est pas pris en charge : le moteur ne compile pas encore pour Windows (voir [docs/STATUS.md](docs/STATUS.md)).

### Publier une version

1. Mettre à jour le CHANGELOG si besoin, puis pousser un tag : `git tag v0.1.0 && git push origin v0.1.0`.
2. Le workflow `release.yml` compile les archives (macOS sur un runner macOS, Linux avec `zig cc`), vérifie le binaire natif de chaque runner, puis crée une **release en brouillon** avec les archives, `SHA256SUMS` et la formule Homebrew `lorag.rb`. Il pousse aussi l'image Docker (`ghcr.io/raucheacho/lorag`, amd64).
3. Relire le brouillon, puis le publier.
4. Copier `lorag.rb` dans le dépôt du tap Homebrew (`Formula/lorag.rb`).

Un tag avec un tiret (`v0.1.0-rc.1`) donne une préversion.

## Dépannage

| Erreur | Solution |
| --- | --- |
| `reading github.com/raucheacho/vectixdb/...: 404` ou `terminal prompts disabled` | Module privé : `GOPRIVATE` et accès git, voir les prérequis |
| `pattern all:dist: no matching files found` | Compiler le Studio d'abord : `cd ui && npm run build` |
| `zig: command not found` (archives Linux) | Installer Zig 0.16.0 |
