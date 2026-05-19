package services

import (
	"sync"
	"time"

	"go-monolith/models"
)

const demoSessionTTL = 24 * time.Hour

type demoEntry struct {
	store     *models.DemoStore
	createdAt time.Time
}

// DemoSessionManager maps auth session tokens to in-memory demo stores.
// Keying on the auth token (not a separate cookie) means a normal login
// always gets a fresh token that is never in the demo map — real DB is used.
type DemoSessionManager struct {
	mu       sync.Mutex
	sessions map[string]*demoEntry
}

func NewDemoSessionManager() *DemoSessionManager {
	m := &DemoSessionManager{sessions: make(map[string]*demoEntry)}
	go m.cleanup()
	return m
}

// Register associates an auth session token with a fresh seeded DemoStore.
func (m *DemoSessionManager) Register(authToken string) {
	store := models.NewDemoStore()
	store.Seed()
	m.mu.Lock()
	m.sessions[authToken] = &demoEntry{store: store, createdAt: time.Now()}
	m.mu.Unlock()
}

// Get returns the DemoStore for the given auth token, or nil if not a demo session.
func (m *DemoSessionManager) Get(authToken string) *models.DemoStore {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.sessions[authToken]
	if !ok || time.Since(entry.createdAt) > demoSessionTTL {
		delete(m.sessions, authToken)
		return nil
	}
	return entry.store
}

// Remove deletes the demo mapping for an auth token (called on logout).
func (m *DemoSessionManager) Remove(authToken string) {
	m.mu.Lock()
	delete(m.sessions, authToken)
	m.mu.Unlock()
}

func (m *DemoSessionManager) cleanup() {
	for range time.Tick(time.Hour) {
		m.mu.Lock()
		for id, entry := range m.sessions {
			if time.Since(entry.createdAt) > demoSessionTTL {
				delete(m.sessions, id)
			}
		}
		m.mu.Unlock()
	}
}
