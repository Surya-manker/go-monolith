package services

import (
	"errors"
	"net/http"
	"strings"

	"go-monolith/models"
	"golang.org/x/crypto/bcrypt"
)

// BusinessStore is needed to create a business during registration.
var businessStoreRef *models.BusinessStore

func SetBusinessStore(bs *models.BusinessStore) {
	businessStoreRef = bs
}

const sessionCookie = "session"

type AuthService struct {
	store *models.AuthStore
}

func NewAuthService(store *models.AuthStore) *AuthService {
	return &AuthService{store: store}
}

// RegisterWithBusiness creates a new business + owner user in one atomic operation.
// Call this during public registration.
func (s *AuthService) RegisterWithBusiness(businessName, name, email, password string) (*models.User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	businessName = strings.TrimSpace(businessName)
	if businessName == "" || name == "" || email == "" || password == "" {
		return nil, errors.New("all fields are required")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}
	if businessStoreRef == nil {
		return nil, errors.New("business store not initialised")
	}
	// 1. Create business first
	biz, err := businessStoreRef.Create(businessName, email)
	if err != nil {
		return nil, errors.New("could not create business: " + err.Error())
	}
	// 2. Create owner user linked to this business
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return s.store.CreateUser(name, email, string(hash), "admin", biz.ID)
}

// Register creates a staff user for an existing business (used by admin invite).
func (s *AuthService) Register(name, email, password, role string, businessID int) (*models.User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" || email == "" || password == "" {
		return nil, errors.New("all fields are required")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}
	if role == "" {
		role = "staff"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return s.store.CreateUser(name, email, string(hash), role, businessID)
}

func (s *AuthService) Login(email, password, ip, ua string, remember bool) (*models.Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		s.store.LogActivity(nil, email, ip, ua, false)
		return nil, errors.New("invalid email or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.store.LogActivity(&user.ID, email, ip, ua, false)
		return nil, errors.New("invalid email or password")
	}
	sess, err := s.store.CreateSession(user.ID, ip, ua, remember)
	if err != nil {
		return nil, err
	}
	s.store.UpdateLastLogin(user.ID)
	s.store.LogActivity(&user.ID, email, ip, ua, true)
	return sess, nil
}

func (s *AuthService) Logout(r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_ = s.store.DeleteSession(cookie.Value)
	}
}

func (s *AuthService) GetUserFromRequest(r *http.Request) (*models.User, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, err
	}
	sess, err := s.store.GetSession(cookie.Value)
	if err != nil {
		return nil, err
	}
	return s.store.GetUserByID(sess.UserID)
}

func (s *AuthService) SetCookie(w http.ResponseWriter, sess *models.Session, remember bool) {
	maxAge := 86400
	if remember {
		maxAge = 86400 * 30
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sess.ID,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *AuthService) UpdateName(userID int, name string) error {
	return s.store.UpdateName(userID, name)
}

func (s *AuthService) UpdatePassword(userID int, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.UpdatePassword(userID, string(hash))
}

func (s *AuthService) VerifyPassword(userID int, password string) error {
	hash, err := s.store.GetPasswordHash(userID)
	if err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (s *AuthService) ListUsers() ([]models.User, error) {
	return s.store.ListUsers()
}

func (s *AuthService) GetUser(id int) (*models.User, error) {
	return s.store.GetUserByID(id)
}

func (s *AuthService) AdminUpdateUser(id int, name, email, role, newPassword string) error {
	if err := s.store.AdminUpdateUser(id, name, email, role); err != nil {
		return err
	}
	if newPassword != "" {
		return s.UpdatePassword(id, newPassword)
	}
	return nil
}

func (s *AuthService) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}
