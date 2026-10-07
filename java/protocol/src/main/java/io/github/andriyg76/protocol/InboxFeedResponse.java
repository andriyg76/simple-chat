package io.github.andriyg76.protocol;

import java.util.List;

public record InboxFeedResponse(long inboxCursor, List<InboxEvent> events) {
}
