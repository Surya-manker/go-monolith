package models

import "database/sql"

type AuditStore struct {
	db *sql.DB
}

func NewAuditStore(db *sql.DB) *AuditStore {
	return &AuditStore{db: db}
}

func (s *AuditStore) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS audit_logs (
			id         INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
			user_id    INT,
			user_name  VARCHAR(255) NOT NULL DEFAULT '',
			module     VARCHAR(50)  NOT NULL,
			action     VARCHAR(50)  NOT NULL,
			record_id  VARCHAR(50)  NOT NULL DEFAULT '',
			old_value  TEXT         NOT NULL DEFAULT '',
			new_value  TEXT         NOT NULL DEFAULT '',
			ip_address VARCHAR(50)  NOT NULL DEFAULT '',
			created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	// Indexes — ignore errors on re-run.
	_, _ = s.db.Exec(`CREATE INDEX idx_audit_module  ON audit_logs(module)`)
	_, _ = s.db.Exec(`CREATE INDEX idx_audit_user    ON audit_logs(user_id)`)
	_, _ = s.db.Exec(`CREATE INDEX idx_audit_created ON audit_logs(created_at)`)
	return nil
}

func (s *AuditStore) Log(userID *int, userName, module, action, recordID, oldVal, newVal, ip string) {
	var uid any
	if userID != nil {
		uid = *userID
	}
	_, _ = s.db.Exec(`
		INSERT INTO audit_logs (user_id, user_name, module, action, record_id, old_value, new_value, ip_address)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, uid, userName, module, action, recordID, oldVal, newVal, ip)
}

// List returns the most recent audit entries across all modules.
func (s *AuditStore) List(limit int) ([]Record, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(user_name,'system'), module, action, record_id, old_value, new_value, ip_address, created_at
		FROM audit_logs
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []Record
	for rows.Next() {
		var id, user, module, action, recID, oldV, newV, ip, ts string
		if err := rows.Scan(&id, &user, &module, &action, &recID, &oldV, &newV, &ip, &ts); err != nil {
			return nil, err
		}
		records = append(records, Record{
			"id": id, "user_name": user, "module": module, "action": action,
			"record_id": recID, "old_value": oldV, "new_value": newV,
			"ip_address": ip, "created_at": ts,
		})
	}
	return records, rows.Err()
}
