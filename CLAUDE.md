# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```powershell
# Run the server (PowerShell — set env vars first)
$env:DATABASE_DSN = "root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4"
go run main.go

# Seed the database with demo data (admin + staff users + sample records)
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

## Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_DSN` | `root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4` | MySQL connection string — must include `parseTime=true` |
| `PORT` | `8080` | HTTP listen port |
| `GST_SELLER_NAME` | `InvoBill Company` | Appears on generated PDF invoices |
| `GST_SELLER_GSTIN` | _(empty)_ | Seller GSTIN — first 2 digits auto-set `GST_STATE_CODE` |
| `GST_SELLER_ADDRESS` | _(empty)_ | Appears on PDF invoices |
| `GST_STATE_CODE` | first 2 chars of GSTIN | Determines CGST+SGST vs IGST on PDFs |

**Database setup:** Create the MySQL database before first run: `CREATE DATABASE invobill CHARACTER SET utf8mb4;`. Schema is auto-migrated on every startup via each store's `Migrate()` method.

## Architecture

**Stack:** Go stdlib `net/http` · MySQL (`go-sql-driver/mysql`) · `html/template` · HTMX · `go-pdf/fpdf` (PDF generation) · bcrypt (passwords)

**Request flow:**
```
HTTP → middleware chain → routes/routes.go → handlers/ → services/ → models/ → MySQL
```

**Middleware chain** (outermost first): `Security → RateLimiter(60rps) → Auth → CSRF → handler`

- Auth routes (`/login`, `/register`, `/logout`) skip CSRF and Auth.
- `/api/v1/*` routes skip CSRF but require Auth (session cookie).
- Admin-only routes (`/admin/users/*`) additionally wrap with `middleware.RequireRole("admin", "super_admin")`.

### Layer responsibilities

| Layer | Location | Notes |
|---|---|---|
| Router | `routes/routes.go` | All routes in one file. Module routes registered via loop. |
| Handlers | `handlers/` | Parse request → call service → render. Never write `c.JSON` directly. |
| Services | `services/` | Business logic and validation. |
| Models | `models/` | Raw SQL queries. Each store owns its `Migrate()`. |
| Templates | `templates/` | `layouts/` (base shell) · `pages/` (full pages) · `partials/` (HTMX fragments) |
| Seed | `seed/main.go` | Standalone binary — safe to re-run (idempotent). |

### Generic Module System

Ten entities (customers, categories, vendors, invoices, purchase-orders, users, payments, credit-notes, jobs, accounts) are driven by a single generic engine:

- **Schema**: `models/business.go` — `ModuleStore` with shared `List`, `Get`, `Create`, `Update`, `Delete`, `Trash`, `Restore`, `HardDelete`.
- **Config**: `services/module_service.go:NewModuleService()` — `[]ModuleConfig` slice defines each module's `Key`, `Table`, `Fields`, `Title`.
- **Routes**: `routes/routes.go` — one loop registers all 5 HTML routes + 3 trash routes per module automatically.
- **Templates**: `templates/pages/crud.html` (full page) + `templates/partials/crud_table.html` (HTMX swap).

**To add a new generic module:** add a `ModuleConfig` to the slice in `NewModuleService()` — no route or template changes needed.

**Products are NOT generic** — they have dedicated handlers (`handlers/product_handler.go`), service (`services/product_service.go`), and model (`models/product.go`) because they require stock management and richer types.

### Renderer

`handlers/renderer.go` has three methods:
- `Page(w, "page.html", data)` — full page: loads `layouts/base.html` + `partials/header.html` + `partials/footer.html` + `pages/<page>`
- `Auth(w, "page.html", data)` — standalone: loads `layouts/auth.html` + `pages/<page>`
- `Partial(w, "partial.html", data)` — HTMX fragment from `partials/<partial>`

Templates are parsed from disk on every request (no caching) — template changes are live without restart.

### CSRF

`middleware/csrf.go` validates POST/PUT/DELETE requests. Token is stored in a cookie (`csrf_token`). HTMX injects it automatically via the `htmx:configRequest` listener in `base.html`. **Regular HTML forms must use HTMX (`hx-post`) or include a hidden `_csrf` field** — plain `method="POST"` without HTMX will fail with 403.

### Toast Notifications

`handlers/crud_handler.go:setToast(w, message, type)` sets the `HX-Trigger` response header. The `showToast` JS listener in `base.html` catches it and renders the toast. **Call `setToast` before any write to `w`** (headers must be set before body).

Types: `"success"` (green), `"error"` (red), `"warning"` (amber), `"info"` (blue).

### RBAC

`middleware/rbac.go` defines role permissions. Template function `hasPermission .User.Role "module" "action"` is available in all templates (registered in `renderer.go:funcMap()`).

Roles: `admin` / `super_admin` (full access) · `manager` · `accountant` · `staff` (most restricted).

The sidebar in `templates/partials/header.html` conditionally shows links using `{{ if hasPermission .User.Role "module" "view" }}`.

### PDF Invoice Generation

`services/pdf_service.go:GenerateInvoicePDF(InvoiceForPDF)` uses `go-pdf/fpdf`. **All strings passed to fpdf must be ASCII/Latin-1** — the built-in Helvetica font does not support Unicode. Use `Rs.` not `₹`, `-` not `—`.

Handler: `handlers/invoice_pdf_handler.go` at `GET /invoices/pdf?id=X`. Looks up the invoice, finds the matching customer by name, determines CGST+SGST vs IGST by comparing the buyer's GSTIN prefix against `App.StateCode`.

### Auto Status Badges

JavaScript in `base.html` scans every `<td>` after page load and HTMX swaps. If the cell's text matches a known status word (paid, pending, overdue, draft, active, inactive, etc.) it wraps it in a `<span class="badge badge--{status}">`. No server-side template change needed for new status words — just add a mapping to `STATUS_MAP` in `base.html`.

### Auth / Sessions

- Sessions stored in MySQL `sessions` table. Session token: 64-char hex (32 random bytes).
- `middleware/auth.go:UserFromContext(ctx)` retrieves the authenticated user from request context.
- In handlers, always use `middleware.UserFromContext(r.Context())` — never read identity from the request body.
- Admin can reset any user's password without knowing the old one via `POST /admin/users/update`.
