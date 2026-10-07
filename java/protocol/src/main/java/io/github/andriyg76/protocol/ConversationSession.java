package io.github.andriyg76.protocol;

import java.time.Instant;

public record ConversationSession(
        String code,
        String title,
        Instant lastMessageAt,
        String reviewSummary
) {
}
