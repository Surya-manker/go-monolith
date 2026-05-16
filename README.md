# InvoBill

A full-stack GST-compliant inventory and billing platform for Indian SMBs, built with Go and HTMX — no JavaScript frameworks, instant page loads.

---

## Features

- **GST Invoices** — Auto-calculates CGST + SGST (intra-state) or IGST (inter-state) from GSTINs. Generates PDF invoices server-side.
- **Live Dashboard** — Real-time metrics via Server-Sent Events (revenue, stock alerts, pending invoices, top customers).
- **Inventory Control** — Product catalogue with stock tracking, low-stock thresholds, and adjustment logs.
- **10+ Modules** — Customers, Vendors, Categories, Invoices, Purchase Orders, Payments, Credit Notes, Accounts, Jobs, Users — all with soft-delete / trash / restore.
- **Role-Based Access** — `admin`, `super_admin`, `manager`, `accountant`, `staff` roles with granular module-level permissions.
- **REST API** — JSON API at `/api/v1/*` for all modules.
- **GST Invoice Generator** — Public client-side tool at `/generator` (no login required).
- **Audit Log** — Every create / update / delete action is recorded with user and timestamp.
- **Dark Mode** — Persisted via `localStorage`, applied before first paint.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25 · stdlib `net/http` |
| Database | MySQL 8+ (`go-sql-driver/mysql`) |
| Frontend | HTMX 2.0 · Vanilla JS |
| Templating | Go `html/template` |
| PDF | `go-pdf/fpdf` |
| Auth | bcrypt · session cookies (MySQL-backed) |
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

# 2. Set env vars and run
$env:DATABASE_DSN = "root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4"
go run main.go

# 3. Seed demo data (admin + staff users + sample records)
go run ./seed/
```

Open **http://localhost:8080** — the landing page is public. Register or use seeded credentials:

| Role | Email | Password |
|---|---|---|
| Admin | admin@invobill.in | admin123456 |
| Staff | staff@invobill.in | staff123456 |

---

## Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_DSN` | `root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4` | MySQL DSN — `parseTime=true` required |
| `PORT` | `8080` | HTTP listen port |
| `GST_SELLER_NAME` | `InvoBill Company` | Business name on PDF invoices |
| `GST_SELLER_GSTIN` | _(empty)_ | Your GSTIN — first 2 digits set the state code |
| `GST_SELLER_ADDRESS` | _(empty)_ | Address on PDF invoices |
| `GST_STATE_CODE` | first 2 chars of GSTIN | Overrides auto-detection |

Schema is auto-migrated on every startup — no migration tool needed.

---

## Public Pages

| Route | Description |
|---|---|
| `/` | Landing / home page |
| `/about` | About InvoBill |
| `/contact` | Contact form |
| `/generator` | Free client-side GST invoice generator |
| `/register` | Create account |
| `/login` | Sign in |

---

## App Routes (require login)

| Route | Description |
|---|---|
| `/dashboard` | Live metrics dashboard |
| `/products` | Inventory with stock adjustment |
| `/invoices` | GST invoices + PDF download |
| `/invoices/pdf?id=X` | Download invoice PDF |
| `/customers` `/vendors` | CRM records |
| `/payments` `/credit-notes` | Payment tracking |
| `/purchase-orders` | Purchase order management |
| `/accounts` `/reports` | Finance & reporting |
| `/audit-logs` | Full action history |
| `/profile` | User profile |
| `/admin/users` | Admin: user management & password reset |

---

## Development

```powershell
# Hot reload
go install github.com/air-verse/air@latest
air

# Build binary
go build -o ./tmp/app.exe .

# Tidy dependencies
go mod tidy
```

Templates are read from disk on every request — HTML/CSS changes are live without restart. Only `.go` file changes require a rebuild (or Air handles it automatically).

---

## Architecture

```
main.go          — wires DB, stores, services, renderer → starts HTTP server
routes/          — all routes in one file; module CRUD registered via loop
handlers/        — parse request → call service → Renderer.Page/Auth/Landing
services/        — business logic; ModuleService drives 10 generic modules
models/          — raw SQL; each store owns its Migrate()
middleware/      — Security, RateLimiter, Auth, CSRF, RBAC
templates/
  layouts/       — base.html (app), auth.html (login), landing.html (public)
  pages/         — one file per route
  partials/      — HTMX fragments (header, crud_table, search_results, …)
static/css/      — single app.css; indigo/violet design system + animations
seed/            — standalone seeder binary, safe to re-run
```

The **generic module engine** (`models/business.go` + `services/module_service.go`) drives all 10 non-product entities through a shared CRUD template. Adding a new module requires only one `ModuleConfig` entry — no new routes, handlers, or templates.

---

## License

MIT

