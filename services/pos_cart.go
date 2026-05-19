package services

import (
	"math"
	"sync"
	"time"
)

// CartItem represents one line in the POS cart.
type CartItem struct {
	ProductID   int
	ProductName string
	SKU         string
	Barcode     string
	UnitPrice   float64
	TaxRate     float64
	Quantity    int
	Discount    float64
}

// LineTotal returns the pre-tax line total after discount.
func (c CartItem) LineSubtotal() float64 {
	return roundCents(c.UnitPrice*float64(c.Quantity) - c.Discount)
}

// LineTax returns the GST/tax amount for this line.
func (c CartItem) LineTax() float64 {
	return roundCents(c.LineSubtotal() * c.TaxRate / 100)
}

// LineTotal returns subtotal + tax.
func (c CartItem) LineTotal() float64 {
	return c.LineSubtotal() + c.LineTax()
}

// POSCart holds all items for a single checkout session.
type POSCart struct {
	Items     []CartItem
	ExpiresAt time.Time
}

func (c *POSCart) IsEmpty() bool { return len(c.Items) == 0 }

func (c *POSCart) Subtotal() float64 {
	var s float64
	for _, item := range c.Items {
		s += item.LineSubtotal()
	}
	return roundCents(s)
}

func (c *POSCart) TaxTotal() float64 {
	var t float64
	for _, item := range c.Items {
		t += item.LineTax()
	}
	return roundCents(t)
}

func (c *POSCart) GrandTotal() float64 {
	return roundCents(c.Subtotal() + c.TaxTotal())
}

func (c *POSCart) ItemCount() int {
	n := 0
	for _, i := range c.Items {
		n += i.Quantity
	}
	return n
}

// AddItem adds a product to the cart or increments its quantity.
func (c *POSCart) AddItem(item CartItem) {
	for i, existing := range c.Items {
		if existing.ProductID == item.ProductID {
			c.Items[i].Quantity += item.Quantity
			return
		}
	}
	if item.Quantity <= 0 {
		item.Quantity = 1
	}
	c.Items = append(c.Items, item)
}

// UpdateQty sets an item's quantity. qty=0 removes the item.
func (c *POSCart) UpdateQty(productID, qty int) {
	if qty <= 0 {
		c.RemoveItem(productID)
		return
	}
	for i, item := range c.Items {
		if item.ProductID == productID {
			c.Items[i].Quantity = qty
			return
		}
	}
}

// RemoveItem removes a product from the cart entirely.
func (c *POSCart) RemoveItem(productID int) {
	filtered := c.Items[:0]
	for _, item := range c.Items {
		if item.ProductID != productID {
			filtered = append(filtered, item)
		}
	}
	c.Items = filtered
}

// POSCartManager manages per-session POS carts in memory.
// Session token (from auth cookie) is the key.
type POSCartManager struct {
	mu    sync.RWMutex
	carts map[string]*POSCart
}

func NewPOSCartManager() *POSCartManager {
	m := &POSCartManager{carts: make(map[string]*POSCart)}
	go m.cleanupLoop()
	return m
}

func (m *POSCartManager) Get(sessionToken string) *POSCart {
	m.mu.RLock()
	cart, ok := m.carts[sessionToken]
	m.mu.RUnlock()
	if !ok || time.Now().After(cart.ExpiresAt) {
		return &POSCart{ExpiresAt: time.Now().Add(4 * time.Hour)}
	}
	return cart
}

func (m *POSCartManager) Save(sessionToken string, cart *POSCart) {
	cart.ExpiresAt = time.Now().Add(4 * time.Hour)
	m.mu.Lock()
	m.carts[sessionToken] = cart
	m.mu.Unlock()
}

func (m *POSCartManager) Clear(sessionToken string) {
	m.mu.Lock()
	delete(m.carts, sessionToken)
	m.mu.Unlock()
}

func (m *POSCartManager) cleanupLoop() {
	ticker := time.NewTicker(20 * time.Minute)
	for range ticker.C {
		now := time.Now()
		m.mu.Lock()
		for k, v := range m.carts {
			if now.After(v.ExpiresAt) {
				delete(m.carts, k)
			}
		}
		m.mu.Unlock()
	}
}

func roundCents(v float64) float64 {
	return math.Round(v*100) / 100
}
