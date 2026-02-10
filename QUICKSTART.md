# Démarrage Rapide Lorag

## 🚀 En 3 minutes chrono

### 1. Installation (30 secondes)

**Windows :**
```powershell
# Téléchargez et exécutez
https://github.com/lorag/lorag/releases/latest/download/lorag-windows.exe
```

**Mac :**
```bash
# Téléchargez le .dmg et glissez dans Applications
curl -LO https://github.com/lorag/lorag/releases/latest/download/lorag-macos.dmg
```

**Linux :**
```bash
curl -fsSL https://lorag.dev/install.sh | bash
```

### 2. Premier lancement (1 minute)

```bash
# Démarrez le serveur
lorag server

# Ou double-cliquez sur l'application
```

Ouvrez votre navigateur à : `http://localhost:8080`

### 3. Configuration initiale (2 minutes)

Suivez l'assistant qui s'ouvre automatiquement :

1. ✅ **Choisissez où stocker les données** (dossier par défaut recommandé)
2. ✅ **Ajoutez un dossier** contenant vos documents
3. ✅ **Laissez indexer** (cela prend quelques minutes)
4. ✅ **C'est prêt !**

## 🎯 Utilisation immédiate

### Rechercher dans vos documents

1. Cliquez sur **"Recherche"** dans le menu
2. Tapez votre question en français naturel :
   - ❌ Mauvais : `contrat date 2024`
   - ✅ Bon : `quel est le dernier contrat signé avec le client X ?`
3. Explorez les résultats avec scores de pertinence

### Connecter Claude (optionnel)

Dans Claude Desktop :
```
Paramètres > Extensions > Ajouter MCP
URL: http://localhost:8080/mcp
```

Demandez à Claude : *"Cherche dans mes documents comment configurer un serveur"*

## 📂 Quels dossiers indexer ?

**Recommandés :**
- 📁 Documents professionnels
- 📁 Notes et réunions
- 📁 Documentation technique
- 📁 Archives de projets

**À éviter :**
- ❌ Dossiers système
- ❌ Fichiers temporaires
- ❌ Vidéos et images (non supportés en version gratuite)

## ⚡ Commandes essentielles (CLI)

Pour les utilisateurs avancés :

```bash
# Indexer un dossier
loragctl index /chemin/vers/documents --recursive

# Rechercher
loragctl search "votre question ici"

# Voir le statut
loragctl status

# Ajouter une surveillance
loragctl watch add /chemin/vers/dossier
```

## 🆘 Problèmes courants

### "Port 8080 déjà utilisé"
```bash
lorag server --port 8081
```

### "Ollama non trouvé"
```bash
# Installez Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Téléchargez le modèle
ollama pull nomic-embed-text
```

### "Aucun résultat de recherche"
1. Vérifiez dans le Tableau de bord que des documents sont indexés
2. Attendez que l'indexation soit terminée
3. Essayez des termes plus généraux

## 🎓 Prochaines étapes

- **Planification** : Configurez l'indexation automatique quotidienne
- **Templates** : Personnalisez les prompts pour vos IA
- **MCP** : Connectez vos agents IA préférés

📖 **[Guide utilisateur complet](USER_GUIDE.md)**

---

**Besoin d'aide ?** 
- 💬 Discord : https://discord.gg/lorag
- ✉️ Email : support@lorag.dev
