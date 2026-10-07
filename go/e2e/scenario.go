package e2e

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andriyg76/simple-chat/go/pkg/chatclient"
)

var errGotEvent = errors.New("got event")

// RunChatV1Scenario exercises create, title, plain send, SSE delivery, and members on any compatible server.
func RunChatV1Scenario(ctx context.Context, baseURL, token string) error {
	httpClient := &http.Client{Timeout: 35 * time.Second}
	client := chatclient.Client{BaseURL: baseURL, Token: token, HTTP: httpClient}

	health, err := httpClient.Get(strings.TrimRight(baseURL, "/") + "/health")
	if err != nil {
		return err
	}
	health.Body.Close()
	if health.StatusCode != http.StatusOK {
		return fmt.Errorf("health %s", health.Status)
	}

	sess, err := client.CreateWithTitle(ctx, "e2e-room")
	if err != nil {
		return err
	}
	updated, err := client.UpdateTitle(ctx, sess.Code, "E2E Room")
	if err != nil {
		return err
	}
	if updated.Title != "E2E Room" {
		return fmt.Errorf("title %q", updated.Title)
	}

	if err := client.AddMember(ctx, sess.Code, "guest-profile"); err != nil {
		return err
	}
	members, err := client.ListMembers(ctx, sess.Code)
	if err != nil {
		return err
	}
	if len(members) < 2 {
		return fmt.Errorf("members %v", members)
	}
	if err := client.RemoveMember(ctx, sess.Code, "guest-profile"); err != nil {
		return err
	}

	inboxCursor, err := client.InboxCursor(ctx)
	if err != nil {
		inboxCursor = 0
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	var subErr error
	var once sync.Once
	done := make(chan struct{})
	go func() {
		subErr = client.SubscribeInbox(ctx, inboxCursor, func(event chatclient.InboxEvent) error {
			if event.SessionID == sess.Code && event.Kind == "message" && event.Text == "hello-e2e" {
				once.Do(func() { close(done) })
				return errGotEvent
			}
			return nil
		})
	}()
	time.Sleep(50 * time.Millisecond)
	if _, err := client.PostPlain(ctx, sess.Code, "hello-e2e"); err != nil {
		return err
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for event; subscribe=%v", subErr)
	}
}
