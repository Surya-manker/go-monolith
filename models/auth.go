package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

type User struct {
	ID            int
	BusinessID    int // tenant identifier — always set, never zero for real users
	Name          string
	Email         string
	PasswordHash  string
	Role          string
	EmailVerified bool
	LastLogin     *time.Time
	CreatedAt     time.Time
}

type Session struct {
	ID        string
	UserID    int
	IPAddress string
	UserAgent string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type AuthStore struct {
	db *sql.DB
}

func NewAuthStore(db *sql.DB) *AuthStore {
	return &AuthStore{db: db}
}

func (s *AuthStore) Migrate() error {
	// Extend existing users table with auth + tenancy columns; ignore "duplicate column" errors.
	for _, col := range []string{
		`ALTER TABLE users ADD COLUMN business_id INT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN email_verified TINYINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN last_login DATETIME`,
	} {
		_, _ = s.db.Exec(col)
	}
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_business ON users(business_id)`)
	_, _ = s.db.Exec(`CREATE INDEX idx_users_business ON users(business_id)`) // ignore error on re-run

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id         VARCHAR(128) NOT NULL PRIMARY KEY,
			user_id    INT          NOT NULL,
			ip_address VARCHAR(50)  NOT NULL DEFAULT '',
			user_agent TEXT         NOT NULL DEFAULT '',
			expires_at DATETIME     NOT NULL,
			created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS login_activities (
			id         INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
			user_id    INT,
			email      VARCHAR(255) NOT NULL,
			ip_address VARCHAR(50)  NOT NULL DEFAULT '',
			user_agent TEXT         NOT NULL DEFAULT '',
			success    TINYINT      NOT NULL DEFAULT 0,
			created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	// Indexes — ignore errors on re-run.
	_, _ = s.db.Exec(`CREATE INDEX idx_sessions_user ON sessions(user_id)`)
	return nil
}

func (s *AuthStore) GetUserByEmail(email string) (*User, error) {
	var u User
	var ll sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, business_id, name, email, password_hash, role, email_verified, last_login, created_at
		FROM users WHERE email = ? AND deleted_at IS NULL
	`, email).Scan(&u.ID, &u.BusinessID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.EmailVerified, &ll, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	if ll.Valid {
		u.LastLogin = &ll.Time
	}
	return &u, nil
}

func (s *AuthStore) GetUserByID(id int) (*User, error) {
	var u User
	var ll sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, business_id, name, email, password_hash, role, email_verified, last_login, created_at
		FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.BusinessID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.EmailVerified, &ll, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	if ll.Valid {
		u.LastLogin = &ll.Time
	}
	return &u, nil
}

// CreateUser creates a user scoped to the given business.
// businessID must never be 0 for real users.
func (s *AuthStore) CreateUser(name, email, passwordHash, role string, businessID int) (*User, error) {
	res, err := s.db.Exec(
		`INSERT INTO users (business_id, name, email, password_hash, role) VALUES (?, ?, ?, ?, ?)`,
		businessID, name, email, passwordHash, role,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(int(id))
}

func (s *AuthStore) UpdateLastLogin(userID int) {
	_, _ = s.db.Exec(`UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE id = ?`, userID)
}

func (s *AuthStore) UpdateName(userID int, name string) error {
	_, err := s.db.Exec(`UPDATE users SET name = ? WHERE id = ?`, name, userID)
	return err
}

func (s *AuthStore) UpdatePassword(userID int, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	return err
}

func (s *AuthStore) GetPasswordHash(userID int) (string, error) {
	var hash string
	err := s.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash)
	return hash, err
}

// ListUsersByBusiness returns only users belonging to the given business.
func (s *AuthStore) ListUsersByBusiness(businessID int) ([]User, error) {
	rows, err := s.db.Query(`
		SELECT id, business_id, name, email, role, email_verified, last_login, created_at
		FROM users WHERE deleted_at IS NULL AND business_id = ? ORDER BY created_at DESC
	`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		var ll sql.NullTime
		if err := rows.Scan(&u.ID, &u.BusinessID, &u.Name, &u.Email, &u.Role, &u.EmailVerified, &ll, &u.CreatedAt); err != nil {
			return nil, err
		}
		if ll.Valid {
			u.LastLogin = &ll.Time
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *AuthStore) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`
		SELECT id, business_id, name, email, role, email_verified, last_login, created_at
		FROM users WHERE deleted_at IS NULL ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		var ll sql.NullTime
		if err := rows.Scan(&u.ID, &u.BusinessID, &u.Name, &u.Email, &u.Role, &u.EmailVerified, &ll, &u.CreatedAt); err != nil {
			return nil, err
		}
		if ll.Valid {
			u.LastLogin = &ll.Time
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *AuthStore) AdminUpdateUser(id int, name, email, role string) error {
	_, err := s.db.Exec(
		`UPDATE users SET name=?, email=?, role=? WHERE id=?`,
		name, email, role, id,
	)
	return err
}

func (s *AuthStore) CreateSession(userID int, ip, ua string, remember bool) (*Session, error) {
	token, err := genToken(32)
	if err != nil {
		return nil, err
	}
	exp := time.Now().Add(24 * time.Hour)
	if remember {
		exp = time.Now().Add(30 * 24 * time.Hour)
	}
	_, err = s.db.Exec(
		`INSERT INTO sessions (id, user_id, ip_address, user_agent, expires_at) VALUES (?, ?, ?, ?, ?)`,
		token, userID, ip, ua, exp,
	)
	if err != nil {
		return nil, err
	}
	return &Session{ID: token, UserID: userID, ExpiresAt: exp}, nil
}

func (s *AuthStore) GetSession(token string) (*Session, error) {
	var sess Session
	err := s.db.QueryRow(`
		SELECT id, user_id, ip_address, user_agent, expires_at, created_at
		FROM sessions WHERE id = ? AND expires_at > CURRENT_TIMESTAMP
	`, token).Scan(&sess.ID, &sess.UserID, &sess.IPAddress, &sess.UserAgent, &sess.ExpiresAt, &sess.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *AuthStore) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, token)
	return err
}

func (s *AuthStore) LogActivity(userID *int, email, ip, ua string, success bool) {
	var uid any
	if userID != nil {
		uid = *userID
	}
	_, _ = s.db.Exec(
		`INSERT INTO login_activities (user_id, email, ip_address, user_agent, success) VALUES (?, ?, ?, ?, ?)`,
		uid, email, ip, ua, boolInt(success),
	)
}

func genToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
