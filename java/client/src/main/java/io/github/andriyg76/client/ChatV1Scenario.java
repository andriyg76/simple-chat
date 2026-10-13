package io.github.andriyg76.client;

import io.github.andriyg76.protocol.ConversationSession;
import io.github.andriyg76.protocol.Profile;
import java.io.IOException;
import java.time.Duration;
import java.util.List;

/** Shared cross-language E2E steps for any /chat/v1 test-server. */
public final class ChatV1Scenario {

    private ChatV1Scenario() {
    }

    public static void run(String baseUrl, String token) throws IOException, InterruptedException {
        ChatHttpClient client = new ChatHttpClient(baseUrl, token);
        client.health();
        Profile profile = client.currentProfile();
        if (profile.code() == null || profile.code().isBlank()
                || profile.displayName() == null || profile.displayName().isBlank()) {
            throw new IOException("profile missing code or displayName: " + profile);
        }
        ConversationSession session = client.createWithTitle("e2e-room");
        ConversationSession updated = client.updateTitle(session.code(), "E2E Room");
        if (!"E2E Room".equals(updated.title())) {
            throw new IOException("unexpected title " + updated.title());
        }
        client.addMember(session.code(), "guest-profile");
        List<String> members = client.listMembers(session.code());
        if (members.size() < 2) {
            throw new IOException("members " + members);
        }
        client.removeMember(session.code(), "guest-profile");
        long inboxCursor = client.inboxCursor();
        client.postPlain(session.code(), "hello-e2e");
        client.waitForInboxMessage(session.code(), inboxCursor, "hello-e2e", Duration.ofSeconds(25));
    }
}
