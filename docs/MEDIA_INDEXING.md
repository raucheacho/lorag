# Indexation de médias (Images, Audio, Vidéo)

## Prérequis

### Pour les images (OCR Vision)

**Obligatoire : Ollama avec un modèle vision**

```bash
# 1. Installer Ollama si pas déjà fait
# macOS: brew install ollama
# Linux: curl -fsSL https://ollama.com/install.sh | sh

# 2. Démarrer Ollama
ollama serve

# 3. Installer un modèle vision
ollama pull llava:7b          # Recommandé
ollama pull llava:latest       # Version plus légère
```

**Configuration dans l'UI :**
- Settings → Advanced
- Activer "OCR (Reconnaissance de texte)"
- Modèle OCR : `llava:7b`

### Pour l'audio et les vidéos (Transcription)

**Obligatoire : whisper.cpp + modèle GGML**

#### 1. Installer whisper.cpp

```bash
# macOS avec Homebrew
brew install whisper.cpp

# Ou compiler depuis les sources
git clone https://github.com/ggerganov/whisper.cpp.git
cd whisper.cpp
make

# Copier le binaire dans le PATH
sudo cp main /usr/local/bin/whisper-cli
```

#### 2. Les modèles sont téléchargés automatiquement

LoraG télécharge automatiquement le modèle choisi au premier usage :
- **Tiny** (39 MB) : Rapide, qualité standard
- **Base** (74 MB) : Bon équilibre qualité/vitesse  
- **Small** (244 MB) : Haute qualité, plus lent

Les modèles sont stockés dans `~/.lorag/models/`.

#### 3. Pour les vidéos uniquement : ffmpeg

```bash
# macOS
brew install ffmpeg

# Ubuntu/Debian  
sudo apt install ffmpeg

# Windows
choco install ffmpeg
```

## Messages d'erreur

| Erreur | Solution |
|--------|----------|
| `OLLAMA_NOT_RUNNING` | `ollama serve` |
| `VISION_MODEL_NOT_INSTALLED` | `ollama pull llava:7b` |
| `WHISPER_CPP_NOT_FOUND` | Installer whisper.cpp (voir ci-dessus) |
| `FFMPEG_NOT_INSTALLED` | `brew install ffmpeg` ou équivalent |

## Fonctionnement

### Images
1. Image envoyée à Ollama Vision (llava:13b)
2. Le modèle décrit l'image et extrait le texte
3. Résultat chunké et indexé

### Audio
1. Conversion en WAV 16kHz si nécessaire (via ffmpeg)
2. Transcription avec whisper.cpp + modèle choisi
3. Texte chunké et indexé

### Vidéos
1. Extraction audio avec ffmpeg
2. Conversion en WAV 16kHz mono
3. Transcription avec whisper.cpp
4. Texte chunké et indexé

## Choix du modèle Whisper

Dans Settings → Advanced :

| Modèle | Taille | Vitesse | Qualité | Usage |
|--------|--------|---------|---------|-------|
| **Tiny** | 39 MB | Très rapide | Correcte | Tests, fichiers courts |
| **Base** | 74 MB | Rapide | Bonne | Usage général |
| **Small** | 244 MB | Modéré | Excellente | Qualité maximale |

## Architecture

```
Image → Ollama Vision (llava:7b) → Texte → Chunks → Index

Audio → ffmpeg (si conversion) → whisper.cpp → Texte → Chunks → Index

Vidéo → ffmpeg (extraction audio) → whisper.cpp → Texte → Chunks → Index
```

## Dépannage

### whisper.cpp non trouvé
```bash
# Vérifier l'installation
which whisper-cli
which whisper-cpp
which main  # Si compilé depuis les sources

# Si pas trouvé, copier dans le PATH
cd whisper.cpp
sudo cp main /usr/local/bin/whisper-cli
```

### Modèles ne se téléchargent pas
Vérifiez la connexion internet. Les modèles sont téléchargés depuis HuggingFace :
- https://huggingface.co/ggerganov/whisper.cpp

### Erreur CUDA/GPU
whisper.cpp fonctionne sur CPU par défaut. Pour utiliser le GPU, compilez avec les flags appropriés :
```bash
WHISPER_CUDA=1 make
```
