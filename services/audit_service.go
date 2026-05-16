package services

import (
	"fmt"

	"go-monolith/models"
)

type AuditService struct {
	store *models.AuditStore
}

func NewAuditService(store *models.AuditStore) *AuditService {
	return &AuditService{store: store}
}

// Log records a mutation. Callers supply user context (extracted in the handler layer).
func (s *AuditService) Log(userID *int, userName, module, action, recordID string, vals map[string]string, ip string) {
	s.store.Log(userID, userName, module, action, recordID, "", formatVals(vals), ip)
}

// LogChange records a before/after mutation for updates.
func (s *AuditService) LogChange(userID *int, userName, module, action, recordID string, before, after map[string]string, ip string) {
	s.store.Log(userID, userName, module, action, recordID, formatVals(before), formatVals(after), ip)
}

func (s *AuditService) List(limit int) ([]models.Record, error) {
	return s.store.List(limit)
}

func formatVals(vals map[string]string) string {
	if len(vals) == 0 {
		return ""
	}
	out := ""
	for k, v := range vals {
		if out != "" {
			out += ", "
		}
		out += fmt.Sprintf("%s=%q", k, v)
	}
	return out
}
