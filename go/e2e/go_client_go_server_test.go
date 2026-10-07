package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/andriyg76/simple-chat/go/pkg/testserver"
)

func TestGoClient_GoTestServer(t *testing.T) {
	inst, err := testserver.Start(testserver.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = inst.Shutdown(ctx)
	}()
	if err := RunChatV1Scenario(context.Background(), inst.BaseURL, inst.Token); err != nil {
		t.Fatal(err)
	}
}
