package chatcore

import "context"

// DeliveryRecipients returns profile ids that receive inbox fan-out for a publish in the conversation.
func (c *Core) DeliveryRecipients(ctx context.Context, actor, conversationCode string) ([]string, error) {
	return c.deliveryRecipients(ctx, actor, conversationCode)
}
