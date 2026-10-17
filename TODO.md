# À faire

## Fusionner la PR #1

`develop` → `main` : Lorag sur VectixDB v0.1.0. La CI de `develop` est verte (tests Go, UI, image Docker).

## En local

```sh
go env -w GOPRIVATE='github.com/raucheacho/*'
```

Évite que Go interroge le proxy et la base de sommes publics pour le module privé VectixDB.

## Jeton `VECTIXDB_TOKEN`

Le secret existe dans le dépôt Lorag (fine-grained, *Contents: read* sur `raucheacho/vectixdb`). Le renouveler avant son expiration : sans lui, la CI, la release et l'image Docker ne peuvent plus télécharger le module.

Le reste des tâches ouvertes est dans [docs/STATUS.md](docs/STATUS.md#à-faire).
