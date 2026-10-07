package chatclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// InboxEvent is a profile-scoped delivery item from /chat/v1/inbox.
type InboxEvent struct {
	InboxCursor int64     `json:"inboxCursor"`
	Cursor      int64     `json:"cursor"`
	SessionID   string    `json:"sessionId"`
	RequestID   string    `json:"requestId"`
	MessageID   string    `json:"messageId"`
	Sender      string    `json:"sender"`
	Kind        string    `json:"kind"`
	Text        string    `json:"text"`
	CreatedAt   time.Time `json:"createdAt"`
}

type inboxFeedResponse struct {
	InboxCursor int64        `json:"inboxCursor"`
	Events      []InboxEvent `json:"events"`
}

func (c Client) InboxCursor(ctx context.Context) (int64, error) {
	var out struct {
		InboxCursor int64 `json:"inboxCursor"`
	}
	err := c.json(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/chat/v1/inbox/cursor", nil, &out)
	return out.InboxCursor, err
}

// PollInbox long-polls the bulk inbox feed.
func (c Client) PollInbox(ctx context.Context, inboxCursor int64, limit int, wait time.Duration) (inboxFeedResponse, error) {
	target := strings.TrimRight(c.BaseURL, "/") + "/chat/v1/inbox/feed?cursor=" + strconv.FormatInt(inboxCursor, 10)
	if limit > 0 {
		target += "&limit=" + strconv.Itoa(limit)
	}
	if wait > 0 {
		target += "&wait=" + strconv.Itoa(int(wait.Seconds()))
	}
	var out inboxFeedResponse
	err := c.json(ctx, http.MethodGet, target, nil, &out)
	return out, err
}

// SubscribeInbox receives profile inbox events (SSE with feed fallback).
func (c Client) SubscribeInbox(ctx context.Context, inboxCursor int64, receive func(InboxEvent) error) error {
	transportFailures := 0
	for ctx.Err() == nil {
		next, opened, err := c.inboxSSE(ctx, inboxCursor, func(event InboxEvent) error {
			if event.InboxCursor <= inboxCursor {
				return nil
			}
			inboxCursor = event.InboxCursor
			return receive(event)
		})
		if next > inboxCursor {
			inboxCursor = next
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
			return c.pollInboxLoop(ctx, inboxCursor, receive)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return ctx.Err()
}

func (c Client) pollInboxLoop(ctx context.Context, inboxCursor int64, receive func(InboxEvent) error) error {
	for ctx.Err() == nil {
		out, err := c.PollInbox(ctx, inboxCursor, 1, 30*time.Second)
		if err != nil {
			return err
		}
		if out.InboxCursor < inboxCursor {
			return fmt.Errorf("inbox cursor moved backward")
		}
		inboxCursor = out.InboxCursor
		for _, event := range out.Events {
			if event.InboxCursor <= 0 {
				continue
			}
			if event.InboxCursor > inboxCursor {
				inboxCursor = event.InboxCursor
			}
			if err := receive(event); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

func (c Client) inboxSSE(ctx context.Context, inboxCursor int64, receive func(InboxEvent) error) (int64, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/chat/v1/inbox/events", nil)
	if err != nil {
		return inboxCursor, false, err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Last-Event-ID", strconv.FormatInt(inboxCursor, 10))
	if err := c.applyAuth(req); err != nil {
		return inboxCursor, false, err
	}
	streamHTTP := *c.http()
	streamHTTP.Timeout = 0
	resp, err := streamHTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return inboxCursor, false, ctx.Err()
		}
		return inboxCursor, false, fmt.Errorf("%w: %v", errTransport, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotImplemented || resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound {
		return inboxCursor, false, errTransport
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return inboxCursor, false, responseError(resp)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return inboxCursor, false, fmt.Errorf("%w: unexpected content type", errTransport)
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
		if kind == "error" && strings.Contains(strings.Join(data, "\n"), `"error":"inbox_cursor_expired"`) {
			return ErrCursorExpired
		}
		var event InboxEvent
		if err := json.Unmarshal([]byte(strings.Join(data, "\n")), &event); err != nil {
			return err
		}
		if id != "" {
			value, err := strconv.ParseInt(id, 10, 64)
			if err != nil || value != event.InboxCursor {
				return fmt.Errorf("invalid SSE inbox cursor")
			}
		}
		if event.InboxCursor <= 0 {
			return fmt.Errorf("invalid SSE inbox cursor")
		}
		inboxCursor = event.InboxCursor
		id, kind, data = "", "", nil
		return receive(event)
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return inboxCursor, opened, err
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
		return inboxCursor, opened, fmt.Errorf("%w: %v", errTransport, err)
	}
	if ctx.Err() != nil {
		return inboxCursor, opened, ctx.Err()
	}
	return inboxCursor, opened, errTransport
}

func (c Client) applyAuth(req *http.Request) error {
	if c.RequestHook != nil {
		return c.RequestHook(req)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return nil
}
