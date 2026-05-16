# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```powershell
# Run the server (PowerShell)
$env:DATABASE_DSN = "root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4"
go run main.go

# Seed the database (admin + staff + sample data; idempotent)
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
| `SMTP_HOST` | _(empty)_ | SMTP server — if unset, mailer is a no-op |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USER` | _(empty)_ | SMTP username |
| `SMTP_PASS` | _(empty)_ | SMTP password |
| `SMTP_FROM` | `noreply@invobill.in` | Sender address on outgoing emails |
| `CONTACT_EMAIL` | value of `SMTP_FROM`, else `admin@invobill.in` | Inbox that receives contact form submissions |

## Architecture

**Stack:** Go stdlib `net/http` · MySQL (`go-sql-driver/mysql`) · `html/template` · HTMX 2 · `go-pdf/fpdf`

**Request flow:**
```
HTTP → Security → RateLimiter(60rps) → [Auth → CSRF →] routes/routes.go → handlers/ → services/ → models/ → MySQL
```

### Route categories (`routes/routes.go`)

| Category | Auth | CSRF | Examples |
|---|---|---|---|
| SEO | No | No | `GET /robots.txt`, `GET /sitemap.xml` |
| Public pages | No | No | `/`, `/about`, `/features`, `/pricing`, `/contact`, `/generator`, `/demo` |
| Auth pages | No | No | `/login`, `/register`, `/logout` |
| Protected HTML | Yes | Yes | `/dashboard`, `/products`, `/setup`, all module routes |
| REST API `/api/v1/*` | Yes (cookie) | No | JSON responses |

The catch-all `mux.Handle("/", authHandler)` routes everything unmatched to the auth+CSRF-protected mux. **All public pages must be registered on the outer `mux` before this catch-all.**

### Renderer (`handlers/renderer.go`)

Three methods — choose based on layout:

| Method | Layout | Use for |
|---|---|---|
| `Page(w, "page.html", data)` | `layouts/base.html` + sidebar | All authenticated app pages |
| `Auth(w, "page.html", data)` | `layouts/auth.html` | Login / register |
| `Landing(w, "page.html", data)` | `layouts/landing.html` | All public marketing pages |

Templates are parsed from disk on every request — HTML/CSS changes are live without restart. `funcMap()` exposes `initial` (first letter) and `hasPermission` (RBAC) to all templates.

### Public Pages

`layouts/landing.html` provides the shared navbar (Home · Features · Pricing · About · Generator · Contact), mobile hamburger menu, multi-column footer, animation JS, and theme toggle. Page templates only define `{{ define "content" }}`.

| Route | Handler | Notes |
|---|---|---|
| `GET /` | `Home` | Full SaaS conversion page |
| `GET /features` | `FeaturesPage` | Feature deep-dives with HTML/CSS art |
| `GET /pricing` | `PricingPage` | 3-tier pricing + comparison table |
| `GET /about` | `About` | Mission, how-it-works, tech stack |
| `GET /contact` | `ContactPage` / `ContactPost` | Contact form with validation |
| `GET /generator` | `Generator` | Client-side GST invoice builder (JS only) |
| `GET /demo` | `Demo` | Auto-login as admin demo account |
| `GET /setup` | `OnboardingPage` | 3-step setup wizard (protected) |

### Mailer (`services/mailer.go`)

`Mailer` interface with two implementations:
- **`NoopMailer`** — default when `SMTP_HOST` is unset; silently discards all mail.
- **`SMTPMailer`** — activated by setting `SMTP_HOST`; sends HTML emails via `net/smtp`.

Methods: `SendInvoice`, `SendWelcome`, `SendPasswordReset`. `App.Mailer` is wired in `main.go`.

### Generic Module System

Ten entities are driven by a single engine with no per-module route or template code:

- **Schema**: `models/business.go` — `ModuleStore` implements `List`, `Get`, `Create`, `Update`, `Delete`, `Trash`, `Restore`, `HardDelete` for any table.
- **Config**: `services/module_service.go:NewModuleService()` — a `[]ModuleConfig` slice defines each module's `Key`, `Table`, `Fields`, and `Title`.
- **Routes**: one loop in `routes/routes.go` registers 5 HTML + 3 trash routes per module.
- **Templates**: `templates/pages/crud.html` (full page) + `templates/partials/crud_table.html` (HTMX swap).

**To add a new generic module:** add one `ModuleConfig` entry to `NewModuleService()`. No other changes needed.

**Products are NOT generic** — dedicated handler/service/model because they require stock management, SKU, and threshold fields.

### Demo Mode

`GET /demo` logs the visitor in as `admin@invobill.com` (seeded by `go run ./seed/`) and sets a `demo_mode=1` cookie (non-HttpOnly so JS can read it). Run the seed script before using the demo route.

### Onboarding Wizard (`/setup`)

Three-step form (Business details → GST setup → Invoice preferences). Protected route — requires a valid session. On completion redirects to `/dashboard?welcome=1`. Steps are stateless: each POST advances via `?step=N` query param.

### Animation System

`static/css/app.css` defines CSS keyframes and a scroll-triggered system. Add `data-anim="up|fade|left|right|scale"` to any landing page element. `IntersectionObserver` in `landing.html` adds `.visible` on viewport entry. Use inline `style="transition-delay:.Xs"` for stagger timing.

### CSRF

`middleware/csrf.go` validates POST/PUT/DELETE on all protected HTML routes. Token lives in the `csrf_token` cookie; HTMX injects it via `htmx:configRequest` in `base.html`. Plain HTML `<form method="POST">` on protected routes must include a hidden `_csrf` field — otherwise 403.

### RBAC

`middleware/rbac.go` maps roles to `module:action` pairs. Roles: `super_admin` / `admin` (all), `manager`, `accountant`, `staff`. Template function `hasPermission .User.Role "module" "action"` gates UI elements. Always use `middleware.UserFromContext(r.Context())` — never read identity from request body.

### Toast Notifications

`handlers/crud_handler.go:setToast(w, message, type)` sets `HX-Trigger` before any body write. JS in `base.html` fires `showToast`. Types: `"success"`, `"error"`, `"warning"`, `"info"`.

### PDF Invoice Generation

`services/pdf_service.go:GenerateInvoicePDF()` uses `go-pdf/fpdf`. **All strings must be ASCII/Latin-1** — use `Rs.` not `₹`, `-` not `—`. Handler at `GET /invoices/pdf?id=X` determines CGST+SGST vs IGST by comparing buyer GSTIN prefix against `App.StateCode`.

### Real-time Dashboard

`GET /sse/dashboard` streams Server-Sent Events. Dashboard template connects via `hx-ext="sse"` and swaps named events into `#sse-products`, `#sse-invoices`, etc.

### SEO

`GET /robots.txt` and `GET /sitemap.xml` are served by `handlers/seo_handler.go`. The sitemap lists all public marketing pages with priorities and changefreq. Update `SitemapXML()` when adding new public routes.

### Auto Status Badges

JS in `base.html` scans every `<td>` after load and HTMX swaps. If text matches a word in `STATUS_MAP`, it wraps in `<span class="badge badge--{status}">`. To add a new status, update `STATUS_MAP` in `base.html` only.
