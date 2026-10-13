package chathttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/andriyg76/simple-chat/go/pkg/chatcore"
	"github.com/google/uuid"
)

// Config wires transport auth for a reference server. Products replace this with real identity + storage.
type Config struct {
	BearerToken         string
	EnrollCode          string
	ParticipantID       string
	IdentityFromContext bool
	PublicBaseURL       string
	AdminToken          string
	DeviceStore         DeviceStore
	PushNotifier        PushNotifier
}

type Handler struct {
	core                *chatcore.Core
	bearerToken         string
	enrollCode          string
	participantID       string
	identityFromContext bool
	publicBaseURL       string
	adminToken          string
	tokens              *TokenStore
	challenges          *ChallengeStore
	devices             DeviceStore
	inboxPresence       *InboxPresence
	pushNotifier        PushNotifier
}

var heartbeatInterval = 15 * time.Second

func New(store chatcore.Store, cfg Config) *Handler {
	participant := cfg.ParticipantID
	if participant == "" {
		participant = "default"
	}
	core := chatcore.New(store)
	if members, ok := store.(chatcore.MembersStore); ok {
		core.WithMembers(members)
	}
	if metadata, ok := store.(chatcore.MetadataStore); ok {
		core.WithMetadata(metadata)
	}
	tokens := NewTokenStore(30*24*time.Hour, 2*time.Minute)
	if cfg.BearerToken != "" {
		tokens.InstallDevToken(cfg.BearerToken, participant, participant)
	}
	devices := cfg.DeviceStore
	if devices == nil {
		devices = NewMemoryDeviceStore()
	}
	push := cfg.PushNotifier
	if push == nil {
		push = &LoggingPushNotifier{}
	}
	return &Handler{
		core:                core,
		bearerToken:         cfg.BearerToken,
		enrollCode:          cfg.EnrollCode,
		participantID:       participant,
		identityFromContext: cfg.IdentityFromContext,
		publicBaseURL:       cfg.PublicBaseURL,
		adminToken:          cfg.AdminToken,
		tokens:              tokens,
		challenges:          NewChallengeStore(10 * time.Minute),
		devices:             devices,
		inboxPresence:       NewInboxPresence(),
		pushNotifier:        push,
	}
}

func (h *Handler) Core() *chatcore.Core { return h.core }

func (h *Handler) Routes() http.Handler {
	r := http.NewServeMux()
	h.registerEnrollmentRoutes(r)
	r.HandleFunc("GET /chat/v1/profile", h.getProfile)
	r.HandleFunc("GET /chat/v1/conversations", h.listConversations)
	r.HandleFunc("POST /chat/v1/conversations", h.createConversation)
	r.HandleFunc("POST /chat/v1/conversations/{code}/messages/async", h.asyncMessage)
	r.HandleFunc("POST /chat/v1/conversations/{code}/messages/plain", h.plainMessage)
	r.HandleFunc("GET /chat/v1/inbox/cursor", h.inboxCursor)
	r.HandleFunc("GET /chat/v1/inbox/feed", h.inboxFeed)
	r.HandleFunc("GET /chat/v1/inbox/events", h.inboxEvents)
	r.HandleFunc("PATCH /chat/v1/conversations/{code}", h.updateConversation)
	r.HandleFunc("GET /chat/v1/conversations/{code}/members", h.listMembers)
	r.HandleFunc("POST /chat/v1/conversations/{code}/members", h.addMember)
	r.HandleFunc("DELETE /chat/v1/conversations/{code}/members/{profileId}", h.removeMember)
	r.HandleFunc("PUT /chat/v1/devices/{deviceId}", h.putDevice)
	r.HandleFunc("DELETE /chat/v1/devices/{deviceId}", h.deleteDevice)
	r.HandleFunc("POST /chat/v1/devices/{deviceId}/heartbeat", h.heartbeatDevice)
	return r
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

func (h *Handler) exchangeEnrollment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &request) {
		return
	}
	if request.Code != h.enrollCode {
		writeError(w, http.StatusUnauthorized, "invalid_enrollment", "invalid enrollment code")
		return
	}
	token, expiresIn := h.tokens.Issue(h.participantID, h.participantID, "user")
	writeJSON(w, http.StatusOK, map[string]any{
		"token":            token,
		"expiresInSeconds": expiresIn,
		"profile": map[string]any{
			"code":        h.participantID,
			"displayName": h.participantID,
			"role":        "user",
		},
	})
}

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	archived := r.URL.Query().Get("archived") == "true"
	items, err := h.core.List(r.Context(), participant, archived)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_conversations", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": items})
}

func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	var request struct {
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&request)
	c, err := h.core.Create(r.Context(), participant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_conversation", err.Error())
		return
	}
	if strings.TrimSpace(request.Title) != "" {
		c, err = h.core.UpdateTitle(r.Context(), participant, c.ID, request.Title)
		if h.metadataError(w, err) {
			return
		}
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handler) updateConversation(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	var request struct {
		Title string `json:"title"`
	}
	if !decode(w, r, &request) {
		return
	}
	session, err := h.core.UpdateTitle(r.Context(), participant, r.PathValue("code"), request.Title)
	if h.metadataError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	members, err := h.core.ListMembers(r.Context(), participant, r.PathValue("code"))
	if h.memberError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	var request struct {
		ProfileID string `json:"profileId"`
	}
	if !decode(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.ProfileID) == "" {
		writeError(w, http.StatusUnprocessableEntity, "profile_id_required", "profileId is required")
		return
	}
	profileID := strings.TrimSpace(request.ProfileID)
	if err := h.core.AddMember(r.Context(), participant, r.PathValue("code"), profileID); h.memberError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"profileId": profileID})
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	if err := h.core.RemoveMember(r.Context(), participant, r.PathValue("code"), r.PathValue("profileId")); h.memberError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) memberError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, chatcore.ErrMembersNotSupported):
		writeError(w, http.StatusNotImplemented, "members_not_supported", err.Error())
	case errors.Is(err, chatcore.ErrMemberNotFound):
		writeError(w, http.StatusNotFound, "member_not_found", err.Error())
	case errors.Is(err, chatcore.ErrMemberExists):
		writeError(w, http.StatusConflict, "member_already_present", err.Error())
	case errors.Is(err, chatcore.ErrLastMember):
		writeError(w, http.StatusConflict, "last_member", "cannot remove the last member")
	case errors.Is(err, chatcore.ErrNotFound):
		writeError(w, http.StatusNotFound, "conversation_not_found", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "members", err.Error())
	}
	return true
}

func (h *Handler) metadataError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, chatcore.ErrMetadataNotSupported):
		writeError(w, http.StatusNotImplemented, "metadata_not_supported", err.Error())
	case errors.Is(err, chatcore.ErrTitleRequired):
		writeError(w, http.StatusUnprocessableEntity, "title_required", "title is required")
	case errors.Is(err, chatcore.ErrNotFound):
		writeError(w, http.StatusNotFound, "conversation_not_found", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "conversation_metadata", err.Error())
	}
	return true
}

func (h *Handler) asyncMessage(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
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
	code := r.PathValue("code")
	baseline, err := h.core.Cursor(r.Context(), participant, code)
	if h.deliveryError(w, err) {
		return
	}
	requestID := uuid.NewString()
	event, err := h.core.Publish(r.Context(), participant, code, "client", request.Message, requestID)
	if h.deliveryError(w, err) {
		return
	}
	h.scheduleMessagePush(r.Context(), participant, code, event)
	writeJSON(w, http.StatusAccepted, map[string]any{"messageId": requestID, "cursor": baseline})
}

func (h *Handler) plainMessage(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
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
	code := r.PathValue("code")
	event, err := h.core.Publish(r.Context(), participant, code, "client", request.Message, "")
	if h.deliveryError(w, err) {
		return
	}
	h.scheduleMessagePush(r.Context(), participant, code, event)
	writeJSON(w, http.StatusCreated, event)
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) (string, bool) {
	if h.identityFromContext {
		id := participantFromContext(r.Context())
		if id == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "chat participant missing from context")
			return "", false
		}
		return id, true
	}
	if id := participantFromContext(r.Context()); id != "" {
		return id, true
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "bearer token is required")
		return "", false
	}
	if h.tokens != nil {
		profile, ok := h.tokens.Resolve(token)
		if ok {
			return profile.Code, true
		}
	}
	if h.bearerToken != "" && token != h.bearerToken {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired bearer token")
		return "", false
	}
	return h.participantID, true
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
