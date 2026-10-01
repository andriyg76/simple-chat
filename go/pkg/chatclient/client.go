// Package chatclient implements the chat HTTP API and SSE-first event delivery.
package chatclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL, Token string
	HTTP           *http.Client
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
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
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

func (c Client) Cursor(ctx context.Context, code string) (int64, error) {
	var out struct {
		Cursor int64 `json:"cursor"`
	}
	err := c.json(ctx, http.MethodGet, c.base(code)+"/cursor", nil, &out)
	return out.Cursor, err
}

func (c Client) Create(ctx context.Context) (Session, error) {
	var out Session
	err := c.json(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/v1/conversations", map[string]any{}, &out)
	return out, err
}

func (c Client) Post(ctx context.Context, code, text string) (Posted, error) {
	var out Posted
	err := c.json(ctx, http.MethodPost, c.base(code)+"/messages/async", map[string]string{"message": text}, &out)
	return out, err
}

// PostPlain sends a chat message without invoking an AI integration.
func (c Client) PostPlain(ctx context.Context, code, text string) (Event, error) {
	var out Event
	err := c.json(ctx, http.MethodPost, c.base(code)+"/messages/plain", map[string]string{"message": text}, &out)
	return out, err
}

// Subscribe receives events independently of Post. It reconnects SSE after a
// broken stream, then uses the existing long-poll feed if transport fails.
func (c Client) Subscribe(ctx context.Context, code string, cursor int64, receive func(Event) error) error {
	transportFailures := 0
	seenReplies := make(map[string]int64)
	deliver := func(event Event) error {
		if event.Kind == "message" && event.Sender == "server" && event.RequestID != "" {
			seenReplies[event.RequestID] = event.Cursor
		}
		if _, seen := seenReplies[event.RequestID]; event.Kind == "reply" && seen {
			delete(seenReplies, event.RequestID)
			return nil
		}
		// Replies older than the journal's replay window cannot arrive later.
		if len(seenReplies) > 100 {
			for requestID, cursor := range seenReplies {
				if cursor <= event.Cursor-100 {
					delete(seenReplies, requestID)
				}
			}
		}
		return receive(event)
	}
	for ctx.Err() == nil {
		next, opened, err := c.sse(ctx, code, cursor, func(event Event) error {
			if event.Cursor <= cursor {
				return nil
			}
			cursor = event.Cursor
			return deliver(event)
		})
		if next > cursor {
			cursor = next
		}
		if err == nil {
			transportFailures = 0
			continue
		}
		if errors.Is(err, context.Canceled) {
			return ctx.Err()
		}
		if !errors.Is(err, errTransport) {
			return err
		}
		transportFailures++
		if !opened || transportFailures >= 2 {
			return c.poll(ctx, code, cursor, deliver)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return ctx.Err()
}

var errTransport = errors.New("SSE transport unavailable")

func (c Client) sse(ctx context.Context, code string, cursor int64, receive func(Event) error) (int64, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base(code)+"/events", nil)
	if err != nil {
		return cursor, false, err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Last-Event-ID", strconv.FormatInt(cursor, 10))
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	streamHTTP := *c.http()
	streamHTTP.Timeout = 0 // a healthy SSE stream is intentionally long-lived
	resp, err := streamHTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return cursor, false, ctx.Err()
		}
		return cursor, false, fmt.Errorf("%w: %v", errTransport, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotImplemented || resp.StatusCode == http.StatusMethodNotAllowed {
		return cursor, false, errTransport
	}
	if resp.StatusCode == http.StatusNotFound {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if !bytes.Contains(raw, []byte("conversation_not_found")) {
			return cursor, false, errTransport
		}
		return cursor, false, fmt.Errorf("server returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return cursor, false, responseError(resp)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return cursor, false, fmt.Errorf("%w: unexpected content type", errTransport)
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	opened := true
	var id, kind string
	var data []string
	dispatch := func() error {
		if len(data) == 0 {
			id, kind = "", ""
			return nil
		}
		if kind == "error" && strings.Contains(strings.Join(data, "\n"), `"error":"cursor_expired"`) {
			return ErrCursorExpired
		}
		var event Event
		if err := json.Unmarshal([]byte(strings.Join(data, "\n")), &event); err != nil {
			return err
		}
		if id != "" {
			value, err := strconv.ParseInt(id, 10, 64)
			if err != nil || value != event.Cursor {
				return fmt.Errorf("invalid SSE cursor")
			}
		}
		if kind != "" && kind != event.Kind {
			return fmt.Errorf("SSE event kind mismatch")
		}
		if event.Cursor <= 0 {
			return fmt.Errorf("invalid SSE cursor")
		}
		cursor = event.Cursor
		id, kind, data = "", "", nil
		return receive(event)
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return cursor, opened, err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "id":
			id = value
		case "event":
			kind = value
		case "data":
			data = append(data, value)
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return cursor, opened, fmt.Errorf("%w: %v", errTransport, err)
	}
	if ctx.Err() != nil {
		return cursor, opened, ctx.Err()
	}
	return cursor, opened, errTransport
}

func (c Client) poll(ctx context.Context, code string, cursor int64, receive func(Event) error) error {
	for ctx.Err() == nil {
		var event Event
		err := c.json(ctx, http.MethodGet, c.base(code)+"/feed?cursor="+strconv.FormatInt(cursor, 10), nil, &event)
		if err != nil {
			return err
		}
		if event.Cursor < cursor {
			return fmt.Errorf("feed cursor moved backward")
		}
		if event.Cursor == cursor || event.Kind == "" {
			continue
		}
		cursor = event.Cursor
		if err := receive(event); err != nil {
			return err
		}
	}
	return ctx.Err()
}
