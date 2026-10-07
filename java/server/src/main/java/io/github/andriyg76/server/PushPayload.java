package io.github.andriyg76.server;

public record PushPayload(
        String type,
        String sessionId,
        String messageId,
        Long inboxCursor,
        String sender,
        String preview) {
}
