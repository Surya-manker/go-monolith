# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```powershell
# Run the server (PowerShell)
$env:DATABASE_DSN = "root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4"
go run main.go

# Seed the database (admin + staff users + sample records; idempotent)
go run ./seed/

# Build binary
go build -o ./tmp/app.exe .

# Hot reload (requires Air)
go install github.com/air-verse/air@latest
air

# Tidy dependencies
go mod tidy
```

No test suite exists. Go version: **1.25**.

**Database setup:** `CREATE DATABASE invobill CHARACTER SET utf8mb4;` — schema auto-migrates on every startup via each store's `Migrate()` method, called in dependency order in `main.go`.

## Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_DSN` | `root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4` | Must include `parseTime=true` |
| `PORT` | `8080` | HTTP listen port |
| `GST_SELLER_NAME` | `InvoBill Company` | Appears on PDF invoices |
| `GST_SELLER_GSTIN` | _(empty)_ | First 2 digits auto-set `GST_STATE_CODE` |
| `GST_SELLER_ADDRESS` | _(empty)_ | Appears on PDF invoices |
| `GST_STATE_CODE` | first 2 chars of GSTIN | Determines CGST+SGST vs IGST |

## Architecture

**Stack:** Go stdlib `net/http` · MySQL (`go-sql-driver/mysql`) · `html/template` · HTMX 2 · `go-pdf/fpdf`

**Request flow:**
```
HTTP → Security → RateLimiter(60rps) → [Auth → CSRF →] routes/routes.go → handlers/ → services/ → models/ → MySQL
```

### Route categories (`routes/routes.go`)

| Category | Auth | CSRF | Examples |
|---|---|---|---|
| Public pages | No | No | `GET /`, `/about`, `/contact`, `/generator` |
| Auth pages | No | No | `/login`, `/register`, `/logout` |
| Protected HTML | Yes | Yes | `/dashboard`, `/products`, all module routes |
| REST API `/api/v1/*` | Yes (cookie) | No | JSON responses |

The catch-all `mux.Handle("/", authHandler)` routes everything not explicitly matched to the auth+CSRF-protected mux. Public pages must be registered **before** this catch-all on the outer `mux`.

### Renderer (`handlers/renderer.go`)

Three methods — choose based on layout needed:

| Method | Layout template | Use for |
|---|---|---|
| `Page(w, "page.html", data)` | `layouts/base.html` + sidebar | All authenticated app pages |
| `Auth(w, "page.html", data)` | `layouts/auth.html` | Login / register |
| `Landing(w, "page.html", data)` | `layouts/landing.html` | Public marketing pages |

Templates are parsed from disk on every request — changes are live without restart. The `funcMap()` exposes `initial` (first letter of a string) and `hasPermission` (RBAC check) to all templates.

### Generic Module System

Ten entities are driven by a single engine with no per-module route or template code:

- **Schema**: `models/business.go` — `ModuleStore` implements `List`, `Get`, `Create`, `Update`, `Delete`, `Trash`, `Restore`, `HardDelete` for any table.
- **Config**: `services/module_service.go:NewModuleService()` — a `[]ModuleConfig` slice defines each module's `Key`, `Table`, `Fields`, and `Title`.
- **Routes**: `routes/routes.go` — one loop registers 5 HTML + 3 trash routes per module.
- **Templates**: `templates/pages/crud.html` (full page) + `templates/partials/crud_table.html` (HTMX swap).

**To add a new generic module:** add a `ModuleConfig` entry to the slice in `NewModuleService()`. No other changes needed.

**Products are NOT generic** — `handlers/product_handler.go`, `services/product_service.go`, and `models/product.go` exist separately because products require stock management, SKU, and threshold fields.

### Public Landing Pages

`/`, `/about`, `/contact`, `/generator` all use `Renderer.Landing()` and the `layouts/landing.html` layout. The layout includes the shared navbar (Home · About · Generator · Contact), mobile hamburger menu, multi-column footer, and all animation + theme JS. Page templates only define `{{ define "content" }}` — no nav/footer repetition.

The `/generator` page is entirely client-side JavaScript — the Go handler just serves the template. It renders a live GST invoice preview and uses `window.print()` for PDF output.

### Animation System

`static/css/app.css` defines CSS keyframes and a scroll-triggered class system. Add `data-anim="up|fade|left|right|scale"` to any element in landing page templates. The `IntersectionObserver` in `landing.html` adds `.visible` when the element enters the viewport, triggering the transition. Inline `style="transition-delay:.Xs"` controls stagger timing.

### CSRF

`middleware/csrf.go` validates POST/PUT/DELETE on all protected HTML routes. Token lives in the `csrf_token` cookie; HTMX injects it via the `htmx:configRequest` listener in `base.html`. Plain HTML `<form method="POST">` must include a hidden `_csrf` field — otherwise 403.

### RBAC

`middleware/rbac.go` maps roles to `module:action` pairs. Roles: `super_admin` / `admin` (all), `manager`, `accountant`, `staff`. The template function `hasPermission .User.Role "module" "action"` gates UI elements. Always use `middleware.UserFromContext(r.Context())` in handlers — never read identity from request body.

### Toast Notifications

`handlers/crud_handler.go:setToast(w, message, type)` sets `HX-Trigger` before any body write. JS in `base.html` fires `showToast`. Types: `"success"`, `"error"`, `"warning"`, `"info"`.

### PDF Invoice Generation

`services/pdf_service.go:GenerateInvoicePDF()` uses `go-pdf/fpdf`. **All strings must be ASCII/Latin-1** — use `Rs.` not `₹`, `-` not `—`. Handler at `GET /invoices/pdf?id=X` determines CGST+SGST vs IGST by comparing buyer GSTIN prefix against `App.StateCode`.

### Real-time Dashboard

`GET /sse/dashboard` streams Server-Sent Events. The dashboard template connects via `hx-ext="sse"` and swaps named SSE events into specific DOM elements (`#sse-products`, `#sse-invoices`, etc.).

### Auto Status Badges

JS in `base.html` scans every `<td>` after load and HTMX swaps. If the cell text matches a word in `STATUS_MAP` (paid, pending, overdue, draft, active, etc.), it wraps it in `<span class="badge badge--{status}">`. To support a new status word, add it to `STATUS_MAP` in `base.html` only.
