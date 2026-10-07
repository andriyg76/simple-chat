package io.github.andriyg76.core;

import io.github.andriyg76.protocol.ConversationSession;
import java.util.List;

/**
 * Product layer implements this (JPA, Mongo, â€¦). Core uses it only for access checks and metadata.
 */
public interface ConversationStore {

    List<ConversationSession> list(String participantId, boolean archived);

    ConversationSession create(String participantId);

    ConversationSession get(String participantId, String conversationCode);
}
