package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/andriyg76/simple-chat/go/pkg/chatclient"
	"github.com/andriyg76/simple-chat/go/pkg/testserver"
)

func TestEnrollmentModeB_GoServer(t *testing.T) {
	inst, err := testserver.Start(testserver.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Shutdown(context.Background())

	client := chatclient.Client{BaseURL: inst.BaseURL, HTTP: &http.Client{Timeout: 35 * time.Second}}
	start, err := client.StartEnrollmentChallenge(context.Background(), "e2e")
	if err != nil {
		t.Fatal(err)
	}
	verifyURL := start.VerificationURL
	if !strings.Contains(verifyURL, start.Secret) {
		t.Fatalf("url %s secret %s", verifyURL, start.Secret)
	}
	req, _ := http.NewRequest(http.MethodPost, verifyURL, bytes.NewBufferString(`{"profileId":"mode-b-user"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("verify %s", resp.Status)
	}
	status, err := client.WaitEnrollmentChallenge(context.Background(), start.ChallengeID, 0)
	if err != nil || status.Status != "completed" || status.Token == "" {
		t.Fatalf("status %+v err=%v", status, err)
	}
	client.Token = status.Token
	if err := RunChatV1Scenario(context.Background(), inst.BaseURL, client.Token); err != nil {
		t.Fatal(err)
	}
}

func TestTokenRotation_GoServer(t *testing.T) {
	inst, err := testserver.Start(testserver.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Shutdown(context.Background())

	client := &chatclient.Client{BaseURL: inst.BaseURL, Token: inst.Token, HTTP: &http.Client{Timeout: 35 * time.Second}}
	inboxCursor, _ := client.InboxCursor(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = client.SubscribeInbox(ctx, inboxCursor, func(ev chatclient.InboxEvent) error {
			if ok, _ := client.ApplyTokenRotated(ev); ok {
				close(done)
				return errGotEvent
			}
			return nil
		})
	}()
	rotateReq, _ := http.NewRequest(http.MethodPost, inst.BaseURL+"/admin/chat/rotate-token", bytes.NewBufferString(`{"profileId":"default"}`))
	rotateReq.Header.Set("Authorization", "Bearer admin")
	rotateReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(rotateReq)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("timeout waiting for token_rotated")
	}
	sess, err := client.CreateWithTitle(context.Background(), "after-rotate")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Code == "" {
		t.Fatal("no session")
	}
}

func TestEmbeddedProfile_GoServer(t *testing.T) {
	cfg := testserver.DefaultConfig()
	cfg.EmbeddedProfile = true
	inst, err := testserver.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Shutdown(context.Background())

	client := chatclient.Client{
		BaseURL: inst.BaseURL,
		HTTP:    &http.Client{Timeout: 35 * time.Second},
		RequestHook: func(req *http.Request) error {
			req.Header.Set("X-Test-Profile", "embedded-user")
			return nil
		},
	}
	sess, err := client.CreateWithTitle(context.Background(), "embedded-room")
	if err != nil {
		t.Fatal(err)
	}
	members, err := client.ListMembers(context.Background(), sess.Code)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) == 0 {
		t.Fatal("expected members")
	}
}
