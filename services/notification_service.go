package services

import "go-monolith/models"

type NotificationService struct {
	store *models.NotificationStore
}

func NewNotificationService(store *models.NotificationStore) *NotificationService {
	return &NotificationService{store: store}
}

func (s *NotificationService) Notify(userID int, nType, message, module, recordID string) {
	_ = s.store.Create(userID, nType, message, module, recordID)
}

func (s *NotificationService) List(userID int) ([]models.Notification, error) {
	return s.store.ListForUser(userID, 30)
}

func (s *NotificationService) UnreadCount(userID int) (int, error) {
	return s.store.UnreadCount(userID)
}

func (s *NotificationService) MarkAllRead(userID int) error {
	return s.store.MarkAllRead(userID)
}

func (s *NotificationService) MarkRead(id, userID int) error {
	return s.store.MarkRead(id, userID)
}
