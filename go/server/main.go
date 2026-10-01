package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agushuley/simple-chat/go/pkg/chat"
	"github.com/agushuley/simple-chat/go/pkg/chathttp"
	"github.com/agushuley/simple-chat/go/pkg/db"
	"github.com/agushuley/simple-chat/go/pkg/msgstore"
)

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func adminAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if strings.TrimSpace(bearer) != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func main() {
	port, _ := strconv.Atoi(env("PORT", "8080"))
	mongoURI := env("MONGO_URI", "mongodb://127.0.0.1:27017")
	mongoDB := env("MONGO_DB", "simple_chat")
	adminToken := env("ADMIN_TOKEN", "dev-admin-token")

	ctx := context.Background()
	database, err := db.NewMongoDB(mongoURI, mongoDB)
	if err != nil {
		log.Fatalf("mongodb: %v", err)
	}
	defer func() {
		_ = database.Disconnect(ctx)
	}()

	conversationSessions := database.Collection("conversation_sessions")
	chatService := chat.NewService(
		database.Collection("profiles"),
		database.Collection("profile_identities"),
		database.Collection("chat_enrollments"),
		database.Collection("chat_sessions"),
		conversationSessions,
	)
	if err := chatService.EnsureIndexes(ctx); err != nil {
		log.Fatalf("chat indexes: %v", err)
	}
	msgStore := msgstore.NewStore(conversationSessions)
	if err := msgStore.EnsureIndexes(ctx); err != nil {
		log.Fatalf("msgstore indexes: %v", err)
	}

	chatHandler := chathttp.New(chatService, msgStore)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /admin/chat/bootstrap", adminAuth(adminToken, chatHandler.Bootstrap))
	mux.HandleFunc("POST /admin/chat/bootstrap/reset", adminAuth(adminToken, chatHandler.ResetBootstrap))
	mux.Handle("/chat/v1/", chatHandler.Routes())

	addr := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 35 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	log.Printf("simple-chat go server on %s (mongo db %s)", addr, mongoDB)
	log.Fatal(server.ListenAndServe())
}
