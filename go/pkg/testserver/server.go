// Package testserver starts an in-memory /chat/v1 harness (health, enrollment).
package testserver

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/andriyg76/simple-chat/go/pkg/chatcore"
	"github.com/andriyg76/simple-chat/go/pkg/chathttp"
	"github.com/andriyg76/simple-chat/go/pkg/session"
)

// Config matches the Java test-server defaults (token, enroll code, participant id).
type Config struct {
	Token           string
	EnrollCode      string
	ParticipantID   string
	AdminToken      string
	EmbeddedProfile bool
	// EchoBot replies "echo: <text>" to every message in the participant's conversations.
	EchoBot bool
}

// Instance is a running harness.
type Instance struct {
	BaseURL    string
	Token      string
	EnrollCode string
	HTTP       *http.Server
	stopBot    context.CancelFunc
}

// DefaultConfig returns dev-style credentials.
func DefaultConfig() Config {
	return Config{Token: "dev", EnrollCode: "dev", ParticipantID: "default"}
}

// Start listens on 127.0.0.1 with an ephemeral port.
func Start(cfg Config) (*Instance, error) {
	return listen(cfg, "127.0.0.1:0")
}

// StartOnAddr listens on addr (e.g. ":8080" or "127.0.0.1:0").
func StartOnAddr(cfg Config, addr string) (*Instance, error) {
	return listen(cfg, addr)
}

func listen(cfg Config, addr string) (*Instance, error) {
	cfg = withDefaults(cfg)
	store := session.NewMemory()
	admin := cfg.AdminToken
	if admin == "" {
		admin = "admin"
	}
	handler := chathttp.New(store, chathttp.Config{
		BearerToken:         cfg.Token,
		EnrollCode:          cfg.EnrollCode,
		ParticipantID:       cfg.ParticipantID,
		AdminToken:          admin,
		IdentityFromContext: cfg.EmbeddedProfile,
	})
	chatRoutes := handler.Routes()
	if cfg.EmbeddedProfile {
		chatRoutes = chathttp.EmbeddedProfileMiddleware(chatRoutes)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// chatRoutes registers /chat/v1/* and /admin/chat/* (token rotation, etc.).
	mux.Handle("/", chatRoutes)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	host := ln.Addr().String()
	baseURL := "http://" + host
	if strings.HasPrefix(host, ":") {
		baseURL = "http://127.0.0.1" + host
	}
	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 35 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		_ = srv.Serve(ln)
	}()
	inst := &Instance{BaseURL: strings.TrimRight(baseURL, "/"), Token: cfg.Token, EnrollCode: cfg.EnrollCode, HTTP: srv}
	if cfg.EchoBot {
		ctx, cancel := context.WithCancel(context.Background())
		inst.stopBot = cancel
		go runEchoBot(ctx, handler.Core(), cfg.ParticipantID)
	}
	return inst, nil
}

// Protocol senders are "client" or "server"; the bot speaks as the server.
const echoBotSender = "server"

func runEchoBot(ctx context.Context, core *chatcore.Core, participant string) {
	cursor, _ := core.InboxCursor(ctx, participant)
	for ctx.Err() == nil {
		events, next, err := core.InboxNext(ctx, participant, cursor, 50, 25*time.Second)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("echo bot: inbox read failed: %v", err)
				cursor, _ = core.InboxCursor(ctx, participant)
			}
			continue
		}
		cursor = next
		for _, event := range events {
			if event.Kind != "message" || event.Sender == echoBotSender {
				continue
			}
			if _, err := core.Publish(ctx, participant, event.SessionID, echoBotSender, "echo: "+event.Text, ""); err != nil {
				log.Printf("echo bot: reply to %s failed: %v", event.SessionID, err)
			}
		}
	}
}

func withDefaults(cfg Config) Config {
	if cfg.Token == "" {
		cfg.Token = "dev"
	}
	if cfg.EnrollCode == "" {
		cfg.EnrollCode = "dev"
	}
	if cfg.ParticipantID == "" {
		cfg.ParticipantID = "default"
	}
	return cfg
}

// Shutdown stops the harness.
func (i *Instance) Shutdown(ctx context.Context) error {
	if i == nil || i.HTTP == nil {
		return nil
	}
	if i.stopBot != nil {
		i.stopBot()
	}
	return i.HTTP.Shutdown(ctx)
}

// ListenLine is printed by the test-server binary for cross-language E2E discovery.
func ListenLine(baseURL, token string) string {
	return fmt.Sprintf("simple-chat go test-server listening on %s (CHAT_TOKEN=%s)", baseURL, token)
}
