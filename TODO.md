# À faire

## Secret `VECTIXDB_TOKEN` (bloque la CI)

Lorag dépend du module Go privé `github.com/raucheacho/vectixdb/bindings/go`. La CI, la release et l'image Docker le téléchargent avec le secret `VECTIXDB_TOKEN`, qui n'existe pas encore : `go vet` échoue avec « Invalid username or token ».

1. GitHub → Settings → Developer settings → Fine-grained tokens → **Generate new token**.
2. Repository access : **Only select repositories** → `raucheacho/vectixdb`.
3. Permissions : **Contents → Read-only** (Metadata est ajouté d'office).
4. L'ajouter au dépôt Lorag :

   ```sh
   gh secret set VECTIXDB_TOKEN -R raucheacho/lorag
   ```

5. Relancer la CI de `develop`, puis ouvrir la PR `develop` → `main` une fois verte.

Une clé de déploiement SSH ne convient pas : Go télécharge le module en HTTPS.

## En local

```sh
go env -w GOPRIVATE='github.com/raucheacho/*'
```

Évite que Go interroge le proxy et la base de sommes publics pour le module privé.

Le reste des tâches ouvertes est dans [docs/STATUS.md](docs/STATUS.md#à-faire).
