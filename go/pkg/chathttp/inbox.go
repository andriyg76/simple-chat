package chathttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/andriyg76/simple-chat/go/pkg/chatcore"
)

func (h *Handler) inboxCursor(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	cursor, err := h.core.InboxCursor(r.Context(), participant)
	if h.inboxError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"inboxCursor": cursor})
}

func (h *Handler) inboxFeed(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	cursor, err := parseCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_inbox_cursor", err.Error())
		return
	}
	limit := 1
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 {
			writeError(w, http.StatusBadRequest, "invalid_inbox_cursor", "invalid limit")
			return
		}
	}
	wait := 30 * time.Second
	if raw := r.URL.Query().Get("wait"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds < 0 {
			writeError(w, http.StatusBadRequest, "invalid_inbox_cursor", "invalid wait")
			return
		}
		wait = time.Duration(seconds) * time.Second
	}
	events, advanced, err := h.core.InboxNext(r.Context(), participant, cursor, limit, wait)
	if h.inboxError(w, err) {
		return
	}
	if advanced > 0 {
		cursor = advanced
	}
	writeJSON(w, http.StatusOK, map[string]any{"inboxCursor": cursor, "events": events})
}

func (h *Handler) inboxEvents(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	h.streamInbox(w, r, participant)
}

func (h *Handler) streamInbox(w http.ResponseWriter, r *http.Request, participant string) {
	if h.inboxPresence != nil {
		h.inboxPresence.Connect(participant)
		defer h.inboxPresence.Disconnect(participant)
	}
	cursor, err := parseCursor(r.Header.Get("Last-Event-ID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_inbox_cursor", err.Error())
		return
	}
	if r.Header.Get("Last-Event-ID") == "" {
		cursor, err = parseCursor(r.URL.Query().Get("cursor"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_inbox_cursor", err.Error())
			return
		}
	}
	events, advanced, err := h.core.InboxNext(r.Context(), participant, cursor, 1, 0)
	if h.inboxError(w, err) {
		return
	}
	if advanced > cursor {
		cursor = advanced
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
	found := len(events) > 0
	for {
		if !found {
			events, advanced, err = h.core.InboxNext(r.Context(), participant, cursor, 1, heartbeatInterval)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				if errors.Is(err, chatcore.ErrInboxCursorExpired) {
					fmt.Fprint(w, "event: error\ndata: {\"error\":\"inbox_cursor_expired\"}\n\n")
					flusher.Flush()
				}
				return
			}
			if advanced > cursor {
				cursor = advanced
			}
			if len(events) == 0 {
				if !h.stillAuthenticated(r, participant) {
					return
				}
				if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
					return
				}
				flusher.Flush()
				continue
			}
		}
		event := events[0]
		events = events[1:]
		found = len(events) > 0
		if event.Kind == "reply" && event.Text != "" {
			cursor = event.InboxCursor
			found = false
			continue
		}
		if !h.stillAuthenticated(r, participant) {
			return
		}
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.InboxCursor, event.Kind, data); err != nil {
			return
		}
		flusher.Flush()
		cursor = event.InboxCursor
		found = false
	}
}

func (h *Handler) inboxError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, chatcore.ErrInboxCursorExpired):
		writeError(w, http.StatusConflict, "inbox_cursor_expired", err.Error())
	case errors.Is(err, chatcore.ErrInvalidCursor):
		writeError(w, http.StatusBadRequest, "invalid_inbox_cursor", err.Error())
	case errors.Is(err, context.Canceled):
		return true
	default:
		writeError(w, http.StatusInternalServerError, "inbox_delivery", err.Error())
	}
	return true
}
