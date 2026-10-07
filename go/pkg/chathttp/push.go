package chathttp

import (
	"context"
	"log"

	"github.com/andriyg76/simple-chat/go/pkg/chatcore"
)

const pushPreviewMaxLen = 120

// PushPayload is the data envelope sent to APNs/FCM (not exposed on HTTP API).
type PushPayload struct {
	Type        string `json:"type"`
	SessionID   string `json:"sessionId"`
	MessageID   string `json:"messageId"`
	InboxCursor int64  `json:"inboxCursor,omitempty"`
	Sender      string `json:"sender"`
	Preview     string `json:"preview"`
}

// PushNotifier delivers mobile push (APNs/FCM) in the product layer.
type PushNotifier interface {
	Notify(ctx context.Context, devices []Device, payload PushPayload) error
}

// LoggingPushNotifier counts deliveries without network I/O (reference test server).
type LoggingPushNotifier struct {
	OnNotify func(devices []Device, payload PushPayload)
}

func (n *LoggingPushNotifier) Notify(_ context.Context, devices []Device, payload PushPayload) error {
	if n != nil && n.OnNotify != nil {
		n.OnNotify(devices, payload)
	}
	return nil
}

func truncatePreview(text string) string {
	if len(text) <= pushPreviewMaxLen {
		return text
	}
	return text[:pushPreviewMaxLen]
}

func (h *Handler) scheduleMessagePush(ctx context.Context, sender, conversationCode string, event chatcore.Event) {
	if h.pushNotifier == nil || h.devices == nil || event.Kind != "message" {
		return
	}
	recipients, err := h.core.DeliveryRecipients(ctx, sender, conversationCode)
	if err != nil || len(recipients) == 0 {
		return
	}
	filtered := make([]string, 0, len(recipients))
	for _, profileID := range recipients {
		if profileID == sender {
			continue
		}
		if h.inboxPresence != nil && h.inboxPresence.IsConnected(profileID) {
			continue
		}
		filtered = append(filtered, profileID)
	}
	if len(filtered) == 0 {
		return
	}
	devices, err := h.devices.ListActiveByProfiles(filtered)
	if err != nil || len(devices) == 0 {
		return
	}
	base := PushPayload{
		Type:      "chat_message",
		SessionID: conversationCode,
		MessageID: event.MessageID,
		Sender:    event.Sender,
		Preview:   truncatePreview(event.Text),
	}
	go func() {
		bg := context.Background()
		byProfile := make(map[string][]Device)
		for _, d := range devices {
			byProfile[d.ProfileID] = append(byProfile[d.ProfileID], d)
		}
		for profileID, batch := range byProfile {
			payload := base
			cursor, err := h.core.InboxCursor(bg, profileID)
			if err == nil && cursor > 0 {
				payload.InboxCursor = cursor
			}
			if err := h.pushNotifier.Notify(bg, batch, payload); err != nil {
				log.Printf("chat push notify: %v", err)
			}
		}
	}()
}
