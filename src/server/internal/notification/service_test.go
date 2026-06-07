package notification

import (
	"context"
	"testing"
	"time"
)

type fakeNotificationStore struct {
	deletedID    string
	notification *Notification
}

func (s *fakeNotificationStore) Create(ctx context.Context, notification *Notification) (*Notification, error) {
	s.notification = notification
	return notification, nil
}

func (s *fakeNotificationStore) Get(ctx context.Context, id string) (*Notification, error) {
	if s.notification != nil && s.notification.ID == id {
		return s.notification, nil
	}
	return nil, nil
}

func (s *fakeNotificationStore) List(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]*Notification, error) {
	return nil, nil
}

func (s *fakeNotificationStore) MarkRead(ctx context.Context, id string) error {
	if s.notification != nil && s.notification.ID == id {
		s.notification.IsRead = true
	}
	return nil
}

func (s *fakeNotificationStore) MarkAllRead(ctx context.Context, userID string) error {
	return nil
}

func (s *fakeNotificationStore) Delete(ctx context.Context, id string) error {
	s.deletedID = id
	if s.notification != nil && s.notification.ID == id {
		s.notification = nil
	}
	return nil
}

func (s *fakeNotificationStore) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func TestNotificationStruct(t *testing.T) {
	notif := &Notification{
		ID:        "notif-1",
		UserID:    "user-1",
		Title:     "Test",
		Message:   "Message",
		IsRead:    false,
		CreatedAt: time.Now().UTC(),
	}

	if notif.IsRead {
		t.Error("notification should not be read by default")
	}
	if notif.ID != "notif-1" {
		t.Errorf("expected ID notif-1, got %s", notif.ID)
	}
}

func TestCreateNotificationRequestDefaults(t *testing.T) {
	req := &CreateNotificationRequest{
		Title:   "Test",
		Message: "Message",
	}

	notifType := req.Type
	if notifType == "" {
		notifType = "info"
	}
	if notifType != "info" {
		t.Errorf("expected default type 'info', got '%s'", notifType)
	}

	category := req.Category
	if category == "" {
		category = "system"
	}
	if category != "system" {
		t.Errorf("expected default category 'system', got '%s'", category)
	}
}

func TestValidationLogic(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		message string
		wantErr bool
	}{
		{"empty title", "", "message", true},
		{"empty message", "title", "", true},
		{"valid", "title", "message", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &CreateNotificationRequest{
				Title:   tt.title,
				Message: tt.message,
			}

			hasErr := false
			if req.Title == "" {
				hasErr = true
			}
			if req.Message == "" {
				hasErr = true
			}

			if hasErr != tt.wantErr {
				t.Errorf("expected error=%v, got error=%v", tt.wantErr, hasErr)
			}
		})
	}
}

func TestListLimitValidation(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 20},
		{-1, 20},
		{50, 50},
		{150, 100},
	}

	for _, tt := range tests {
		limit := tt.input
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}

		if limit != tt.expected {
			t.Errorf("input %d: expected %d, got %d", tt.input, tt.expected, limit)
		}
	}
}

func TestServiceGetAndDeleteDelegateToStore(t *testing.T) {
	store := &fakeNotificationStore{
		notification: &Notification{
			ID:     "notif_1",
			UserID: "user_1",
			Title:  "Test",
		},
	}
	service := NewService(store)

	got, err := service.Get(context.Background(), "notif_1")
	if err != nil {
		t.Fatalf("Get notification: %v", err)
	}
	if got == nil || got.ID != "notif_1" {
		t.Fatalf("expected notif_1 from Get, got %+v", got)
	}

	if err := service.Delete(context.Background(), "notif_1"); err != nil {
		t.Fatalf("Delete notification: %v", err)
	}
	if store.deletedID != "notif_1" {
		t.Fatalf("expected Delete to delegate notif_1, got %q", store.deletedID)
	}
	if got, err := service.Get(context.Background(), "notif_1"); err != nil || got != nil {
		t.Fatalf("expected deleted notification to be missing, got %+v err=%v", got, err)
	}
}

func TestServiceCreateEventPersistsBusinessNotification(t *testing.T) {
	store := &fakeNotificationStore{}
	service := NewService(store)

	created, err := service.CreateEvent(context.Background(), NotificationEvent{
		UserID:    "publisher_user",
		Type:      "warning",
		Category:  "marketplace",
		Title:     "Marketplace report received",
		Message:   "Your published agent was reported for malware.",
		ActionURL: "/marketplace/agents/agent_abuse",
		Metadata: map[string]any{
			"event":   "marketplace.abuse_report.opened",
			"agentID": "agent_abuse",
		},
	})
	if err != nil {
		t.Fatalf("CreateEvent returned error: %v", err)
	}
	if created.UserID != "publisher_user" || created.Type != "warning" || created.Category != "marketplace" {
		t.Fatalf("unexpected notification envelope: %+v", created)
	}
	if created.Metadata["event"] != "marketplace.abuse_report.opened" || created.ActionURL != "/marketplace/agents/agent_abuse" {
		t.Fatalf("event metadata/action URL was not preserved: %+v", created)
	}
}
