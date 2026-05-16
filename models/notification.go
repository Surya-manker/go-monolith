package models

import "database/sql"

type Notification struct {
	ID        int
	UserID    int
	Type      string
	Message   string
	Module    string
	RecordID  string
	Read      bool
	CreatedAt string
}

type NotificationStore struct {
	db *sql.DB
}

func NewNotificationStore(db *sql.DB) *NotificationStore {
	return &NotificationStore{db: db}
}

func (s *NotificationStore) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS notifications (
			id         INT         NOT NULL AUTO_INCREMENT PRIMARY KEY,
			user_id    INT         NOT NULL,
			type       VARCHAR(20) NOT NULL DEFAULT 'info',
			message    TEXT        NOT NULL,
			module     VARCHAR(50) NOT NULL DEFAULT '',
			record_id  VARCHAR(50) NOT NULL DEFAULT '',
			is_read    TINYINT     NOT NULL DEFAULT 0,
			created_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`CREATE INDEX idx_notif_user_unread ON notifications(user_id, is_read)`)
	return nil
}

func (s *NotificationStore) Create(userID int, nType, message, module, recordID string) error {
	_, err := s.db.Exec(
		`INSERT INTO notifications (user_id, type, message, module, record_id) VALUES (?,?,?,?,?)`,
		userID, nType, message, module, recordID,
	)
	return err
}

func (s *NotificationStore) ListForUser(userID, limit int) ([]Notification, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, type, message, module, record_id, is_read, created_at
		FROM notifications WHERE user_id = ?
		ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var notifs []Notification
	for rows.Next() {
		var n Notification
		var isRead int
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Message, &n.Module, &n.RecordID, &isRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.Read = isRead == 1
		notifs = append(notifs, n)
	}
	return notifs, rows.Err()
}

func (s *NotificationStore) UnreadCount(userID int) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0`, userID).Scan(&count)
	return count, err
}

func (s *NotificationStore) MarkAllRead(userID int) error {
	_, err := s.db.Exec(`UPDATE notifications SET is_read = 1 WHERE user_id = ?`, userID)
	return err
}

func (s *NotificationStore) MarkRead(id, userID int) error {
	_, err := s.db.Exec(`UPDATE notifications SET is_read = 1 WHERE id = ? AND user_id = ?`, id, userID)
	return err
}
