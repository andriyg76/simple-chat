package io.github.andriyg76.core;

import java.util.List;

public interface ConversationMembersStore {

    List<String> listMembers(String actorParticipantId, String conversationCode);

    void addMember(String actorParticipantId, String conversationCode, String profileId);

    void removeMember(String actorParticipantId, String conversationCode, String profileId);
}
