# À faire

## Authentification avant tout déploiement sur un serveur

Le REST, le MCP et le Studio n'ont aucune authentification : seule l'écoute sur `localhost` les protège. Sur un serveur (`LORAG_HOST=0.0.0.0`, l'image Docker), n'importe qui sur le réseau peut lire le texte indexé et la mémoire, lister les dossiers du serveur (`/api/v1/fs/dirs`) et faire indexer n'importe quel chemin. Le CORS et le contrôle d'`Origin` du MCP n'arrêtent que les pages web, pas un `curl`.

1. Jeton `LORAG_TOKEN` exigé sur le REST, le MCP et le Studio (`Authorization: Bearer …`) ; refuser de démarrer sur une autre adresse que `localhost` sans jeton. Le Studio demande le jeton une fois, la page Connect l'ajoute à la configuration MCP.
2. HTTPS derrière un reverse proxy (Caddy ou nginx), documenté dans BUILD_INSTRUCTIONS.
3. Plus tard, pour la plateforme : comptes et droits par dossier, fournis par la plateforme qui intègre Lorag.

À savoir pour un déploiement : les chemins indexés sont ceux du serveur, les originaux ne sont pas copiés mais leur texte extrait est stocké dans `~/.lorag/data` ; un fichier glissé dans le Studio est copié dans `data/imports/`.

Le reste des tâches ouvertes est dans [docs/STATUS.md](docs/STATUS.md#à-faire).
