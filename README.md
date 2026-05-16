# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run dev server (port 8080)
go run .

# Hot reload — watches *.go, *.html, *.css; requires Air
go install github.com/air-verse/air@latest
air

# Build binary
go build -o ./tmp/app.exe .
```

Go version: **1.22**. No tests exist in this codebase.

## Architecture

**Stack:** Go stdlib `net/http` + SQLite (`modernc.org/sqlite`, pure-Go) + Go `html/template` + HTMX

**Request flow:** `routes/routes.go` → `handlers/` → `services/` → `models/` → SQLite (`data/app.db`)

**Config:** `PORT` env var (default 8080). SQLite path is hardcoded to `data/app.db` (auto-created on startup). The SQLite connection pool is intentionally capped at `MaxOpenConns(1)`, so all writes serialize — no concurrent write contention, but no parallelism either.

### Layers

| Layer | Location | Responsibility |
|-------|----------|---------------|
| Router | `routes/routes.go` | Maps URL patterns to handlers |
| Handlers | `handlers/` | Renders full pages or HTMX partials via `renderer.go` |
| Services | `services/` | Validation and business logic |
| Models | `models/` | SQL queries; auto-migrates tables on startup |
| Templates | `templates/` | `layouts/base.html` + `pages/` (full pages) + `partials/` (HTMX swaps) |

### Generic Module Pattern
`models/business.go` defines a single `ModuleStore` backed by a `modules` table that drives 10+ entities: Customers, Categories, Vendors, Invoices, Purchase Orders, Users, Payments, Credit Notes, Jobs, and Accounts.

To add a new entity, add a `ModuleConfig` entry to the `configs` slice in `services/module_service.go:NewModuleService()` — define its `Key`, `Table`, `Fields`, and `Description`. The router in `routes/routes.go` registers CRUD routes for all modules via a single loop, so no route changes are needed.

### Templating
- `handlers/renderer.go` distinguishes full-page renders from HTMX partial renders based on the `HX-Request` header.
- HTMX partial templates live in `templates/partials/` and are swapped into the DOM without a full page reload.
- There is no JSON API — all responses are HTML.
