package handlers

import (
	"net/http"
	"strings"

	"go-monolith/middleware"
)

type OnboardingData struct {
	AppContext
	Step         int
	Error        string
	Checklist    []ChecklistItem
	AllDone      bool
	DoneCount    int
	TotalCount   int
}

type ChecklistItem struct {
	Key   string
	Label string
	Desc  string
	Done  bool
	Link  string
	Icon  string
}

// OnboardingPage renders the step-by-step first-time setup wizard.
func (a *App) OnboardingPage(w http.ResponseWriter, r *http.Request) {
	step := 1
	switch r.URL.Query().Get("step") {
	case "2":
		step = 2
	case "3":
		step = 3
	}
	a.Renderer.Page(w, "onboarding.html", OnboardingData{
		AppContext: a.ctx(r),
		Step:       step,
	})
}

// OnboardingPost handles each wizard step and persists data to the businesses table.
func (a *App) OnboardingPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}

	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	step := r.FormValue("step")
	bizID := user.BusinessID

	switch step {
	case "1":
		name := strings.TrimSpace(r.FormValue("business_name"))
		email := strings.TrimSpace(r.FormValue("business_email"))
		phone := strings.TrimSpace(r.FormValue("phone"))
		address := strings.TrimSpace(r.FormValue("address"))

		if name == "" {
			a.Renderer.Page(w, "onboarding.html", OnboardingData{
				AppContext: a.ctx(r), Step: 1,
				Error: "Business name is required.",
			})
			return
		}

		// Persist to businesses table if we have a business.
		if bizID > 0 {
			a.BusinessStore.Update(bizID, name, email, phone, "", address, "")
		}
		// Update the user's display name if provided.
		if ownerName := strings.TrimSpace(r.FormValue("owner_name")); ownerName != "" {
			a.AuthService.UpdateName(user.ID, ownerName)
		}
		http.Redirect(w, r, "/setup?step=2", http.StatusFound)

	case "2":
		gstin := strings.TrimSpace(r.FormValue("gstin"))
		if bizID > 0 && gstin != "" {
			stateCode := ""
			if len(gstin) >= 2 {
				stateCode = gstin[:2]
			}
			// Read current biz to keep name/email unchanged.
			biz, _ := a.BusinessStore.GetByID(bizID)
			if biz != nil {
				a.BusinessStore.Update(bizID, biz.Name, biz.Email, biz.Phone, gstin, biz.Address, stateCode)
			}
		}
		http.Redirect(w, r, "/setup?step=3", http.StatusFound)

	case "3":
		http.Redirect(w, r, "/dashboard?welcome=1", http.StatusFound)

	default:
		http.Redirect(w, r, "/setup", http.StatusFound)
	}
}

// ChecklistPage shows the post-setup onboarding checklist.
func (a *App) ChecklistPage(w http.ResponseWriter, r *http.Request) {
	bizID := a.bizID(r)
	checklist := a.buildChecklist(bizID, r)

	done := 0
	for _, item := range checklist {
		if item.Done {
			done++
		}
	}

	a.Renderer.Page(w, "checklist.html", OnboardingData{
		AppContext:  a.ctx(r),
		Checklist:  checklist,
		AllDone:    done == len(checklist),
		DoneCount:  done,
		TotalCount: len(checklist),
	})
}

// buildChecklist checks each onboarding step against live data.
func (a *App) buildChecklist(bizID int, r *http.Request) []ChecklistItem {
	items := []ChecklistItem{
		{
			Key: "business", Label: "Set up your business profile",
			Desc: "Add your business name, address, and GST number",
			Link: "/setup", Icon: "🏢",
		},
		{
			Key: "product", Label: "Add your first product",
			Desc: "Create a product with SKU, price, and stock quantity",
			Link: "/products", Icon: "📦",
		},
		{
			Key: "warehouse", Label: "Create a warehouse",
			Desc: "Set up at least one location to track inventory",
			Link: "/warehouses", Icon: "🏭",
		},
		{
			Key: "supplier", Label: "Add a supplier",
			Desc: "Record a supplier for procurement orders",
			Link: "/suppliers", Icon: "🤝",
		},
		{
			Key: "customer", Label: "Add a customer",
			Desc: "Add your first customer to start creating invoices",
			Link: "/crm/customers", Icon: "👤",
		},
		{
			Key: "invoice", Label: "Create your first invoice",
			Desc: "Generate a professional GST invoice for a customer",
			Link: "/invoices", Icon: "🧾",
		},
		{
			Key: "pos", Label: "Make a POS sale",
			Desc: "Use the Point of Sale terminal for a retail transaction",
			Link: "/pos", Icon: "🖥",
		},
		{
			Key: "bank", Label: "Add a bank account",
			Desc: "Connect a bank account to track payments and transactions",
			Link: "/finance/bank", Icon: "🏦",
		},
	}

	// Check each item against live DB state.
	if biz, err := a.BusinessStore.GetByID(bizID); err == nil && biz != nil && biz.GSTIN != "" {
		items[0].Done = true
	}

	if count, _ := a.ProductService.Count(bizID); count > 0 {
		items[1].Done = true
	}

	if count, _ := a.WarehouseService.Count(bizID); count > 0 {
		items[2].Done = true
	}

	if modSvc := a.moduleService(r); modSvc != nil {
		counts, _ := modSvc.Counts(bizID)
		if counts["customers"] > 0 {
			items[4].Done = true
		}
		if counts["invoices"] > 0 {
			items[5].Done = true
		}
	}

	return items
}
