package models

import (
	"database/sql"
)

// Business represents one tenant — one company/shop using the system.
type Business struct {
	ID        int
	Name      string
	Email     string
	Phone     string
	GSTIN     string
	Address   string
	StateCode string
	Status    string // active, trial, suspended
	CreatedAt string
}

type BusinessStore struct {
	db *sql.DB
}

func NewBusinessStore(db *sql.DB) *BusinessStore {
	return &BusinessStore{db: db}
}

func (s *BusinessStore) Migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS businesses (
		id         INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
		name       VARCHAR(255) NOT NULL,
		email      VARCHAR(255) NOT NULL DEFAULT '',
		phone      VARCHAR(50)  NOT NULL DEFAULT '',
		gstin      VARCHAR(20)  NOT NULL DEFAULT '',
		address    TEXT         NOT NULL DEFAULT '',
		state_code VARCHAR(5)   NOT NULL DEFAULT '',
		status     VARCHAR(20)  NOT NULL DEFAULT 'active',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

func (s *BusinessStore) Create(name, email string) (*Business, error) {
	res, err := s.db.Exec(
		`INSERT INTO businesses (name, email) VALUES (?, ?)`,
		name, email,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetByID(int(id))
}

func (s *BusinessStore) GetByID(id int) (*Business, error) {
	var b Business
	err := s.db.QueryRow(
		`SELECT id, name, email, phone, gstin, address, state_code, status, created_at
		 FROM businesses WHERE id = ?`, id,
	).Scan(&b.ID, &b.Name, &b.Email, &b.Phone, &b.GSTIN,
		&b.Address, &b.StateCode, &b.Status, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *BusinessStore) Update(id int, name, email, phone, gstin, address, stateCode string) error {
	_, err := s.db.Exec(
		`UPDATE businesses SET name=?, email=?, phone=?, gstin=?, address=?, state_code=? WHERE id=?`,
		name, email, phone, gstin, address, stateCode, id,
	)
	return err
}

// Ping checks the database connection — used by the /health/ready endpoint.
func (s *BusinessStore) Ping() error {
	return s.db.Ping()
}

func (s *BusinessStore) List() ([]Business, error) {
	rows, err := s.db.Query(
		`SELECT id, name, email, phone, status, created_at FROM businesses ORDER BY id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Business
	for rows.Next() {
		var b Business
		if err := rows.Scan(&b.ID, &b.Name, &b.Email, &b.Phone, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
