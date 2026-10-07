package io.github.andriyg76.protocol;

import java.time.Instant;

public record InboxEvent(
        long inboxCursor,
        long cursor,
        String sessionId,
        String requestId,
        String messageId,
        String sender,
        String kind,
        String text,
        Instant createdAt
) {

    public String toJson() {
        StringBuilder json = new StringBuilder(160);
        json.append('{');
        json.append("\"inboxCursor\":").append(inboxCursor);
        json.append(",\"cursor\":").append(cursor);
        appendString(json, "sessionId", sessionId);
        appendString(json, "requestId", requestId);
        appendString(json, "messageId", messageId);
        appendString(json, "sender", sender);
        appendString(json, "kind", kind);
        appendString(json, "text", text);
        if (createdAt != null) {
            json.append(",\"createdAt\":").append(JsonEscape.string(createdAt.toString()));
        }
        json.append('}');
        return json.toString();
    }

    private static void appendString(StringBuilder json, String name, String value) {
        if (value == null) {
            return;
        }
        json.append(',').append(JsonEscape.string(name)).append(':').append(JsonEscape.string(value));
    }
}
