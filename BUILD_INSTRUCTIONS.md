# Instructions de Build - Lorag

## Prérequis

### Pour tous les systèmes
- Go 1.22 ou supérieur
- Node.js 18+ et npm
- Git
- CGO (inclus avec Go)

### macOS spécifique
- Xcode Command Line Tools: `xcode-select --install`

### Windows spécifique
- MinGW-w64 pour CGO: https://www.mingw-w64.org/
- Ou utiliser WSL2

## Build rapide

```bash
# 1. Cloner le repository
git clone https://github.com/lorag/lorag.git
cd lorag

# 2. Installer les dépendances
make install

# 3. Build complet (UI + binaires)
make build

# 4. Lancer
make launch
```

## Build étape par étape

### 1. UI Web

```bash
cd web
npm install
npm run build
cd ..
```

### 2. Backend Go

```bash
# Server
go build -o bin/lorag ./cmd/lorag

# CLI
go build -o bin/loragctl ./cmd/loragctl
```

### 3. Exécution

```bash
# Démarrer le serveur
./bin/lorag server

# Ou avec le script de lancement
./scripts/launch.sh
```

## Build pour release (multi-platforme)

```bash
make release
```

Crée les binaires pour :
- macOS (Intel & Apple Silicon)
- Linux (AMD64 & ARM64)
- Windows (AMD64)

## Docker

```bash
# Build
docker build -t lorag .

# Run
docker run -p 8080:8080 -v $(pwd)/data:/data lorag
```

## Structure des binaires

```
bin/
├── lorag              # Serveur principal
├── loragctl           # CLI
└── [archives release]
```

## Tests

```bash
# Tests Go
make test

# Tests avec couverture
make test-coverage

# Lint
make lint
```

## Packaging

```bash
make package
```

Crée des archives `.tar.gz` prêtes pour la distribution.

## Installation locale

```bash
make install-local
```

Installe les binaires dans `/usr/local/bin`.

## Dépannage

### "undefined: syscall.Something" sur Windows
- Assurez-vous d'utiliser MinGW-w64
- Ou compilez avec WSL2

### "cannot find -ldl" sur Linux
- Installez les headers: `sudo apt-get install build-essential`

### Erreurs CGO
- macOS: `xcode-select --install`
- Linux: `sudo apt-get install gcc`
- Windows: Installez MinGW-w64

### UI ne se build pas
```bash
cd web
rm -rf node_modules package-lock.json
npm install
npm run build
```
