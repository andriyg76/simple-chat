package io.github.andriyg76.core;

import io.github.andriyg76.protocol.ConversationSession;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;

public final class InMemoryConversationStore implements ConversationStore, ConversationMembersStore, ConversationMetadataStore {

    private final Map<String, Room> rooms = new LinkedHashMap<>();
    private final Map<String, Map<String, ConversationSession>> byParticipant = new LinkedHashMap<>();

    @Override
    public List<ConversationSession> list(String participantId, boolean archived) {
        Map<String, ConversationSession> rooms = byParticipant.get(participantId);
        if (rooms == null) {
            return List.of();
        }
        return new ArrayList<>(rooms.values());
    }

    @Override
    public ConversationSession create(String participantId) {
        byParticipant.computeIfAbsent(participantId, ignored -> new LinkedHashMap<>());
        int n = byParticipant.get(participantId).size() + 1;
        Instant now = Instant.now();
        ConversationSession session = new ConversationSession(
                UUID.randomUUID().toString(),
                "Chat " + n,
                now,
                null);
        Room room = new Room(session, new LinkedHashSet<>(Set.of(participantId)));
        rooms.put(session.code(), room);
        byParticipant.get(participantId).put(session.code(), session);
        return session;
    }

    @Override
    public ConversationSession get(String participantId, String conversationCode) {
        Room room = rooms.get(conversationCode);
        if (room == null || !room.members.contains(participantId)) {
            throw ChatException.notFound("conversation not found");
        }
        return room.session;
    }

    @Override
    public ConversationSession updateTitle(String actorParticipantId, String conversationCode, String title) {
        Room room = requireRoom(actorParticipantId, conversationCode);
        if (title == null || title.trim().isEmpty()) {
            throw new ChatException("title_required", 422, "title is required");
        }
        ConversationSession updated = new ConversationSession(
                room.session.code(),
                title.trim(),
                room.session.lastMessageAt(),
                room.session.reviewSummary());
        room.session = updated;
        propagateSession(room);
        return updated;
    }

    @Override
    public List<String> listMembers(String actorParticipantId, String conversationCode) {
        Room room = requireRoom(actorParticipantId, conversationCode);
        return new ArrayList<>(room.members);
    }

    @Override
    public void addMember(String actorParticipantId, String conversationCode, String profileId) {
        Room room = requireRoom(actorParticipantId, conversationCode);
        if (profileId == null || profileId.isBlank()) {
            throw new ChatException("member_not_found", 404, "member not found");
        }
        if (room.members.contains(profileId)) {
            throw new ChatException("member_already_present", 409, "member already present");
        }
        room.members.add(profileId);
        byParticipant.computeIfAbsent(profileId, ignored -> new LinkedHashMap<>())
                .put(conversationCode, room.session);
    }

    @Override
    public void removeMember(String actorParticipantId, String conversationCode, String profileId) {
        Room room = requireRoom(actorParticipantId, conversationCode);
        if (!room.members.contains(profileId)) {
            throw new ChatException("member_not_found", 404, "member not found");
        }
        if (room.members.size() <= 1) {
            throw new ChatException("last_member", 409, "cannot remove the last member");
        }
        room.members.remove(profileId);
        Map<String, ConversationSession> map = byParticipant.get(profileId);
        if (map != null) {
            map.remove(conversationCode);
        }
    }

    private Room requireRoom(String actorParticipantId, String conversationCode) {
        Room room = rooms.get(conversationCode);
        if (room == null || !room.members.contains(actorParticipantId)) {
            throw ChatException.notFound("conversation not found");
        }
        return room;
    }

    private void propagateSession(Room room) {
        ConversationSession session = room.session;
        for (String profileId : room.members) {
            byParticipant.computeIfAbsent(profileId, ignored -> new LinkedHashMap<>())
                    .put(session.code(), session);
        }
    }

    private static final class Room {
        ConversationSession session;
        final Set<String> members;

        Room(ConversationSession session, Set<String> members) {
            this.session = session;
            this.members = members;
        }
    }
}
