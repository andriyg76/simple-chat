package chathttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agushuley/simple-chat/go/pkg/chat"
	"github.com/agushuley/simple-chat/go/pkg/chatcore"
	"github.com/agushuley/simple-chat/go/pkg/msgstore"
)

type Handler struct {
	chat  *chat.Service
	store *msgstore.Store
	core  *chatcore.Core
}

var heartbeatInterval = 15 * time.Second

func New(chatService *chat.Service, store *msgstore.Store) *Handler {
	return &Handler{
		chat:  chatService,
		store: store,
		core:  chatcore.New(coreStore{chatService, store}),
	}
}

func (h *Handler) Core() *chatcore.Core { return h.core }

func (h *Handler) Routes() http.Handler {
	r := http.NewServeMux()
	r.HandleFunc("POST /chat/v1/enrollments/exchange", h.exchangeEnrollment)
	r.HandleFunc("GET /chat/v1/conversations", h.listConversations)
	r.HandleFunc("POST /chat/v1/conversations", h.createConversation)
	r.HandleFunc("POST /chat/v1/conversations/{code}/messages/plain", h.plainMessage)
	r.HandleFunc("GET /chat/v1/conversations/{code}/feed", h.pollFeed)
	r.HandleFunc("GET /chat/v1/conversations/{code}/events", h.events)
	r.HandleFunc("GET /chat/v1/conversations/{code}/cursor", h.currentCursor)
	return r
}

func (h *Handler) pollFeed(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	cursor, err := parseCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	event, found, err := h.core.Next(r.Context(), p.ID.Hex(), r.PathValue("code"), cursor, 30*time.Second)
	if h.deliveryError(w, err) {
		return
	}
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{"cursor": event.Cursor})
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func parseCursor(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 {
		return 0, chatcore.ErrInvalidCursor
	}
	return n, nil
}

func (h *Handler) currentCursor(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	cursor, err := h.core.Cursor(r.Context(), p.ID.Hex(), r.PathValue("code"))
	if h.deliveryError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"cursor": cursor})
}

func (h *Handler) deliveryError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, chatcore.ErrNotFound):
		writeError(w, http.StatusNotFound, "conversation_not_found", err.Error())
	case errors.Is(err, chatcore.ErrCursorExpired):
		writeError(w, http.StatusConflict, "cursor_expired", err.Error())
	case errors.Is(err, chatcore.ErrInvalidCursor):
		writeError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
	case errors.Is(err, context.Canceled):
		return true
	default:
		writeError(w, http.StatusInternalServerError, "chat_delivery", err.Error())
	}
	return true
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	h.streamEvents(w, r, p.ID.Hex())
}

func (h *Handler) streamEvents(w http.ResponseWriter, r *http.Request, participant string) {
	cursor, err := parseCursor(r.Header.Get("Last-Event-ID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	if r.Header.Get("Last-Event-ID") == "" {
		cursor, err = parseCursor(r.URL.Query().Get("cursor"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
			return
		}
	}
	code := r.PathValue("code")
	event, found, err := h.core.Next(r.Context(), participant, code, cursor, 0)
	if h.deliveryError(w, err) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", "streaming is unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	for {
		if !found {
			event, found, err = h.core.Next(r.Context(), participant, code, cursor, heartbeatInterval)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				if errors.Is(err, chatcore.ErrCursorExpired) {
					fmt.Fprint(w, "event: error\ndata: {\"error\":\"cursor_expired\"}\n\n")
					flusher.Flush()
				}
				return
			}
			if !found {
				if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
					return
				}
				flusher.Flush()
				continue
			}
		}
		if event.Kind == "reply" && event.Text != "" {
			cursor, found = event.Cursor, false
			continue
		}
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Cursor, event.Kind, data); err != nil {
			return
		}
		flusher.Flush()
		cursor, found = event.Cursor, false
	}
}

func (h *Handler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DisplayName string `json:"displayName"`
	}
	if !decode(w, r, &request) {
		return
	}
	profile, code, err := h.chat.Bootstrap(r.Context(), request.DisplayName)
	if err != nil {
		writeError(w, http.StatusConflict, "bootstrap_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"profile": profile, "enrollmentCode": code})
}

func (h *Handler) ResetBootstrap(w http.ResponseWriter, r *http.Request) {
	removed, err := h.chat.ResetBootstrapState(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reset_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removedProfiles": removed})
}

func (h *Handler) exchangeEnrollment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &request) {
		return
	}
	profile, token, err := h.chat.ExchangeEnrollment(r.Context(), request.Code)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_enrollment", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expiresInSeconds": 30 * 24 * 60 * 60, "profile": profile})
}

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	archived := r.URL.Query().Get("archived") == "true"
	items, err := h.core.List(r.Context(), p.ID.Hex(), archived)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_conversations", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": items})
}

func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	c, err := h.core.Create(r.Context(), p.ID.Hex())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_conversation", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handler) plainMessage(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	var request struct {
		Message string `json:"message"`
	}
	if !decode(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Message) == "" {
		writeError(w, http.StatusUnprocessableEntity, "message_required", "message is required")
		return
	}
	event, err := h.core.Publish(r.Context(), p.ID.Hex(), r.PathValue("code"), "client", request.Message, "")
	if h.deliveryError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) (*chat.Profile, bool) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "bearer token is required")
		return nil, false
	}
	p, err := h.chat.Authenticate(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired bearer token")
		return nil, false
	}
	return p, true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON request")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
