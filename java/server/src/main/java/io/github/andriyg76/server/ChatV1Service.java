package io.github.andriyg76.server;

import io.github.andriyg76.core.ChatCore;
import io.github.andriyg76.core.ChatException;
import io.github.andriyg76.core.ConversationStore;
import io.github.andriyg76.protocol.ChatEvent;
import io.github.andriyg76.protocol.ConversationListResponse;
import io.github.andriyg76.protocol.ConversationSession;
import io.github.andriyg76.protocol.CursorResponse;
import io.github.andriyg76.core.ConversationMembersStore;
import io.github.andriyg76.core.ConversationMetadataStore;
import io.github.andriyg76.protocol.AddMemberRequest;
import io.github.andriyg76.protocol.ConversationMembersResponse;
import io.github.andriyg76.protocol.EnrollmentExchangeResponse;
import io.github.andriyg76.protocol.PostedMessage;
import io.github.andriyg76.protocol.SendMessageRequest;
import io.github.andriyg76.protocol.DeviceHeartbeatRequest;
import io.github.andriyg76.protocol.DevicePutRequest;
import io.github.andriyg76.protocol.DeviceRecord;
import io.github.andriyg76.protocol.Profile;
import io.github.andriyg76.protocol.ProfileResponse;
import io.github.andriyg76.protocol.UpdateConversationRequest;
import java.time.Duration;
import java.util.List;
import java.util.Objects;
import java.util.UUID;
import java.util.concurrent.Executor;

/**
 * Framework-neutral application service for /chat/v1. Wire from Spring, Jersey, or servlet in the host app.
 */
public class ChatV1Service {

    private final ChatCore core;
    private final ChatAuthResolver auth;
    private final EnrollmentSettings enrollment;
    private final ConversationStore store;
    private final ConversationMembersStore members;
    private final ConversationMetadataStore metadata;
    private final DeviceStore devices;
    private final InboxPresence inboxPresence;
    private final MessagePushCoordinator messagePush;

    public ChatV1Service(ConversationStore store, ChatAuthResolver auth) {
        this(new ChatCore(store), auth, null, store, null, null, null);
    }

    public ChatV1Service(ChatCore core, ChatAuthResolver auth, EnrollmentSettings enrollment, ConversationStore store) {
        this(core, auth, enrollment, store, null, null, null);
    }

    public ChatV1Service(
            ChatCore core,
            ChatAuthResolver auth,
            EnrollmentSettings enrollment,
            ConversationStore store,
            DeviceStore devices,
            InboxPresence inboxPresence,
            PushNotifier pushNotifier) {
        this(core, auth, enrollment, store, devices, inboxPresence, pushNotifier, Runnable::run);
    }

    public ChatV1Service(
            ChatCore core,
            ChatAuthResolver auth,
            EnrollmentSettings enrollment,
            ConversationStore store,
            DeviceStore devices,
            InboxPresence inboxPresence,
            PushNotifier pushNotifier,
            Executor pushExecutor) {
        this.core = Objects.requireNonNull(core);
        this.auth = Objects.requireNonNull(auth);
        this.enrollment = enrollment;
        this.store = store;
        this.members = store instanceof ConversationMembersStore ? (ConversationMembersStore) store : null;
        this.metadata = store instanceof ConversationMetadataStore ? (ConversationMetadataStore) store : null;
        this.devices = devices;
        this.inboxPresence = inboxPresence;
        if (this.members != null) {
            core.attachMembersStore(this.members);
        }
        this.messagePush = devices != null && pushNotifier != null
                ? new MessagePushCoordinator(core, devices, inboxPresence, pushNotifier, pushExecutor)
                : null;
    }

    public ChatCore core() {
        return core;
    }

    public InboxPresence inboxPresence() {
        return inboxPresence;
    }

    public ChatParticipant requireParticipant(String bearerToken) {
        if (bearerToken == null || bearerToken.isBlank()) {
            throw ChatException.unauthorized("bearer token is required");
        }
        return auth.resolve(bearerToken.trim())
                .orElseThrow(() -> ChatException.unauthorized("invalid or expired bearer token"));
    }

    public EnrollmentExchangeResponse exchangeEnrollment(String code) {
        if (enrollment == null || enrollment.enrollCode() == null) {
            throw ChatException.unauthorized("invalid enrollment");
        }
        if (!enrollment.enrollCode().equals(code)) {
            throw ChatException.unauthorized("invalid enrollment");
        }
        ChatParticipant participant = requireParticipant(enrollment.issuedToken());
        return new EnrollmentExchangeResponse(
                enrollment.issuedToken(),
                enrollment.expiresInSeconds(),
                participant.toProfile());
    }

    public ProfileResponse currentProfile(String bearerToken) {
        return new ProfileResponse(requireParticipant(bearerToken).toProfile());
    }

    public ConversationListResponse listConversations(String bearerToken, boolean archived) {
        ChatParticipant p = requireParticipant(bearerToken);
        return new ConversationListResponse(core.list(p.id(), archived));
    }

    public ConversationSession createConversation(String bearerToken) {
        ChatParticipant p = requireParticipant(bearerToken);
        return core.create(p.id());
    }

    public ChatEvent sendPlain(String bearerToken, String conversationCode, SendMessageRequest body) {
        ChatParticipant p = requireParticipant(bearerToken);
        if (body == null || body.message() == null || body.message().trim().isEmpty()) {
            throw ChatException.messageRequired();
        }
        ChatEvent event =
                core.publish(p.id(), conversationCode, io.github.andriyg76.protocol.MessageSender.CLIENT, body.message(), null);
        if (messagePush != null) {
            messagePush.schedule(p.id(), conversationCode, event);
        }
        return event;
    }

    /** Async send — 202 semantics; delivery via SSE / long poll only. */
    public PostedMessage sendAsync(String bearerToken, String conversationCode, SendMessageRequest body) {
        ChatParticipant p = requireParticipant(bearerToken);
        if (body == null || body.message() == null || body.message().trim().isEmpty()) {
            throw ChatException.messageRequired();
        }
        long baseline = core.cursor(p.id(), conversationCode);
        String requestId = UUID.randomUUID().toString();
        ChatEvent event =
                core.publish(p.id(), conversationCode, io.github.andriyg76.protocol.MessageSender.CLIENT, body.message(), requestId);
        if (messagePush != null) {
            messagePush.schedule(p.id(), conversationCode, event);
        }
        return new PostedMessage(requestId, baseline);
    }

    public ConversationSession updateTitle(String bearerToken, String conversationCode, UpdateConversationRequest body) {
        ChatParticipant p = requireParticipant(bearerToken);
        if (metadata == null) {
            throw new ChatException("metadata_not_supported", 501, "metadata not supported");
        }
        ConversationSession session = metadata.updateTitle(p.id(), conversationCode, body == null ? null : body.title());
        String title = session.title();
        core.emitInboxToMembers(
                p.id(),
                conversationCode,
                "conversation_updated",
                "{\"title\":\"" + title.replace("\\", "\\\\").replace("\"", "\\\"") + "\"}");
        return session;
    }

    public ConversationMembersResponse listMembers(String bearerToken, String conversationCode) {
        ChatParticipant p = requireParticipant(bearerToken);
        if (members == null) {
            throw new ChatException("members_not_supported", 501, "members not supported");
        }
        return new ConversationMembersResponse(members.listMembers(p.id(), conversationCode));
    }

    public void addMember(String bearerToken, String conversationCode, AddMemberRequest body) {
        ChatParticipant p = requireParticipant(bearerToken);
        if (members == null) {
            throw new ChatException("members_not_supported", 501, "members not supported");
        }
        if (body == null || body.profileId() == null || body.profileId().isBlank()) {
            throw new ChatException("profile_id_required", 422, "profileId is required");
        }
        String profileId = body.profileId().trim();
        members.addMember(p.id(), conversationCode, profileId);
        core.emitInboxToMembers(
                p.id(), conversationCode, "member_added", "{\"profileId\":\"" + profileId + "\"}");
    }

    public void removeMember(String bearerToken, String conversationCode, String profileId) {
        ChatParticipant p = requireParticipant(bearerToken);
        if (members == null) {
            throw new ChatException("members_not_supported", 501, "members not supported");
        }
        // Captured before the removal: a member who leaves can no longer list the conversation's members.
        List<String> recipients = core.deliveryRecipients(p.id(), conversationCode);
        members.removeMember(p.id(), conversationCode, profileId);
        core.memberRemoved(conversationCode, profileId);
        core.emitInboxTo(recipients, conversationCode, "member_removed", "{\"profileId\":\"" + profileId + "\"}");
    }

    public io.github.andriyg76.protocol.InboxCursorResponse inboxCursor(String bearerToken) {
        ChatParticipant p = requireParticipant(bearerToken);
        return new io.github.andriyg76.protocol.InboxCursorResponse(core.inboxCursor(p.id()));
    }

    public io.github.andriyg76.protocol.InboxFeedResponse pollInbox(
            String bearerToken, long inboxCursor, int limit, int waitSeconds) {
        ChatParticipant p = requireParticipant(bearerToken);
        var result = core.inboxNext(
                p.id(), inboxCursor, limit, Duration.ofSeconds(waitSeconds).toMillis());
        return new io.github.andriyg76.protocol.InboxFeedResponse(result.inboxCursor(), result.events());
    }

    public DeviceRecord upsertDevice(String bearerToken, String deviceId, DevicePutRequest body) {
        ChatParticipant p = requireParticipant(bearerToken);
        if (devices == null) {
            throw new ChatException("devices_not_supported", 501, "devices not supported");
        }
        return devices.upsert(p.id(), deviceId, body);
    }

    public void deleteDevice(String bearerToken, String deviceId) {
        ChatParticipant p = requireParticipant(bearerToken);
        if (devices == null) {
            throw new ChatException("devices_not_supported", 501, "devices not supported");
        }
        devices.delete(p.id(), deviceId);
    }

    public void heartbeatDevice(String bearerToken, String deviceId, DeviceHeartbeatRequest body) {
        ChatParticipant p = requireParticipant(bearerToken);
        if (devices == null) {
            throw new ChatException("devices_not_supported", 501, "devices not supported");
        }
        boolean foreground = body != null && Boolean.TRUE.equals(body.foreground());
        devices.touch(p.id(), deviceId, foreground);
    }
}
