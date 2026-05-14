package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/notification"
)

func TestNotificationMutationRoutesEnforceOwnership(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	service := notification.NewService(notification.NewSQLStore(database))

	ownerID, ownerCookie := notificationRegisterUser(t, router, "notif-owner@example.com")
	_, otherCookie := notificationRegisterUser(t, router, "notif-other@example.com")

	notif, err := service.Create(context.Background(), ownerID, &notification.CreateNotificationRequest{
		Title:   "Owned notice",
		Message: "Only owner can mutate this",
	})
	if err != nil {
		t.Fatalf("create notification: %v", err)
	}

	forbiddenPatch := httptest.NewRecorder()
	forbiddenPatchRequest := httptest.NewRequest(stdhttp.MethodPatch, "/api/v1/app/notifications/"+notif.ID, nil)
	forbiddenPatchRequest.AddCookie(otherCookie)
	router.ServeHTTP(forbiddenPatch, forbiddenPatchRequest)
	if forbiddenPatch.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected non-owner PATCH to return 403, got %d with body %s", forbiddenPatch.Code, forbiddenPatch.Body.String())
	}
	stored, err := service.Get(context.Background(), notif.ID)
	if err != nil {
		t.Fatalf("get notification after forbidden patch: %v", err)
	}
	if stored == nil || stored.IsRead {
		t.Fatalf("expected notification to remain unread after forbidden patch, got %+v", stored)
	}

	ownerPatch := httptest.NewRecorder()
	ownerPatchRequest := httptest.NewRequest(stdhttp.MethodPatch, "/api/v1/app/notifications/"+notif.ID, nil)
	ownerPatchRequest.AddCookie(ownerCookie)
	router.ServeHTTP(ownerPatch, ownerPatchRequest)
	if ownerPatch.Code != stdhttp.StatusOK {
		t.Fatalf("expected owner PATCH to return 200, got %d with body %s", ownerPatch.Code, ownerPatch.Body.String())
	}
	stored, err = service.Get(context.Background(), notif.ID)
	if err != nil {
		t.Fatalf("get notification after owner patch: %v", err)
	}
	if stored == nil || !stored.IsRead {
		t.Fatalf("expected owner patch to mark notification read, got %+v", stored)
	}

	deleteTarget, err := service.Create(context.Background(), ownerID, &notification.CreateNotificationRequest{
		Title:   "Delete me",
		Message: "Only owner can delete this",
	})
	if err != nil {
		t.Fatalf("create delete target: %v", err)
	}

	forbiddenDelete := httptest.NewRecorder()
	forbiddenDeleteRequest := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/app/notifications/"+deleteTarget.ID, nil)
	forbiddenDeleteRequest.AddCookie(otherCookie)
	router.ServeHTTP(forbiddenDelete, forbiddenDeleteRequest)
	if forbiddenDelete.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected non-owner DELETE to return 403, got %d with body %s", forbiddenDelete.Code, forbiddenDelete.Body.String())
	}
	stored, err = service.Get(context.Background(), deleteTarget.ID)
	if err != nil {
		t.Fatalf("get notification after forbidden delete: %v", err)
	}
	if stored == nil {
		t.Fatal("expected notification to still exist after forbidden delete")
	}

	ownerDelete := httptest.NewRecorder()
	ownerDeleteRequest := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/app/notifications/"+deleteTarget.ID, nil)
	ownerDeleteRequest.AddCookie(ownerCookie)
	router.ServeHTTP(ownerDelete, ownerDeleteRequest)
	if ownerDelete.Code != stdhttp.StatusOK {
		t.Fatalf("expected owner DELETE to return 200, got %d with body %s", ownerDelete.Code, ownerDelete.Body.String())
	}
	stored, err = service.Get(context.Background(), deleteTarget.ID)
	if err != nil {
		t.Fatalf("get notification after owner delete: %v", err)
	}
	if stored != nil {
		t.Fatalf("expected notification to be deleted, got %+v", stored)
	}
}

func notificationRegisterUser(t *testing.T, router stdhttp.Handler, email string) (string, *stdhttp.Cookie) {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"`+email+`","password":"secret"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("register %s: expected 200, got %d with body %s", email, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	cookies := recorder.Result().Cookies()
	if response.Data.User.ID == "" || len(cookies) == 0 {
		t.Fatalf("expected user id and session cookie, got id=%q cookies=%d", response.Data.User.ID, len(cookies))
	}
	return response.Data.User.ID, cookies[0]
}
