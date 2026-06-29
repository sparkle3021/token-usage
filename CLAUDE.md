# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Backend
wails dev       # Live development (Vite HMR + Go backend)
wails build     # Production build

# Frontend (inside frontend/)
npm run dev     # Vite dev server only (browser mode, port 5173)
npm run build   # Vite production build
npm run lint    # Run oxlint
npm run preview # Preview Vite production build

# Lint frontend
npx oxlint@latest --fix  # Auto-fix lint issues
```

## Tech Stack

- **Desktop Shell**: Wails v2 (Go → WebView2)
- **Backend**: Go 1.23, Wails v2.12
- **Frontend**: React 19, Vite 8, Tailwind CSS v4
- **UI**: shadcn/ui (based on @base-ui/react)
- **Linter**: oxlint (replaces ESLint)
- **Font**: Geist Variable

## Architecture

### Go Backend (`main.go` → `app.go`)

Wails binds the `App` struct to the frontend. Methods on `App` become callable from JS via `window.go.main.App.MethodName()`.

- `main.go` — Entry point, Wails app config (title, size, asset embed)
- `app.go` — `App` struct holding `context.Context` + `*service.Service`
- `internal/service/` — Business logic layer (to be implemented)
- `internal/model/` — Data model definitions
- `internal/util/` — Shared utility functions

Wails embeds `frontend/dist/` into the Go binary at build time.

### React Frontend (`frontend/`)

- `src/App.jsx` — Root component
- `src/components/ui/` — shadcn/ui components (currently: `button.jsx`)
- `src/lib/utils.js` — `cn()` utility (clsx + tailwind-merge)
- `src/index.css` — Tailwind v4 + shadcn theme variables (light/dark)
- `vite.config.js` — `@/` path alias mapped to `src/`, Tailwind CSS v4 plugin

### Communication Pattern

Frontend calls Go backend via Wails runtime bindings (`window.go.main.App.*`). Methods on `App` struct are directly exposed to JS. No HTTP API — Wails handles the IPC bridge between WebView2 JS context and Go.

### Project Config

- `wails.json` — Wails project config (frontend install/build/dev watcher commands)
- `frontend/package.json` — npm dependencies and scripts
- `frontend/.oxlintrc.json` — oxlint config (React hooks + export rules)
