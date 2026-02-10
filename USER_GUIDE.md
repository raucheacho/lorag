# Guide Utilisateur Lorag

## Qu'est-ce que Lorag ?

**Lorag** est votre mémoire locale universelle pour agents IA. Il permet à n'importe quelle intelligence artificielle (Claude, ChatGPT, etc.) d'accéder à vos documents locaux de manière sécurisée et privée.

### Pourquoi utiliser Lorag ?

- 🔒 **100% Privé** - Vos données restent sur votre ordinateur
- 🤖 **Compatible IA** - Fonctionne avec Claude, GPT et autres agents
- 📄 **Multi-format** - PDF, Word, Markdown, code source, et plus
- 🔄 **Automatique** - Indexation automatique des nouveaux documents
- ⚡ **Rapide** - Recherche sémantique en quelques millisecondes

## Installation rapide

### Option 1 : Application Desktop (Recommandée pour les débutants)

1. Téléchargez Lorag pour votre système :
   - [Windows](https://github.com/lorag/lorag/releases/latest/download/lorag-windows.exe)
   - [Mac](https://github.com/lorag/lorag/releases/latest/download/lorag-macos.dmg)
   - [Linux](https://github.com/lorag/lorag/releases/latest/download/lorag-linux.AppImage)

2. Double-cliquez sur le fichier téléchargé

3. Suivez l'assistant d'installation

### Option 2 : Ligne de commande (Pour utilisateurs avancés)

```bash
# Sur macOS/Linux
curl -fsSL https://lorag.dev/install.sh | bash

# Sur Windows (PowerShell)
iwr -useb https://lorag.dev/install.ps1 | iex
```

## Premiers pas

### 1. Lancer Lorag

- **Windows/Mac** : Double-cliquez sur l'icône Lorag
- **Ligne de commande** : Tapez `lorag server`

L'interface web s'ouvrira automatiquement dans votre navigateur à l'adresse `http://localhost:8080`.

### 2. Assistant de configuration

Au premier démarrage, un assistant vous guidera :

1. **Choisissez l'emplacement des données** - Où stocker les index
2. **Ajoutez vos dossiers** - Sélectionnez les dossiers à indexer
3. **Configurez les embeddings** - Choisissez entre Ollama (local) ou OpenAI
4. **Démarrez l'indexation** - Lorag analysera vos documents

### 3. Commencer à utiliser

Une fois configuré, vous pouvez :

- **Rechercher** : Utilisez la barre de recherche pour trouver du contenu
- **Explorer** : Parcourez vos documents indexés
- **Connecter vos IA** : Utilisez l'endpoint MCP pour connecter Claude, GPT, etc.

## Fonctionnalités principales

### 🔍 Recherche sémantique

Au lieu de chercher des mots exacts, Lorag comprend le **sens** de votre recherche.

**Exemple :**
- Vous cherchez : "comment faire du machine learning"
- Lorag trouve : Des documents sur l'IA, l'apprentissage automatique, les tutoriels Python, etc.

**Comment faire :**
1. Allez dans l'onglet "Recherche"
2. Tapez votre question en langage naturel
3. Explorez les résultats avec le score de pertinence

### 📁 Gestion des sources

**Ajouter un dossier à surveiller :**
1. Allez dans "Sources"
2. Cliquez sur "Ajouter un dossier"
3. Sélectionnez le dossier contenant vos documents
4. Cochez "Récursif" pour inclure les sous-dossiers

Lorag surveillera automatiquement ce dossier et indexera les nouveaux fichiers.

### ⏰ Planification

Programmez l'indexation automatique pour maintenir votre base à jour :

1. Allez dans "Planification"
2. Créez une nouvelle tâche
3. Choisissez la fréquence :
   - **Toutes les heures** - Pour les dossiers très actifs
   - **Tous les jours** - Recommandé pour la plupart des usages
   - **Toutes les semaines** - Pour les archives

### 📝 Templates de prompts

Personnalisez comment vos résultats sont présentés aux IA :

1. Allez dans "Templates"
2. Créez un nouveau template ou modifiez un existant
3. Utilisez les variables :
   - `{{.Query}}` - La question de l'utilisateur
   - `{{.Context}}` - Le contenu trouvé dans les documents

**Exemple de template :**
```
Basé sur les documents suivants :

{{.Context}}

Réponds à cette question de manière concise : {{.Query}}
```

## Connecter vos agents IA

### Avec Claude (via MCP)

1. Ouvrez Claude Desktop
2. Allez dans Paramètres > Extensions
3. Ajoutez l'endpoint MCP : `http://localhost:8080/mcp`
4. Claude peut maintenant rechercher dans vos documents !

### Avec ChatGPT (via API)

Utilisez le plugin Lorag dans ChatGPT ou configurez manuellement :

```python
import requests

# Rechercher dans vos documents
response = requests.post('http://localhost:8080/api/v1/search', json={
    'query': 'votre question ici',
    'top_k': 5
})

results = response.json()['results']
```

### Avec VS Code

Installez l'extension Lorag pour VS Code :
1. Ouvrez les extensions (Ctrl+Shift+X)
2. Cherchez "Lorag"
3. Installez et configurez l'URL : `http://localhost:8080`

## Formats supportés

Lorag peut indexer :

| Format | Description |
|--------|-------------|
| **PDF** | Documents Adobe PDF |
| **DOCX** | Documents Microsoft Word |
| **Markdown** | Fichiers .md et .markdown |
| **Texte** | Fichiers .txt |
| **Code** | .py, .js, .go, .java, etc. |
| **HTML** | Pages web sauvegardées |

*Note : L'OCR pour les images est disponible en version Pro.*

## Dépannage

### Lorag ne démarre pas

**Vérifier si le port 8080 est libre :**
```bash
# Windows
netstat -ano | findstr :8080

# Mac/Linux
lsof -i :8080
```

**Changer le port :**
```bash
lorag server --port 8081
```

### L'indexation échoue

1. Vérifiez que Ollama est installé et en cours d'exécution :
   ```bash
   ollama --version
   ollama serve
   ```

2. Téléchargez le modèle d'embeddings :
   ```bash
   ollama pull nomic-embed-text
   ```

3. Vérifiez les permissions des dossiers à indexer

### La recherche ne trouve rien

1. Vérifiez que des documents sont indexés (Tableau de bord)
2. Essayez des termes de recherche plus généraux
3. Vérifiez que vos documents sont dans les formats supportés
4. Regardez les logs d'erreur dans le tableau de bord

## FAQ

**Q : Mes données sont-elles envoyées sur Internet ?**
R : Non ! Lorag fonctionne 100% localement. Vos documents ne quittent jamais votre ordinateur.

**Q : Puis-je utiliser Lorag sans connexion Internet ?**
R : Oui, si vous utilisez Ollama pour les embeddings. L'indexation et la recherche fonctionnent hors-ligne.

**Q : Quelle quantité de documents puis-je indexer ?**
R : Version gratuite : 5 000 chunks (environ 500-1000 documents). Version Pro : illimité.

**Q : Puis-je indexer des documents sur un NAS ou disque réseau ?**
R : Oui, montez le dossier réseau et ajoutez-le comme source.

**Q : Comment sauvegarder mon index ?**
R : Copiez le dossier de données (par défaut `./lorag-data` ou `~/Library/Application Support/Lorag` sur Mac).

## Ressources

- **Documentation technique** : [docs/API.md](docs/API.md)
- **Protocole MCP** : [docs/MCP.md](docs/MCP.md)
- **Support** : support@lorag.dev
- **Communauté** : [Discord](https://discord.gg/lorag)

## Mises à jour

Lorag se met à jour automatiquement. Pour mettre à jour manuellement :

```bash
loragctl update
```

---

**Besoin d'aide ?** Contactez-nous à support@lorag.dev
