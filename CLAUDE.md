# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```powershell
# Run the server
go run main.go

# Seed the database — idempotent, creates demo business + 90 days of data
go run ./seed/

# Build binary
go build -o ./tmp/app.exe .

# Hot reload (requires Air)
air

# Tidy dependencies
go mod tidy

# Build check (no tests exist)
go build ./...
go vet ./...
```

**Database setup:** `CREATE DATABASE invobill CHARACTER SET utf8mb4;` — schema auto-migrates on startup via each store's `Migrate()` called in dependency order in `main.go`.

Config is loaded from `app.env` (copy from `app.env.example`). Go version: **1.25**.

## Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_DSN` | `root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4` | Must include `parseTime=true` |
| `PORT` | `8080` | HTTP listen port |
| `HTTPS` | `false` | Set `true` behind TLS to enable HSTS header |
| `GST_SELLER_NAME` | `InvoBill Company` | Appears on PDF invoices |
| `GST_SELLER_GSTIN` | _(empty)_ | First 2 digits auto-set `GST_STATE_CODE` |
| `GST_SELLER_ADDRESS` | _(empty)_ | Appears on PDF invoices |
| `SMTP_HOST` | _(empty)_ | If unset, mailer is a no-op |
| `SMTP_FROM` | `noreply@invobill.in` | Sender address |
| `CONTACT_EMAIL` | `SMTP_FROM` | Receives contact form submissions |
| `BACKUP_DIR` | `./backups` | Used by `scripts/backup.sh` |
| `BACKUP_KEEP` | `7` | Days of backups to retain |

## Architecture

**Stack:** Go stdlib `net/http` · MySQL (`go-sql-driver/mysql`) · `html/template` · HTMX 2 · `go-pdf/fpdf`

**Middleware stack (outermost → innermost):**
```
Recovery → RequestID → Logger → Security → RateLimit(60rps) → Auth → CSRF → handler
```
`middleware/recovery.go` catches panics. `middleware/logger.go` logs every request with duration and a random `X-Request-Id`. `middleware/security.go` sets hardened headers and enforces a 10 MB body limit.

**Request flow:**
```
HTTP → middleware chain → routes/routes.go → handlers/ → services/ → models/ → MySQL
```

### Route categories (`routes/routes.go`)

| Category | Auth | CSRF | Examples |
|---|---|---|---|
| Health | No | No | `GET /health`, `GET /health/ready` |
| SEO | No | No | `GET /robots.txt`, `GET /sitemap.xml` |
| Public pages | No | No | `/`, `/about`, `/features`, `/pricing`, `/contact`, `/demo` |
| Auth pages | No | No | `/login`, `/register`, `/logout` |
| Protected HTML | Yes | Yes | `/dashboard`, `/products`, `/setup`, all module routes |
| REST API `/api/v1/*` | Yes (cookie) | No | JSON responses |

The catch-all `mux.Handle("/", authHandler)` routes everything unmatched to the auth+CSRF-protected mux. **All public pages must be registered on the outer `mux` before this catch-all.**

### Renderer (`handlers/renderer.go`)

Four methods — choose based on layout:

| Method | Layout | Use for |
|---|---|---|
| `Page(w, "page.html", data)` | `layouts/base.html` + sidebar | All authenticated app pages |
| `PageWith(w, "page.html", data, "partial.html", ...)` | Same + extra partials | Pages that call `{{ template "X" }}` where X is defined in a separate partial file |
| `Auth(w, "page.html", data)` | `layouts/auth.html` | Login / register |
| `Landing(w, "page.html", data)` | `layouts/landing.html` | Public marketing pages |

**Critical sub-template rule:** Every page template is self-contained — sub-templates it calls must be defined **inline** in the same page file using `{{ define "..." }}` blocks, OR the handler must use `PageWith` with the partial file listed explicitly. `Partial(w, "file.html", data)` is only for HTMX swap responses. The only current exception is `pos.html`, which uses `PageWith(..., "pos_cart.html")`.

Templates are parsed from disk on every request — HTML/CSS changes are live without restart.

### Template FuncMap (`handlers/renderer.go:funcMap()`)

All templates have access to: `initial`, `hasPermission`, `navActive`, `orStr`, `upper`, `lower`, `now`, `today`, `jsonSafe`, `dateOnly`, `seq`, `monthName`, `sub`, `add`, `sub1`, `add1`, `absf`, `mul`, `div`, `float64`, `int`, `pct`, `remaining`.

- `navActive .Path "/route"` → returns `"sidebar-link--active"` or `""` (server-side active link detection)
- `dateOnly` accepts both `time.Time` and `string`, returns `"YYYY-MM-DD"`
- `today` returns today's date as `"YYYY-MM-DD"`
- `float64` converts `int` → `float64` for template arithmetic (needed in report templates)
- External JS libraries (e.g. Chart.js) must be `<script src="...">` **before** any inline script that uses them

### AppContext (`handlers/context.go`)

Embedded in every page data struct. Contains `User *models.User` and `Path string` (set from `r.URL.Path`). `Path` powers `navActive` in `header.html`. Always call `a.ctx(r)` to populate it.

### Sidebar Active Links (`templates/partials/header.html`)

Active state is set **server-side** at render time using `{{ navActive .Path "/route" }}` on every `sidebar-link`. There is no JavaScript active-link detection. The `navActive` function handles: exact path match, prefix+slash match (`/finance` active on `/finance/expenses`), and the `/dashboard` special case (active on both `/` and `/dashboard`). Query strings in hrefs are stripped before comparison.

### Auto-Generated Numbers

All document numbers are generated server-side — users never need to type them:

| Entity | Format | Generated in |
|---|---|---|
| Invoice | `INV-YYYY-XXXX` | `services/module_service.go:Create()` |
| Purchase Order (generic) | `PO-YYYY-XXXX` | `services/module_service.go:Create()` |
| Credit Note | `CN-YYYY-XXXX` | `services/module_service.go:Create()` |
| Procurement PO | `PO-YYYYMM-XXXX` | `models/procurement.go:NextPONumber()` |
| GRN | `GRN-YYYYMM-XXXX` | `models/procurement.go:NextGRNNumber()` |
| Quotation | `QT-YYYYMM-XXXX` | `models/crm.go:NextQuoteNumber()` |
| Sales Order | `SO-YYYYMM-XXXX` | `models/crm.go:NextOrderNumber()` |
| Delivery | `DEL-YYYYMM-XXXX` | `models/crm.go` |
| POS Sale | `POS-XXXX` | `models/pos.go:NextSaleNumber()` |
| Sales Return | `SAL-RET-XXXX` | `models/batch.go:NextReturnNumber()` |
| Purchase Return | `PUR-RET-XXXX` | `models/batch.go:NextReturnNumber()` |
| CRM Payment | `CPAY-YYYYMM-XXXX` | `models/crm.go:CreatePayment()` |
| Supplier Payment | `PAY-YYYYMM-XXXX` | `models/procurement.go:CreatePayment()` |
| Batch Number | `BATCH-YYYY-XXXX` | `models/batch.go:Create()` |
| Supplier Code | `SUP-XXXX` | `models/procurement.go:CreateSupplier()` |
| Customer Code | `CUST-XXXX` | `models/crm.go:CreateCustomer()` |

The `number` field is hidden from the create form in `crud.html` — it only shows in the edit form. Supplier Code and Customer Code fields are not shown in the create forms at all. Batch Number is sent as a hidden empty field; Lot Number is the only optional user input.

### Generic Module System

Ten entities driven by a single engine: customers, categories, vendors, invoices, purchase-orders, users, payments, credit-notes, jobs.

- **Schema**: `models/business.go` — `ModuleStore` implements generic CRUD for any table
- **Config**: `services/module_service.go:NewModuleService()` — `[]ModuleConfig` defines `Key`, `Table`, `Fields`, `Title`
- **Routes**: one loop in `routes/routes.go` registers 5 HTML + 3 trash routes per module
- **Templates**: `templates/pages/crud.html` defines `crud-form-create`, `crud-table`, `pagination` inline
- **Auto-number**: `ModuleService.Create()` auto-generates `number` for invoices, purchase-orders, credit-notes if blank

**To add a new generic module:** add one `ModuleConfig` entry to `NewModuleService()`. No other changes needed.

**Products are NOT generic** — dedicated handler/service/model for stock management, SKU, barcode, and threshold fields.

### RBAC (`middleware/rbac.go`)

Roles: `super_admin` / `admin` (all), `manager`, `warehouse_manager`, `accountant`, `staff`. Template function `hasPermission .User.Role "module" "action"` gates UI elements. Always use `middleware.UserFromContext(r.Context())` — never read identity from request body.

### CSRF (`middleware/csrf.go`)

Validates POST/PUT/DELETE on all protected HTML routes. Token lives in the `csrf_token` cookie; HTMX injects it via `htmx:configRequest` in `base.html`. Plain HTML `<form method="POST">` on protected routes must include a hidden `_csrf` field — otherwise 403.

### Demo Mode

`GET /demo` logs the visitor in as `admin@invobill.com` and binds their session token to an in-memory `DemoStore` (`models/demo_store.go`). All CRUD for that session hits the in-memory store — the real DB is never touched. `go run ./seed/` must be run first. The `DemoSessionManager` is keyed by auth session token; a normal login always gets a different token so it uses real MySQL.

### Toast Notifications

`handlers/crud_handler.go:setToast(w, message, type)` sets `HX-Trigger` before any body write. JS in `base.html` fires `showToast`. Types: `"success"`, `"error"`, `"warning"`, `"info"`.

### PDF Invoice Generation

`services/pdf_service.go:GenerateInvoicePDF()` uses `go-pdf/fpdf`. **All strings must be ASCII/Latin-1** — use `Rs.` not `₹`, `-` not `—`. Handler at `GET /invoices/pdf?id=X` determines CGST+SGST vs IGST by comparing buyer GSTIN prefix against `App.StateCode`.

### Onboarding (`handlers/onboarding_handler.go`)

Three-step wizard (`/setup`): Business details → GST setup → Invoice preferences. Each step POST saves to the `businesses` table. On completion redirects to `/dashboard?welcome=1` which shows a one-time welcome modal (dismissed via `localStorage`). `/checklist` shows an 8-item setup checklist that checks live DB state.

### Health Endpoints

- `GET /health` — always 200, returns uptime + Go version (liveness probe)
- `GET /health/ready` — pings MySQL; returns 503 if unreachable (readiness probe)

### Backup (`scripts/`)

`scripts/backup.sh` reads `DATABASE_DSN` from `app.env`, runs `mysqldump`, gzips to `./backups/`, and prunes files older than `BACKUP_KEEP` days (default 7). `scripts/restore.sh` prompts before dropping the DB. See `scripts/README.md` for cron setup.

### Docker

`docker-compose.yml` includes MySQL 8 (health-checked) + app (waits on MySQL) + an on-demand backup service (`--profile backup`). All env vars flow from a `.env` file using `${VAR:-default}` syntax. The app's `HEALTHCHECK` calls `GET /health`.

### PWA

`static/manifest.json` + `static/sw.js` make the app installable. The service worker caches `/static/` assets (cache-first) and HTML navigation (network-first with inline offline shell). Registered in `base.html` via `navigator.serviceWorker.register('/static/sw.js')`.

### Real-time Dashboard

`GET /sse/dashboard` streams Server-Sent Events. Dashboard template connects via `hx-ext="sse"`. Dashboard handler fans out ~14 DB queries concurrently using `sync.WaitGroup` goroutines.

### GST Logic

`intra_state` is true when the first 2 digits of the customer's GSTIN match `App.StateCode`. Intra-state → CGST + SGST split equally; inter-state → IGST only.

### Auto Status Badges

JS in `base.html` scans every `<td>` after load and HTMX swaps. If text matches a word in `STATUS_MAP`, it wraps in `<span class="badge badge--{status}">`. To add a new status, update `STATUS_MAP` in `base.html` only.

### Public Website (`templates/pages/home.html`, `features.html`, `pricing.html`, `about.html`)

Public pages use `layouts/landing.html` and the `Landing` renderer. No tally or competitor references anywhere — all marketing copy is InvoBill-specific. `handlers/home_handler.go` supplies the `Modules []string` list (40+ items) shown in the home page modules grid. When adding new features, update this list to keep the landing page current. The pricing comparison table in `pricing.html` covers all module categories: Invoicing, Inventory & Warehouse, POS & Sales, CRM, Procurement, Finance, Team & Access, Platform.
