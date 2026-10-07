// Package chatclient implements the chat HTTP API and SSE-first event delivery.
package chatclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL, Token string
	HTTP           *http.Client
	RequestHook    func(*http.Request) error
}

type Session struct {
	Code          string    `json:"code"`
	Title         string    `json:"title"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	ReviewSummary string    `json:"reviewSummary"`
}

type Event struct {
	Cursor    int64     `json:"cursor"`
	SessionID string    `json:"sessionId"`
	RequestID string    `json:"requestId"`
	MessageID string    `json:"messageId"`
	Sender    string    `json:"sender"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

type Posted struct {
	MessageID string `json:"messageId"`
	Cursor    int64  `json:"cursor"`
}

var ErrCursorExpired = errors.New("cursor_expired")

func (c Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}
func (c Client) base(code string) string {
	return strings.TrimRight(c.BaseURL, "/") + "/chat/v1/conversations/" + url.PathEscape(code)
}

func (c Client) request(ctx context.Context, method, target string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.applyAuth(req); err != nil {
		return nil, err
	}
	return c.http().Do(req)
}

func responseError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var envelope struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	_ = json.Unmarshal(raw, &envelope)
	if envelope.Error.Code == "cursor_expired" {
		return ErrCursorExpired
	}
	if envelope.Error.Message != "" {
		return fmt.Errorf("server returned %s: %s", resp.Status, envelope.Error.Message)
	}
	return fmt.Errorf("server returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
}

func (c Client) json(ctx context.Context, method, target string, body, dst any) error {
	resp, err := c.request(ctx, method, target, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (c Client) List(ctx context.Context) ([]Session, error) {
	var out struct {
		Conversations []Session `json:"conversations"`
	}
	err := c.json(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/chat/v1/conversations", nil, &out)
	return out.Conversations, err
}

func Exchange(ctx context.Context, server, code string, client *http.Client) (string, time.Duration, error) {
	c := Client{BaseURL: server, HTTP: client}
	var out struct {
		Token            string `json:"token"`
		ExpiresInSeconds int    `json:"expiresInSeconds"`
	}
	err := c.json(ctx, http.MethodPost, strings.TrimRight(server, "/")+"/chat/v1/enrollments/exchange", map[string]string{"code": code}, &out)
	return out.Token, time.Duration(out.ExpiresInSeconds) * time.Second, err
}

func (c Client) Create(ctx context.Context) (Session, error) {
	return c.CreateWithTitle(ctx, "")
}

func (c Client) CreateWithTitle(ctx context.Context, title string) (Session, error) {
	body := map[string]any{}
	if strings.TrimSpace(title) != "" {
		body["title"] = strings.TrimSpace(title)
	}
	var out Session
	err := c.json(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/v1/conversations", body, &out)
	return out, err
}

// Post accepts the message asynchronously (202). Use SubscribeInbox for delivery; no reply in the HTTP body.
func (c Client) Post(ctx context.Context, code, text string) (Posted, error) {
	var out Posted
	err := c.json(ctx, http.MethodPost, c.base(code)+"/messages/async", map[string]string{"message": text}, &out)
	return out, err
}

// PostPlain sync-send: publishes the client message (POST .../messages/plain).
func (c Client) PostPlain(ctx context.Context, code, text string) (Event, error) {
	var out Event
	err := c.json(ctx, http.MethodPost, c.base(code)+"/messages/plain", map[string]string{"message": text}, &out)
	return out, err
}

var errTransport = errors.New("SSE transport unavailable")
