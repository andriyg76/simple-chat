package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/agushuley/simple-chat/go/pkg/chatclient"
)

func main() {
	server := flag.String("server", "http://localhost:8080", "chat server base URL")
	token := flag.String("token", "", "bearer token")
	enroll := flag.String("enroll", "", "enrollment code (with --server)")
	conversation := flag.String("conversation", "", "conversation code (create if empty)")
	flag.Parse()

	httpClient := &http.Client{Timeout: 35 * time.Second}
	if *token == "" && *enroll != "" {
		t, _, err := chatclient.Exchange(context.Background(), *server, *enroll, httpClient)
		if err != nil {
			fatal(err)
		}
		*token = t
		fmt.Println("Enrolled; token stored for this run.")
	}
	if *token == "" {
		fatal(errors.New("set --token or --enroll with --server"))
	}

	client := chatclient.Client{BaseURL: *server, Token: *token, HTTP: httpClient}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	code := *conversation
	if code == "" {
		session, err := client.Create(ctx)
		if err != nil {
			fatal(err)
		}
		code = session.Code
		fmt.Printf("conversation %s (%s)\n", code, session.Title)
	}

	cursor, err := client.Cursor(ctx, code)
	if err != nil {
		cursor = 0
	}

	go func() {
		err := client.Subscribe(ctx, code, cursor, func(event chatclient.Event) error {
			if event.Kind == "message" || event.Kind == "progress" || event.Kind == "error" {
				fmt.Printf("\n[%s] %s\n> ", event.Sender, event.Text)
			}
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "subscribe ended: %v\n", err)
			stop()
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("> ")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		text := strings.TrimSpace(line)
		if text == "" {
			fmt.Print("> ")
			continue
		}
		if text == "/quit" {
			return
		}
		_, err = client.PostPlain(ctx, code, text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "send: %v\n", err)
		}
		fmt.Print("> ")
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
