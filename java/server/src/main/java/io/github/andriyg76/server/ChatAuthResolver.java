package io.github.andriyg76.server;

import java.util.Optional;

/**
 * Product implements: validate bearer token (JWT, session, API key) and return participant id used in {@link io.github.andriyg76.core.ConversationStore}.
 */
public interface ChatAuthResolver {

    Optional<ChatParticipant> resolve(String bearerToken);
}
