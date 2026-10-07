package chathttp

import "context"

type contextKey int

const participantContextKey contextKey = 1

// WithParticipant stores the chat profile id on the request context (embedded security profile).
func WithParticipant(ctx context.Context, participantID string) context.Context {
	return context.WithValue(ctx, participantContextKey, participantID)
}

func participantFromContext(ctx context.Context) string {
	id, _ := ctx.Value(participantContextKey).(string)
	return id
}
