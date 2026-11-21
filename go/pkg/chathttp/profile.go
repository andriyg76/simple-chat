package chathttp

import (
	"net/http"
	"strings"
)

func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]Profile{"profile": h.profileFor(participant, r)})
}

func (h *Handler) profileFor(participantID string, r *http.Request) Profile {
	if h.tokens != nil {
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if token != "" {
			// A context identity from an embedding host wins over a forwarded bearer token.
			if profile, ok := h.tokens.Resolve(token); ok && profile.Code == participantID {
				return profile
			}
		}
		return h.tokens.ProfileForID(participantID)
	}
	return Profile{Code: participantID, DisplayName: participantID, Role: "user"}
}
