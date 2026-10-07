package io.github.andriyg76.core;

import io.github.andriyg76.protocol.ConversationSession;

public interface ConversationMetadataStore {

    ConversationSession updateTitle(String actorParticipantId, String conversationCode, String title);
}
