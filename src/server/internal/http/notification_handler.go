package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strconv"

	"oblivious/server/internal/notification"
)

type notificationHandler struct {
	service *notification.Service
}

func newNotificationHandler(service *notification.Service) notificationHandler {
	return notificationHandler{service: service}
}

func (h notificationHandler) list(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	unreadOnly := r.URL.Query().Get("unread") == "true"
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	notifications, err := h.service.List(r.Context(), session.User.ID, unreadOnly, limit, offset)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, notifications)
}

func (h notificationHandler) getUnreadCount(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	count, err := h.service.GetUnreadCount(r.Context(), session.User.ID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]int{"count": count})
}

func (h notificationHandler) markRead(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	notif, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if notif == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "notification not found")
		return
	}
	if notif.UserID != session.User.ID {
		writeError(w, stdhttp.StatusForbidden, "forbidden", "notification access denied")
		return
	}

	if err := h.service.MarkRead(r.Context(), id); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "ok"})
}

func (h notificationHandler) markAllRead(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	if err := h.service.MarkAllRead(r.Context(), session.User.ID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "ok"})
}

func (h notificationHandler) delete(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	notif, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if notif == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "notification not found")
		return
	}
	if notif.UserID != session.User.ID {
		writeError(w, stdhttp.StatusForbidden, "forbidden", "notification access denied")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

func (h notificationHandler) create(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req notification.CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	notif, err := h.service.Create(r.Context(), session.User.ID, &req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, notif)
}
