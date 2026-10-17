# Contributing to Lorag

## Before you start

- [docs/STATUS.md](docs/STATUS.md) (in French): what is done, what is pending, known issues.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): how the daemon is built.
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md): setup, commands, tests, recipes for common changes.

The search engine is a separate project, [VectixDB](https://github.com/raucheacho/vectixdb), with its own development guide. Engine changes (storage, indexes, query features) go there. Lorag only uses the engine's generic API.

## Making a change

1. Branch from `main`.
2. Write the code and its tests. For ranking or memory changes, also check with real models (see DEVELOPMENT.md, "Checking with real models and files").
3. Run:

   ```sh
   go vet ./... && go test -race ./...
   (cd ui && npx tsc --noEmit && npm run build)
   ```

4. Update the documentation the change touches:

   | Change | Documentation |
   | --- | --- |
   | REST API | `docs/API.md` |
   | MCP tools | `docs/MCP.md` |
   | Memory | `docs/MEMORY.md` |
   | File types | `docs/MEDIA_INDEXING.md` |
   | Studio screens | `USER_GUIDE.md` |
   | Decisions and known issues | `docs/STATUS.md` |

5. Commit with a short message in French, lowercase, without prefix (`ajout du filtre par type`).

## Code style

- **Go.** `gofmt` the files you touch; comments explain why; errors are wrapped with context.
- **TypeScript.** Strict mode. Build screens from the primitives in `ui/src/components/` (Select, NumberField, InputShell, Button with `tip`, Drawer), not from native form controls.
- **Agent-facing text** (tool descriptions, MCP instructions, recall format). It is part of the product: keep it short, precise and in English, and measure its size in tokens when it is returned to agents.

## Reporting a bug

Open an issue with:
- what you did;
- what you expected;
- what happened;
- the Lorag version (`lorag version`), the OS, the embedding model;
- when relevant, the Agent logs entry or a Test a recall screenshot.
