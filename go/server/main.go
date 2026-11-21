package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
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
	port, _ := strconv.Atoi(env("PORT", "8080"))
	cfg := testserver.Config{
		Token:         env("CHAT_TOKEN", "dev"),
		EnrollCode:    env("CHAT_ENROLL_CODE", "dev"),
		ParticipantID: env("CHAT_PARTICIPANT_ID", "default"),
		EchoBot:       env("CHAT_ECHO_BOT", "") == "1",
	}
	inst, err := testserver.StartOnAddr(cfg, fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("simple-chat go server on %s (in-memory; no persistence; echo bot %t)", inst.BaseURL, cfg.EchoBot)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	_ = inst.Shutdown(context.Background())
}
