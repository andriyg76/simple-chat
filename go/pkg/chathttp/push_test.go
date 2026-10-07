package chathttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andriyg76/simple-chat/go/pkg/session"
)

func TestPushNotifierExcludesSenderAndHonorsPresence(t *testing.T) {
	store := session.NewMemory()
	var mu sync.Mutex
	var notified []string
	h := New(store, Config{
		BearerToken:   "t1",
		ParticipantID: "alice",
		PushNotifier: &LoggingPushNotifier{
			OnNotify: func(devices []Device, _ PushPayload) {
				mu.Lock()
				for _, d := range devices {
					notified = append(notified, d.ProfileID)
				}
				mu.Unlock()
			},
		},
	})
	h.tokens.InstallDevToken("t2", "bob", "bob")

	ctx := context.Background()
	conv, err := h.core.Create(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.core.AddMember(ctx, "alice", conv.ID, "bob"); err != nil {
		t.Fatal(err)
	}
	reg := DeviceRegistration{Platform: "ios", PushChannel: "apns", PushToken: "tok-bob"}
	if _, err := h.devices.Upsert("bob", "dev1", reg); err != nil {
		t.Fatal(err)
	}
	if _, err := h.devices.Upsert("alice", "dev2", reg); err != nil {
		t.Fatal(err)
	}

	event, err := h.core.Publish(ctx, "alice", conv.ID, "client", "hi bob", "")
	if err != nil {
		t.Fatal(err)
	}
	h.scheduleMessagePush(ctx, "alice", conv.ID, event)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for _, id := range notified {
		if id == "alice" {
			t.Fatalf("push sent to sender: %v", notified)
		}
	}
	if len(notified) != 1 || notified[0] != "bob" {
		t.Fatalf("notified = %v", notified)
	}

	notified = nil
	h.inboxPresence.Connect("bob")
	event, err = h.core.Publish(ctx, "alice", conv.ID, "client", "again", "")
	if err != nil {
		t.Fatal(err)
	}
	h.scheduleMessagePush(ctx, "alice", conv.ID, event)
	time.Sleep(50 * time.Millisecond)
	if len(notified) != 0 {
		t.Fatalf("expected no push with inbox SSE, got %v", notified)
	}
}

func TestPutDeviceAndPublishTriggersPushForOtherMember(t *testing.T) {
	store := session.NewMemory()
	var mu sync.Mutex
	var payloads []PushPayload
	h := New(store, Config{
		BearerToken: "alice-token",
		PushNotifier: &LoggingPushNotifier{
			OnNotify: func(_ []Device, p PushPayload) {
				mu.Lock()
				payloads = append(payloads, p)
				mu.Unlock()
			},
		},
	})
	h.tokens.InstallDevToken("alice-token", "alice", "alice")
	h.tokens.InstallDevToken("bob-token", "bob", "bob")

	ctx := context.Background()
	conv, _ := h.core.Create(ctx, "alice")
	_ = h.core.AddMember(ctx, "alice", conv.ID, "bob")

	mux := h.Routes()
	putBody := `{"platform":"android","pushChannel":"fcm","pushToken":"fcm-1"}`
	req := httptest.NewRequest(http.MethodPut, "/chat/v1/devices/phone-1", strings.NewReader(putBody))
	req.Header.Set("Authorization", "Bearer bob-token")
	req.SetPathValue("deviceId", "phone-1")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("put device: %d %s", w.Code, w.Body.String())
	}

	msg := `{"message":"hello"}`
	req = httptest.NewRequest(http.MethodPost, "/chat/v1/conversations/"+conv.ID+"/messages/plain", strings.NewReader(msg))
	req.Header.Set("Authorization", "Bearer alice-token")
	req.SetPathValue("code", conv.ID)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("publish: %d %s", w.Code, w.Body.String())
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(payloads) != 1 || payloads[0].Preview != "hello" || payloads[0].Type != "chat_message" {
		t.Fatalf("payloads = %+v", payloads)
	}
}
