package io.github.andriyg76.protocol;

import java.util.List;

public record ConversationListResponse(List<ConversationSession> conversations) {
}
