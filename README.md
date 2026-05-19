# InvoBill ERP

A production-ready GST billing and inventory ERP platform for Indian SMBs — built with Go and HTMX, no JavaScript frameworks, instant server-side rendering.

---

## Features

### Billing & Invoicing
- **GST Invoices** — Auto CGST+SGST (intra-state) or IGST (inter-state) from GSTINs. PDF generation server-side via `go-pdf/fpdf`. Sequential auto-numbering (`INV-2025-0001`).
- **Credit Notes** — Issue credit notes for returns, auto-numbered (`CN-2025-0001`).
- **Payments** — Record and track payments against invoices with multiple payment methods.
- **Email Invoices** — Send PDF invoices directly to customers via SMTP (Pro+).

### Point of Sale
- **POS Terminal** — Fast retail billing with barcode scan, cart management, discount, multiple payment modes, and receipt printing.
- **Sales History** — Full POS transaction log with filters and daily totals.

### Inventory
- **Products** — Stock tracking, SKU, HSN code, barcode, low-stock thresholds, and full adjustment logs.
- **Multi-Warehouse** — Multiple locations, inter-warehouse stock transfers.
- **Batch & Expiry Tracking** — FEFO-tracked inventory for food and pharma. Expiry alerts on dashboard.
- **Barcodes** — Code128 / EAN-13 / QR generation and printable label sheets.
- **Returns** — Sales and purchase returns with batch-level stock restoration.

### CRM & Sales Pipeline
- **Customers** — Full profiles with GSTIN, credit limits, payment terms, auto-code (`CUST-XXXX`).
- **Quotations** — Create quotes, send to customers, convert to sales orders.
- **Sales Orders** — Draft → Confirmed → Packed → Delivered workflow with stock reservation.
- **Deliveries** — Delivery challans linked to sales orders.
- **Customer Payments** — Collect and track payments against orders.

### Procurement
- **Suppliers** — Profiles with GSTIN, credit limits, payment terms, auto-code (`SUP-XXXX`).
- **Purchase Orders** — Draft → Approved → Received workflow, auto-numbered (`PO-2025-0001`).
- **GRN** — Goods Receipt Note with batch and lot tracking, links back to POs.
- **Supplier Payments** — Record payments against POs.
- **Reorder Suggestions** — Auto-flags products below threshold with last supplier and price.

### Finance
- **Expense Tracking** — Categories, approval workflow (`pending → approved → rejected`), CSV export.
- **Bank Accounts** — Multi-account with debit/credit transactions and running balance.
- **Cash Ledger** — Cash-in / cash-out view.
- **Profit & Loss** — Revenue (POS + sales orders) vs expenses and COGS.
- **Cashflow** — Monthly cash-in / cash-out / net cashflow statement.
- **GST Summary** — Output tax (sales) vs input tax (purchases) with monthly breakdown.

### Reports & Analytics
Stock Valuation · Warehouse Inventory · Stock Movement · Dead Stock · Low Stock · Sales Analytics (daily + product + payment method breakdown) · Returns Analytics — all exportable to CSV and PDF.

### Platform
- **Live Dashboard** — Real-time KPI cards, revenue sparkline chart, batch expiry alerts, low-stock list, top customers, quick-action grid — all via Server-Sent Events (no polling).
- **Role-Based Access** — `super_admin`, `admin`, `manager`, `warehouse_manager`, `accountant`, `staff` with granular module-level permissions.
- **Audit Log** — Every create/update/delete recorded with user, module, action, and timestamp.
- **REST API** — Full JSON API at `/api/v1/*` for all core modules.
- **Dark Mode** — Persisted via `localStorage`, applied before first paint.
- **PWA** — Installable app, offline shell, service worker asset caching.
- **Docker** — Multi-service compose (MySQL 8 + app + on-demand backup).
- **Backup** — `scripts/backup.sh` — scheduled MySQL dumps with automatic rotation.
- **Health Probes** — `GET /health` (liveness) and `GET /health/ready` (readiness, checks DB).
- **Structured Logging** — Every request logged with method, path, status, duration, and `X-Request-Id`.
- **Panic Recovery** — Middleware catches panics, logs stack trace, returns 500.

---

## Auto-Generated Numbers

Users never need to type document numbers — all are generated automatically:

| Document | Format | Example |
|---|---|---|
| Invoice | `INV-YYYY-XXXX` | `INV-2025-0007` |
| Credit Note | `CN-YYYY-XXXX` | `CN-2025-0001` |
| Procurement PO | `PO-YYYYMM-XXXX` | `PO-202506-0003` |
| Quotation | `QT-YYYYMM-XXXX` | `QT-202506-0005` |
| Sales Order | `SO-YYYYMM-XXXX` | `SO-202506-0012` |
| POS Sale | `POS-XXXX` | `POS-0148` |
| GRN | auto | `GRN-202506-0002` |
| Returns | `SAL-RET-XXXX` / `PUR-RET-XXXX` | auto |
| Batch Number | `BATCH-YYYY-XXXX` | `BATCH-2025-0023` |
| Supplier Code | `SUP-XXXX` | `SUP-0005` |
| Customer Code | `CUST-XXXX` | `CUST-0012` |
| Payments | `CPAY-*` / `PAY-*` | auto |

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25 · stdlib `net/http` |
| Database | MySQL 8+ (`go-sql-driver/mysql`) |
| Frontend | HTMX 2.0 · Vanilla JS · Chart.js 4 (reports & dashboard) |
| Templating | Go `html/template` (SSR, hot-reload from disk) |
| PDF | `go-pdf/fpdf` |
| Auth | bcrypt · session cookies (MySQL-backed) |
| Email | `net/smtp` (NoopMailer default) |
| Config | `godotenv` — loads `app.env` on startup |
| PWA | Service Worker (`static/sw.js`) + Web App Manifest |

---

## Prerequisites

- Go 1.25+
- MySQL 8+ running locally
- (Optional) [Air](https://github.com/air-verse/air) for hot reload
- (Optional) Docker + Docker Compose for containerised setup

---

## Quick Start

### Local

```powershell
# 1. Create the database
mysql -u root -e "CREATE DATABASE invobill CHARACTER SET utf8mb4;"

# 2. Copy config and adjust if needed
cp app.env.example app.env

# 3. Run the server
go run main.go

# 4. Seed 90 days of demo data (idempotent — safe to run again)
go run ./seed/
```

Open **http://localhost:8080**. Use demo credentials or register a new account.

### Docker

```bash
# Start MySQL + app together
docker-compose up -d

# Run seed locally pointing at the container
DATABASE_DSN="invobill:invobill_pass@tcp(localhost:3306)/invobill?parseTime=true&charset=utf8mb4" go run ./seed/
```

---

## Demo Credentials

Run `go run ./seed/` first, then log in at `/login` or click **Try Demo** on the homepage:

| Role | Email | Password |
|---|---|---|
| Admin | admin@invobill.com | admin123456 |
| Manager | manager@invobill.com | manager123456 |
| Accountant | accounts@invobill.com | accounts123456 |
| Staff | staff@invobill.com | staff123456 |

The seed creates **Mehta Electronics & General Store** with **90 days** of realistic data:
- 25 products (electronics, food, pharma, office supplies, clothing, tools)
- 3 warehouses with distributed stock and transfers
- 15 batches with realistic expiry dates (including 2 already expired — for alerts)
- 5 suppliers + 8 procurement orders + 6 supplier payments
- 8 CRM customers + 10 sales orders + 6 deliveries + 7 CRM payments
- ~150 POS sales across 45 days
- 20 expenses across 8 categories + 3 bank accounts + 15 transactions
- 12 invoices (paid/pending/overdue/draft) + 8 notifications

---

## Environment Variables

Copy `app.env.example` → `app.env`. Key variables:

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_DSN` | `root:@tcp(127.0.0.1:3306)/invobill?parseTime=true&charset=utf8mb4` | Must include `parseTime=true` |
| `PORT` | `8080` | HTTP listen port |
| `HTTPS` | `false` | Set `true` behind TLS — enables HSTS header |
| `GST_SELLER_NAME` | `InvoBill Company` | Business name on PDF invoices |
| `GST_SELLER_GSTIN` | _(empty)_ | Your GSTIN — first 2 digits auto-set state code |
| `GST_SELLER_ADDRESS` | _(empty)_ | Address on PDF invoices |
| `SMTP_HOST` | _(empty)_ | SMTP server — if unset, all email is silently discarded |
| `SMTP_FROM` | `noreply@invobill.in` | Sender address |
| `CONTACT_EMAIL` | `SMTP_FROM` | Inbox for contact form submissions |
| `BACKUP_DIR` | `./backups` | Where `scripts/backup.sh` writes dumps |
| `BACKUP_KEEP` | `7` | Days of backups to retain |

Schema is **auto-migrated on every startup** via each store's `Migrate()` — no migration tool needed.

---

## App Routes

### Public
| Route | Description |
|---|---|
| `/` | Landing page — features, pricing, testimonials, FAQ, 40+ module chips |
| `/features` | Complete ERP platform overview — deep-dive per module |
| `/pricing` | 3-tier pricing with full feature comparison table |
| `/about` | Mission, how-it-works, tech stack |
| `/contact` | Contact form |
| `/generator` | Free client-side GST invoice builder (no login required) |
| `/demo` | One-click demo login (run seed first) |
| `/health` | Liveness probe — always 200 |
| `/health/ready` | Readiness probe — 503 if DB unreachable |

### Protected (require login)
| Route | Description |
|---|---|
| `/dashboard` | Live KPI metrics, revenue sparkline, expiry alerts, quick actions |
| `/setup` | First-time onboarding wizard (3 steps) |
| `/checklist` | Post-setup 8-item progress checklist |
| `/products` | Product catalogue with stock management |
| `/warehouses` · `/warehouses/stock` | Multi-location inventory |
| `/transfers` | Inter-warehouse stock transfers |
| `/batches` · `/batches/expiry` | Batch tracking and expiry report |
| `/barcodes` · `/barcodes/labels` | Barcode generation and label printing |
| `/returns` · `/returns/sales` · `/returns/purchase` | Returns management |
| `/pos` · `/pos/sales` | POS terminal and sales history |
| `/crm` · `/crm/customers` · `/crm/quotations` | CRM dashboard and pipeline |
| `/crm/orders` · `/crm/delivery` · `/crm/payments` | Orders, deliveries, collections |
| `/procurement` · `/suppliers` · `/procurement/orders` | Procurement dashboard |
| `/procurement/grn` · `/procurement/payments` · `/procurement/reorder` | GRN, payments, reorder |
| `/finance` · `/finance/expenses` · `/finance/bank` | Finance dashboard, expenses, bank |
| `/finance/pl` · `/finance/cashflow` · `/finance/gst` · `/finance/ledger` | Financial statements |
| `/reports` | Reports hub with interactive charts |
| `/reports/sales` · `/reports/stock-valuation` · `/reports/stock-movement` | Sales and stock reports |
| `/reports/dead-stock` · `/reports/low-stock` · `/reports/returns` | Inventory health reports |
| `/invoices` · `/invoices/pdf?id=X` | GST invoices and PDF download |
| `/customers` · `/vendors` · `/payments` · `/credit-notes` | Generic CRUD modules |
| `/audit-logs` · `/stock-logs` | Activity history |
| `/admin/users` | User management (admin only) |
| `/profile` | User profile and password change |

---

## Backup

```bash
# One-time backup (reads DATABASE_DSN from app.env automatically)
./scripts/backup.sh

# Schedule daily at 2 AM via cron
0 2 * * * /path/to/go-monolith/scripts/backup.sh >> /var/log/invobill-backup.log 2>&1

# Restore from a backup file
./scripts/restore.sh ./backups/invobill_20250519_020000.sql.gz

# Docker: on-demand backup
docker-compose --profile backup run --rm backup
```

See `scripts/README.md` for full options.

---

## Development

```powershell
# Hot reload — restarts on .go changes; templates reload without restart
air

# Build production binary
go build -o ./tmp/app.exe .

# Static analysis
go build ./...
go vet ./...

# Tidy dependencies
go mod tidy
```

HTML/CSS changes in `templates/` and `static/` are live without restart — the renderer re-reads files on every request.

---

## Architecture

```
main.go              — wires DB → stores → services → renderer → HTTP server
routes/routes.go     — all routes in one file; module CRUD registered via loop
middleware/          — Recovery → RequestID → Logger → Security → RateLimit → Auth → CSRF
handlers/            — parse request → call service → Renderer.Page/Auth/Landing
  home_handler.go    — supplies 40+ module names to landing page
  onboarding_handler — 3-step setup wizard + 8-item checklist
  health_handler     — /health and /health/ready probes
services/            — business logic (one file per domain)
  module_service.go  — generic CRUD + auto-number generation for invoices/POs/CNs
models/              — raw SQL; each store owns its Migrate() + auto-number functions
  demo_store.go      — in-memory store for /demo sessions
templates/
  layouts/           — base.html (app sidebar), auth.html, landing.html (public)
  pages/             — one file per route; sub-templates defined inline
  partials/          — HTMX swap fragments (pos_cart.html, products_table.html, etc.)
static/
  css/app.css        — indigo/violet design system, dark mode tokens, all components
  sw.js              — PWA service worker
  manifest.json      — PWA manifest
scripts/             — backup.sh, restore.sh, backup-rotate.sh
seed/main.go         — idempotent seeder: full business with 90 days of history
```

**Generic module engine** — `models/business.go` (`ModuleStore`) + `services/module_service.go` (`ModuleConfig`) drive 10 CRUD modules with soft-delete/trash/restore. Adding a new module requires one `ModuleConfig` entry — zero route or template changes.

**Auto-numbering** — `ModuleService.Create()` auto-generates `number` field for invoices (`INV-YYYY-XXXX`), purchase-orders (`PO-YYYY-XXXX`), and credit-notes (`CN-YYYY-XXXX`) when left blank. Specialist stores (procurement, CRM, POS, batches) generate their own numbers via `Next*Number()` methods.

**Demo isolation** — `GET /demo` binds the session token to an in-memory `DemoStore`. All CRUD for that session hits in-memory data only; a normal login always uses MySQL.

---

## Pricing

| Plan | Price | Highlights |
|---|---|---|
| Free | ₹0/month | 50 invoices · 100 products · 1 user · GST PDF · POS · Barcodes |
| Pro | ₹999/month | Unlimited · 5 users · CRM · Procurement · Finance · Batch tracking · Email invoices |
| Business | ₹2,499/month | Unlimited users · REST API · Dedicated manager · SLA · White-label invoices |

Prices exclude 18% GST. Annual billing saves 20%.

---

## License

MIT
