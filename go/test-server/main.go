package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/andriyg76/simple-chat/go/pkg/testserver"
)

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func main() {
	cfg := testserver.Config{
		Token:         env("CHAT_TOKEN", "dev"),
		EnrollCode:    env("CHAT_ENROLL_CODE", "dev"),
		ParticipantID: env("CHAT_PARTICIPANT_ID", "default"),
	}
	inst, err := testserver.Start(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(testserver.ListenLine(inst.BaseURL, inst.Token))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	_ = inst.Shutdown(context.Background())
}
