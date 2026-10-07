package io.github.andriyg76.server;

import io.github.andriyg76.core.ChatCore;
import io.github.andriyg76.protocol.ChatEvent;
import io.github.andriyg76.protocol.EventKind;
import java.io.IOException;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.time.Duration;

/**
 * Writes protocol SSE frames to an {@link OutputStream}. Host apps may use Spring {@code SseEmitter} instead.
 */
public final class ChatSseSupport {

    public static final Duration HEARTBEAT = Duration.ofSeconds(15);

    private ChatSseSupport() {
    }

    public static void stream(
            ChatV1Service service,
            String bearerToken,
            String conversationCode,
            long cursor,
            OutputStream out) throws IOException {
        ChatParticipant participant = service.requireParticipant(bearerToken);
        String participantId = participant.id();
        ChatCore core = service.core();

        ChatCore.NextResult pending = core.next(participantId, conversationCode, cursor, 0L);
        boolean found = pending.found();
        ChatEvent event = pending.event();
        long position = cursor;

        while (!Thread.currentThread().isInterrupted()) {
            if (!found) {
                ChatCore.NextResult waited = core.next(participantId, conversationCode, position, HEARTBEAT.toMillis());
                if (Thread.currentThread().isInterrupted()) {
                    return;
                }
                if (!waited.found()) {
                    writeRaw(out, ": heartbeat\n\n");
                    continue;
                }
                event = waited.event();
                found = true;
            }
            if (event.kind() == null) {
                position = event.cursor() > 0 ? event.cursor() : position;
                found = false;
                continue;
            }
            if (EventKind.REPLY.equals(event.kind()) && event.text() != null && !event.text().isEmpty()) {
                position = event.cursor();
                found = false;
                continue;
            }
            writeRaw(out, "id: " + event.cursor() + "\n");
            writeRaw(out, "event: " + event.kind() + "\n");
            writeRaw(out, "data: " + event.toJson() + "\n\n");
            position = event.cursor();
            found = false;
        }
    }

    public static void writeCursorExpired(OutputStream out) throws IOException {
        writeRaw(out, "event: error\ndata: {\"error\":\"cursor_expired\"}\n\n");
    }

    private static void writeRaw(OutputStream out, String chunk) throws IOException {
        out.write(chunk.getBytes(StandardCharsets.UTF_8));
        out.flush();
    }
}
