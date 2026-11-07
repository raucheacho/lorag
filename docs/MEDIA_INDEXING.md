# Types de fichiers et positions

Chaque fichier est découpé en morceaux (chunks) qui gardent leur place dans la source. Un résultat de recherche indique donc la page d'un PDF, les lignes d'un fichier ou la minute d'un enregistrement, et le Studio ouvre la source à cet endroit.

| Type | Extensions | Extraction | Position |
| --- | --- | --- | --- |
| Texte | `.txt`, `.log`, `.csv`, `.tsv`, `.rst`, `.org` | lecture directe | lignes |
| Markdown | `.md`, `.markdown`, `.mdx` | découpage par titres | lignes et section (`Guide › Rollback`) |
| Code | `.go`, `.py`, `.ts`, `.js`, `.rs`, `.java`, `.c`, `.swift`, `.sql`, `.yaml`, `.json`… | découpage avant chaque déclaration (fonction, classe, type) | lignes et déclaration |
| PDF | `.pdf` | `pdftotext`, page par page | page |
| Présentations | `.pptx` | texte et notes de chaque diapositive | diapositive |
| Tableurs | `.xlsx` | une ligne de texte par rangée (`cellule \| cellule`) | feuille |
| Word | `.docx` | texte | aucune |
| Audio | `.mp3`, `.m4a`, `.wav`, `.ogg`, `.flac`, `.aac`, `.opus` | transcription horodatée (whisper.cpp) | minutes, au plus une minute par chunk |
| Vidéo | `.mp4`, `.mov`, `.mkv`, `.webm`, `.avi`, `.m4v` | transcription de la piste audio et texte lu à l'écran (OCR des images clés) | minutes |
| Images | `.png`, `.jpg`, `.webp`, `.gif`, `.bmp`, `.tiff` | texte lu par OCR ; description par un modèle de vision si activée | aucune |

Les pages, diapositives et feuilles ne sont jamais fusionnées entre elles. Les petites sections voisines d'un même fichier texte sont regroupées jusqu'à la taille d'un chunk.

## Outils à installer

Tout tourne en local. La formule Homebrew (`brew install raucheacho/tap/lorag`) installe déjà poppler, ffmpeg, whisper.cpp et tesseract. Sur Linux ou depuis les sources, installez-les vous-même ; sans l'outil d'un type de fichier, ce type échoue avec un message clair dans la page Ingestion, et le reste s'indexe normalement.

| Outil | Pour | macOS | Debian, Ubuntu |
| --- | --- | --- | --- |
| `pdftotext` (poppler) | PDF page par page | `brew install poppler` | `apt install poppler-utils` |
| `ffmpeg`, `ffprobe` | audio et vidéo | `brew install ffmpeg` | `apt install ffmpeg` |
| `whisper-cli` (whisper.cpp) | transcription | `brew install whisper.cpp` | à compiler depuis github.com/ggml-org/whisper.cpp |
| `tesseract` | texte dans les images et à l'écran des vidéos | `brew install tesseract` | `apt install tesseract-ocr` |
| Ollama + modèle de vision | description des images (facultatif) | `ollama pull llava:7b` | idem |

Sans `pdftotext`, Lorag extrait quand même le texte des PDF, mais sans les numéros de page.

## L'audio et la vidéo

La transcription est activée par défaut : il suffit que `ffmpeg` et `whisper-cli` soient installés. Pour changer de modèle Whisper ou la désactiver : Studio → Settings → Images and audio, puis redémarrer Lorag.

| Modèle | Taille | Remarque |
| --- | --- | --- |
| `tiny` | 75 Mo | rapide ; se trompe sur les termes techniques |
| `base` | 142 Mo | |
| `small` | 466 Mo | nettement meilleur, plus lent |

Le modèle est téléchargé au premier usage dans `~/.lorag/models`. La langue est détectée automatiquement.

## Vidéo : le texte à l'écran

Pour les présentations et les captures d'écran, ce qui est écrit à l'écran compte autant que ce qui est dit.

- **Quelles images.** Lorag lit le texte de l'image à chaque changement de scène, et au moins toutes les 5 secondes (ou toutes les durée/120 secondes pour une longue vidéo).
- **Doublons.** Un texte identique à un texte déjà lu est ignoré.
- **Placement.** Ce texte est rangé dans la transcription à son horodatage (`On screen: …`).
- **Exemple.** Une commande affichée sur une diapositive (« helm rollback payments 41 ») se retrouve même quand la transcription l'a mal entendue.

## Images

- **OCR.** Avec `tesseract`, le texte de l'image est toujours lu.
- **Description.** Avec **Image description** activée (Settings), un modèle de vision d'Ollama (par défaut `llava:7b`) décrit aussi l'image. Comptez quelques secondes par image.

## Réindexer après un changement de réglage

Les fichiers déjà indexés ne sont pas retraités automatiquement. Après avoir activé la transcription ou changé de modèle Whisper :
- **un fichier :** Studio → Explorer → le document → *Reindex* ;
- **tous les médias :** `POST /api/v1/documents/reindex-media` avec `{"types": ["audio", "video", "image"]}`.
