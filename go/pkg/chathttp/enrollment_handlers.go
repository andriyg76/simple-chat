package chathttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) registerEnrollmentRoutes(r *http.ServeMux) {
	if h.enrollCode != "" {
		r.HandleFunc("POST /chat/v1/enrollments/exchange", h.exchangeEnrollment)
	}
	if h.challenges != nil {
		r.HandleFunc("POST /chat/v1/enrollments/challenges", h.createChallenge)
		r.HandleFunc("GET /chat/v1/enrollments/challenges/{challengeId}", h.getChallenge)
		r.HandleFunc("GET /chat/v1/enrollments/verify/{secret}", h.verifyChallengeGet)
		r.HandleFunc("POST /chat/v1/enrollments/verify/{secret}", h.verifyChallengePost)
		if h.adminToken != "" {
			r.HandleFunc("POST /chat/v1/enrollments/challenges/{challengeId}/complete", h.completeChallenge)
		}
	}
	if h.tokens != nil {
		r.HandleFunc("POST /chat/v1/tokens/refresh", h.refreshToken)
	}
	if h.adminToken != "" {
		r.HandleFunc("POST /admin/chat/rotate-token", h.adminRotateToken)
	}
}

func (h *Handler) createChallenge(w http.ResponseWriter, r *http.Request) {
	ch := h.challenges.Create()
	verifyPath := "/chat/v1/enrollments/verify/" + ch.secret
	base := h.publicBaseURL
	if base == "" {
		base = requestBaseURL(r)
	}
	verifyURL := strings.TrimRight(base, "/") + verifyPath
	writeJSON(w, http.StatusCreated, map[string]any{
		"challengeId":      ch.id,
		"verificationUrl":  verifyURL,
		"secret":           ch.secret,
		"expiresInSeconds": ch.RemainingSeconds(),
		"pollUrl":          "/chat/v1/enrollments/challenges/" + ch.id,
	})
}

func (h *Handler) getChallenge(w http.ResponseWriter, r *http.Request) {
	ch, ok := h.challenges.Get(r.PathValue("challengeId"))
	if !ok {
		writeError(w, http.StatusNotFound, "verification_not_found", "challenge not found")
		return
	}
	wait := 0
	if raw := r.URL.Query().Get("wait"); raw != "" {
		wait, _ = strconv.Atoi(raw)
	}
	if wait > 30 {
		wait = 30
	}
	deadline := time.Now().Add(time.Duration(wait) * time.Second)
	for {
		ch, ok = h.challenges.Get(r.PathValue("challengeId"))
		if !ok {
			writeError(w, http.StatusNotFound, "verification_not_found", "challenge not found")
			return
		}
		if ch.Expired() {
			writeError(w, http.StatusGone, "enrollment_challenge_expired", "challenge expired")
			return
		}
		if ch.status == challengeCompleted {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":           "completed",
				"token":            ch.token,
				"expiresInSeconds": ch.expiresIn,
				"profile":          ch.profile,
			})
			return
		}
		if wait <= 0 || time.Now().After(deadline) {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":           "pending",
				"expiresInSeconds": ch.RemainingSeconds(),
			})
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (h *Handler) verifyChallengeGet(w http.ResponseWriter, r *http.Request) {
	ch, ok := h.challenges.BySecret(r.PathValue("secret"))
	if !ok || ch.Expired() {
		writeError(w, http.StatusNotFound, "verification_not_found", "verification not found")
		return
	}
	if ch.status == challengeCompleted {
		writeError(w, http.StatusConflict, "verification_already_used", "already verified")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><body>
<h1>Link chat client</h1>
<p>Confirm linking this device to your profile.</p>
<form method="POST">
<label>Profile id <input name="profileId" value="%s"/></label>
<button type="submit">Confirm</button>
</form></body></html>`, h.participantID)
}

func (h *Handler) verifyChallengePost(w http.ResponseWriter, r *http.Request) {
	ch, ok := h.challenges.BySecret(r.PathValue("secret"))
	if !ok || ch.Expired() {
		writeError(w, http.StatusNotFound, "verification_not_found", "verification not found")
		return
	}
	if ch.status == challengeCompleted {
		writeError(w, http.StatusConflict, "verification_already_used", "already verified")
		return
	}
	profileID, displayName := h.resolveVerifyProfile(r)
	if profileID == "" {
		writeError(w, http.StatusUnprocessableEntity, "profile_required", "profileId is required")
		return
	}
	token, expiresIn := h.tokens.Issue(profileID, displayName, "user")
	profile := Profile{Code: profileID, DisplayName: displayName, Role: "user"}
	h.challenges.Complete(ch, token, expiresIn, profile)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resolveVerifyProfile(r *http.Request) (string, string) {
	profileID := ""
	displayName := ""
	if strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		_ = r.ParseForm()
		profileID = strings.TrimSpace(r.FormValue("profileId"))
		displayName = strings.TrimSpace(r.FormValue("displayName"))
	} else {
		var body struct {
			ProfileID   string `json:"profileId"`
			DisplayName string `json:"displayName"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		profileID = strings.TrimSpace(body.ProfileID)
		displayName = strings.TrimSpace(body.DisplayName)
	}
	if profileID == "" {
		profileID = strings.TrimSpace(r.Header.Get("X-Verify-Profile"))
	}
	if displayName == "" {
		displayName = profileID
	}
	return profileID, displayName
}

func (h *Handler) completeChallenge(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdmin(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "admin token required")
		return
	}
	ch, ok := h.challenges.Get(r.PathValue("challengeId"))
	if !ok || ch.Expired() {
		writeError(w, http.StatusNotFound, "verification_not_found", "challenge not found")
		return
	}
	var body struct {
		Profile Profile `json:"profile"`
	}
	if !decode(w, r, &body) || body.Profile.Code == "" {
		writeError(w, http.StatusUnprocessableEntity, "profile_required", "profile.code is required")
		return
	}
	token, expiresIn := h.tokens.Issue(body.Profile.Code, body.Profile.DisplayName, body.Profile.Role)
	h.challenges.Complete(ch, token, expiresIn, body.Profile)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":            token,
		"expiresInSeconds": expiresIn,
		"profile":          body.Profile,
	})
}

func (h *Handler) refreshToken(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	profile := h.tokens.ProfileForID(participant)
	token, expiresIn := h.tokens.Issue(profile.Code, profile.DisplayName, profile.Role)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":            token,
		"expiresInSeconds": expiresIn,
		"profile":          profile,
	})
}

func (h *Handler) adminRotateToken(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdmin(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "admin token required")
		return
	}
	var body struct {
		ProfileID string `json:"profileId"`
	}
	if !decode(w, r, &body) {
		return
	}
	profileID := strings.TrimSpace(body.ProfileID)
	if profileID == "" {
		profileID = h.participantID
	}
	token, rotationID, expiresIn := h.tokens.Rotate(profileID)
	h.emitTokenRotated(profileID, token, rotationID, expiresIn)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":            token,
		"expiresInSeconds": expiresIn,
		"rotationId":       rotationID,
	})
}

func (h *Handler) emitTokenRotated(profileID, token, rotationID string, expiresIn int64) {
	payload, _ := json.Marshal(map[string]any{
		"token":            token,
		"expiresInSeconds": expiresIn,
		"rotationId":       rotationID,
	})
	h.core.EmitInbox(profileID, "", "token_rotated", string(payload))
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (h *Handler) checkAdmin(r *http.Request) bool {
	auth := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	return h.adminToken != "" && auth == h.adminToken
}
