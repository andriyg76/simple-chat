package io.github.andriyg76.server;

import io.github.andriyg76.core.ChatCore;
import io.github.andriyg76.core.ChatException;
import io.github.andriyg76.core.InboxDelivery;
import io.github.andriyg76.protocol.EventKind;
import io.github.andriyg76.protocol.InboxEvent;
import java.io.IOException;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.time.Duration;

public final class ChatInboxSseSupport {

    public static final Duration HEARTBEAT = Duration.ofSeconds(15);

    private ChatInboxSseSupport() {
    }

    public static void stream(ChatV1Service service, String bearerToken, long inboxCursor, OutputStream out)
            throws IOException {
        ChatParticipant participant = service.requireParticipant(bearerToken);
        String profileId = participant.id();
        InboxPresence presence = service.inboxPresence();
        if (presence != null) {
            presence.connect(profileId);
        }
        try {
            streamLoop(service, bearerToken, profileId, inboxCursor, out);
        } finally {
            if (presence != null) {
                presence.disconnect(profileId);
            }
        }
    }

    private static void streamLoop(
            ChatV1Service service, String bearerToken, String profileId, long inboxCursor, OutputStream out)
            throws IOException {
        ChatCore core = service.core();

        InboxDelivery.InboxNextResult pending = core.inboxNext(profileId, inboxCursor, 1, 0L);
        boolean found = !pending.events().isEmpty();
        InboxEvent event = found ? pending.events().getFirst() : null;
        long position = found ? inboxCursor : pending.inboxCursor();

        while (!Thread.currentThread().isInterrupted()) {
            if (!found) {
                InboxDelivery.InboxNextResult waited = core.inboxNext(profileId, position, 1, HEARTBEAT.toMillis());
                if (Thread.currentThread().isInterrupted()) {
                    return;
                }
                if (waited.events().isEmpty()) {
                    position = waited.inboxCursor();
                    // A revoked or expired token ends the stream at the next heartbeat.
                    if (!sameParticipant(service, bearerToken, profileId)) {
                        return;
                    }
                    writeRaw(out, ": heartbeat\n\n");
                    continue;
                }
                event = waited.events().getFirst();
                found = true;
            }
            if (event.kind() == null) {
                position = event.inboxCursor() > 0 ? event.inboxCursor() : position;
                found = false;
                continue;
            }
            if (EventKind.REPLY.equals(event.kind()) && event.text() != null && !event.text().isEmpty()) {
                position = event.inboxCursor();
                found = false;
                continue;
            }
            if (!sameParticipant(service, bearerToken, profileId)) {
                return;
            }
            writeRaw(out, "id: " + event.inboxCursor() + "\n");
            writeRaw(out, "event: " + event.kind() + "\n");
            writeRaw(out, "data: " + event.toJson() + "\n\n");
            position = event.inboxCursor();
            found = false;
        }
    }

    private static boolean sameParticipant(ChatV1Service service, String bearerToken, String profileId) {
        try {
            return profileId.equals(service.requireParticipant(bearerToken).id());
        } catch (ChatException e) {
            return false;
        }
    }

    public static void writeInboxCursorExpired(OutputStream out) throws IOException {
        writeRaw(out, "event: error\ndata: {\"error\":\"inbox_cursor_expired\"}\n\n");
    }

    private static void writeRaw(OutputStream out, String chunk) throws IOException {
        out.write(chunk.getBytes(StandardCharsets.UTF_8));
        out.flush();
    }
}
