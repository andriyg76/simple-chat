package io.github.andriyg76.core;

public class ChatException extends RuntimeException {

    private final String errorCode;
    private final int httpStatus;

    public ChatException(String errorCode, int httpStatus, String message) {
        super(message);
        this.errorCode = errorCode;
        this.httpStatus = httpStatus;
    }

    public String errorCode() {
        return errorCode;
    }

    public int httpStatus() {
        return httpStatus;
    }

    public static ChatException unauthorized(String message) {
        return new ChatException("unauthorized", 401, message);
    }

    public static ChatException notFound(String message) {
        return new ChatException("conversation_not_found", 404, message);
    }

    public static ChatException messageRequired() {
        return new ChatException("message_required", 422, "message is required");
    }

    public static ChatException invalidCursor(String message) {
        return new ChatException("invalid_cursor", 400, message);
    }

    public static ChatException cursorExpired() {
        return new ChatException("cursor_expired", 409, "cursor_expired");
    }

    public static ChatException inboxCursorExpired() {
        return new ChatException("inbox_cursor_expired", 409, "inbox_cursor_expired");
    }

    public static ChatException invalidInboxCursor(String message) {
        return new ChatException("invalid_inbox_cursor", 400, message);
    }
}
