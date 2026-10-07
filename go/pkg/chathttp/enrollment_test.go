package chathttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnrollmentChallengeAndVerify(t *testing.T) {
	h := testHandler()
	mux := http.NewServeMux()
	h.registerEnrollmentRoutes(mux)

	createReq := httptest.NewRequest(http.MethodPost, "/chat/v1/enrollments/challenges", nil)
	createW := httptest.NewRecorder()
	mux.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create %d %s", createW.Code, createW.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(createW.Body.Bytes(), &created)
	challengeID, _ := created["challengeId"].(string)
	secret, _ := created["secret"].(string)

	verifyReq := httptest.NewRequest(http.MethodPost, "/chat/v1/enrollments/verify/"+secret, bytes.NewBufferString(`{"profileId":"linked-user"}`))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyW := httptest.NewRecorder()
	mux.ServeHTTP(verifyW, verifyReq)
	if verifyW.Code != http.StatusNoContent {
		t.Fatalf("verify %d %s", verifyW.Code, verifyW.Body.String())
	}

	pollReq := httptest.NewRequest(http.MethodGet, "/chat/v1/enrollments/challenges/"+challengeID, nil)
	pollW := httptest.NewRecorder()
	mux.ServeHTTP(pollW, pollReq)
	if pollW.Code != http.StatusOK || !strings.Contains(pollW.Body.String(), `"status":"completed"`) {
		t.Fatalf("poll %d %s", pollW.Code, pollW.Body.String())
	}
}

func TestTokenRotateInboxEvent(t *testing.T) {
	h := testHandler()
	ctx := context.Background()
	token, _, _ := h.tokens.Rotate("owner")
	h.emitTokenRotated("owner", token, "rot-1", 3600)
	events, _, err := h.core.InboxNext(ctx, "owner", 0, 10, 0)
	if err != nil || len(events) == 0 || events[0].Kind != "token_rotated" {
		t.Fatalf("inbox %+v err=%v", events, err)
	}
}

func TestEmbeddedProfileHeader(t *testing.T) {
	h := New(eventStore{}, Config{BearerToken: "secret", ParticipantID: "owner", IdentityFromContext: true})
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.authenticate(w, r)
		if !ok || id != "from-header" {
			t.Fatalf("auth %q ok=%v", id, ok)
		}
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/chat/v1/conversations", nil)
	req = req.WithContext(WithParticipant(req.Context(), "from-header"))
	w := httptest.NewRecorder()
	EmbeddedProfileMiddleware(inner).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
}
