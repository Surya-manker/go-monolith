# InvoBill

A production-ready GST billing and inventory platform for Indian SMBs — built with Go and HTMX, no JavaScript frameworks, instant server-side rendering.

---

## Features

- **GST Invoices** — Auto-calculates CGST + SGST (intra-state) or IGST (inter-state) from GSTINs. Generates PDF invoices server-side via `go-pdf/fpdf`.
- **Live Dashboard** — Real-time revenue, stock, and invoice metrics via Server-Sent Events.
- **Inventory Control** — Stock tracking, low-stock thresholds, and full adjustment logs.
- **13 Modules** — Customers, Vendors, Categories, Invoices, Purchase Orders, Payments, Credit Notes, Accounts, Jobs, Users, Stock Logs, Audit Logs, Reports — all with soft-delete / trash / restore.
- **Role-Based Access** — `admin`, `super_admin`, `manager`, `accountant`, `staff` with granular module-level permissions.
- **REST API** — Full JSON API at `/api/v1/*` for all modules.
- **Email Service** — Transactional emails for invoices, welcome, and password reset via SMTP (no-op by default).
- **GST Invoice Generator** — Public client-side tool at `/generator`, no login required.
- **Audit Log** — Every create/update/delete action recorded with user and timestamp.
- **Dark Mode** — Persisted via `localStorage`, applied before first paint to avoid flash.
- **3-step Onboarding** — Business setup wizard for new accounts.
- **Demo Mode** — One-click demo login at `/demo` with pre-seeded data.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25 · stdlib `net/http` |
| Database | MySQL 8+ (`go-sql-driver/mysql`) |
| Frontend | HTMX 2.0 · Vanilla JS |
| Templating | Go `html/template` (SSR, parsed from disk) |
| PDF | `go-pdf/fpdf` |
| Auth | bcrypt · session cookies (MySQL-backed) |
| Email | `net/smtp` (NoopMailer default, SMTPMailer when configured) |
| Fonts | Inter + Plus Jakarta Sans (Google Fonts) |

---

## Prerequisites

- Go 1.25+
- MySQL 8+ running locally
- (Optional) [Air](https://github.com/air-verse/air) for hot reload

---

## Quick Start

```powershell
# 1. Create the database
mysql -u root -e "CREATE DATABASE invobill CHARACTER SET utf8mb4;"

# 2. Run the server
$env:DATABASE_DSN = "root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4"
go run main.go

# 3. Seed demo data (run once; idempotent)
go run ./seed/
```

Open **http://localhost:8080**. The landing page is public. Use seeded credentials or register a new account:

| Role | Email | Password |
|---|---|---|
| Admin | admin@invobill.com | admin123456 |
| Manager | manager@invobill.com | manager123 |
| Accountant | accounts@invobill.com | accounts123 |
| Staff | staff@invobill.com | staff123456 |

Or click **Try Demo** on the homepage to log in instantly.

---

## Environment Variables

### Required

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_DSN` | `root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4` | MySQL DSN — `parseTime=true` required |
| `PORT` | `8080` | HTTP listen port |

### GST / PDF Invoices

| Variable | Default | Purpose |
|---|---|---|
| `GST_SELLER_NAME` | `InvoBill Company` | Business name on PDF invoices |
| `GST_SELLER_GSTIN` | _(empty)_ | Your GSTIN — first 2 digits auto-set state code |
| `GST_SELLER_ADDRESS` | _(empty)_ | Address line on PDF invoices |
| `GST_STATE_CODE` | first 2 chars of GSTIN | Overrides auto-detection |

### Email (all optional — disabled if `SMTP_HOST` is unset)

| Variable | Default | Purpose |
|---|---|---|
| `SMTP_HOST` | _(empty)_ | SMTP server hostname — activates email sending |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USER` | _(empty)_ | SMTP username |
| `SMTP_PASS` | _(empty)_ | SMTP password |
| `SMTP_FROM` | `noreply@invobill.in` | Sender address |

Schema is auto-migrated on every startup — no migration tool needed.

---

## Public Pages

| Route | Description |
|---|---|
| `/` | Full SaaS landing page — hero, pricing, testimonials, FAQ |
| `/features` | Feature deep-dives with interactive illustrations |
| `/pricing` | Pricing plans with monthly/annual toggle + comparison table |
| `/about` | Mission, how-it-works, tech stack |
| `/contact` | Contact form |
| `/generator` | Free client-side GST invoice builder (no login) |
| `/demo` | One-click demo login (requires seeded data) |
| `/register` | Create account |
| `/login` | Sign in |
| `/robots.txt` | SEO crawl rules |
| `/sitemap.xml` | Auto-generated XML sitemap |

---

## App Routes (require login)

| Route | Description |
|---|---|
| `/dashboard` | Live real-time metrics |
| `/setup` | First-time onboarding wizard (3 steps) |
| `/products` | Inventory with stock adjustment |
| `/invoices` | GST invoices + PDF download |
| `/invoices/pdf?id=X` | Serve invoice PDF |
| `/customers` · `/vendors` | CRM records |
| `/payments` · `/credit-notes` | Payment tracking |
| `/purchase-orders` | Purchase order management |
| `/accounts` · `/reports` | Finance and reporting |
| `/audit-logs` | Full action history |
| `/profile` | User profile |
| `/admin/users` | Admin: user management and password reset |

---

## Development

```powershell
# Hot reload — restarts on .go changes; templates reload automatically
go install github.com/air-verse/air@latest
air

# Build production binary
go build -o ./tmp/app.exe .

# Tidy dependencies
go mod tidy
```

HTML/CSS changes in `templates/` and `static/` are live without restart. Only `.go` changes need a rebuild.

---

## Architecture

```
main.go            — wires DB, stores, services, mailer, renderer → starts HTTP server
routes/routes.go   — all routes in one file; module CRUD registered via loop
handlers/          — parse request → call service → Renderer.Page / Auth / Landing
  demo_handler.go     — auto-login as demo account
  seo_handler.go      — /robots.txt and /sitemap.xml
  onboarding_handler  — 3-step setup wizard
services/          — business logic
  mailer.go           — Mailer interface: NoopMailer (default) + SMTPMailer
  module_service.go   — drives all 10 generic modules
models/            — raw SQL; each store owns its Migrate()
middleware/        — Security, RateLimiter(60rps), Auth, CSRF, RBAC
templates/
  layouts/            — base.html (app), auth.html (login), landing.html (public)
  pages/              — one file per route
  partials/           — HTMX fragments
static/css/app.css — indigo/violet design system, animations, all component styles
seed/main.go       — idempotent seeder: 4 users, 6 vendors, 8 customers, 18 products, invoices
```

**Generic module engine** (`models/business.go` + `services/module_service.go`) — adding a new module requires one `ModuleConfig` entry in `NewModuleService()`, zero other changes.

---

## Pricing

| Plan | Price | Limits |
|---|---|---|
| Free | ₹0/month | 50 invoices, 100 products, 1 user |
| Pro | ₹999/month | Unlimited invoices/products, 5 users, email invoices |
| Business | ₹2,499/month | Unlimited users, API access, dedicated support |

---

## License

MIT
