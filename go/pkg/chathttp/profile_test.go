package chathttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetProfileReturnsCodeAndDisplayName(t *testing.T) {
	h := New(eventStore{}, Config{BearerToken: "secret", ParticipantID: "owner"})
	h.tokens.InstallDevToken("secret", "owner", "Owner Name")
	mux := h.Routes()

	req := httptest.NewRequest(http.MethodGet, "/chat/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Profile Profile `json:"profile"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Profile.Code != "owner" || body.Profile.DisplayName != "Owner Name" {
		t.Fatalf("profile = %+v", body.Profile)
	}
}

func TestGetProfileRequiresAuth(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/chat/v1/profile", nil)
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
}

func TestGetProfileFollowsContextIdentityOverBearer(t *testing.T) {
	h := New(eventStore{}, Config{BearerToken: "secret", ParticipantID: "owner"})
	h.tokens.InstallDevToken("secret", "owner", "Owner Name")
	mux := h.Routes()

	req := httptest.NewRequest(http.MethodGet, "/chat/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req = req.WithContext(WithParticipant(req.Context(), "embedded-user"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Profile Profile `json:"profile"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Profile.Code != "embedded-user" {
		t.Fatalf("profile = %+v, want code embedded-user", body.Profile)
	}
}
