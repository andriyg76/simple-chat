package e2e

import (
	"context"
	"testing"
)

func TestGoClient_JavaTestServer(t *testing.T) {
	baseURL, token, stop := startJavaTestServer(t)
	defer stop()
	if err := RunChatV1Scenario(context.Background(), baseURL, token); err != nil {
		t.Fatal(err)
	}
}
