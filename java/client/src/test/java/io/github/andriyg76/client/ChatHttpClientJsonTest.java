package io.github.andriyg76.client;

import static org.junit.jupiter.api.Assertions.assertEquals;

import io.github.andriyg76.protocol.InboxEvent;
import java.util.List;
import org.junit.jupiter.api.Test;

class ChatHttpClientJsonTest {

    @Test
    void fieldDecodesEscapesAndToleratesWhitespace() {
        String json = "{\n  \"profile\" : {\n    \"code\" : \"u1\",\n    \"displayName\": \"A \\u0026 \\\"B\\\"\"\n  }\n}";

        assertEquals("u1", ChatHttpClient.Json.field(json, "code"));
        assertEquals("A & \"B\"", ChatHttpClient.Json.field(json, "displayName"));
    }

    /** Chat text is arbitrary, so brackets inside messages must not split or truncate the event list. */
    @Test
    void inboxEventsIgnoreBracketsInsideStrings() {
        String feed = "{\"events\":[{\"inboxCursor\":1,\"text\":\"a ] } [ {\"},"
                + "{\"inboxCursor\":2,\"text\":\"say \\\"}]\\\"\"}],\"inboxCursor\":2}";

        List<String> events = ChatHttpClient.Json.inboxEvents(feed);

        assertEquals(2, events.size());
        assertEquals("a ] } [ {", ChatHttpClient.Json.field(events.get(0), "text"));
        assertEquals("say \"}]\"", ChatHttpClient.Json.field(events.get(1), "text"));
    }

    @Test
    void parseEventPrefersSseIdForInboxCursor() {
        String json = "{\"inboxCursor\":3,\"cursor\":2,\"sessionId\":\"s\",\"messageId\":\"m\",\"sender\":\"server\","
                + "\"kind\":\"message\",\"text\":\"hi\",\"createdAt\":\"2026-11-21T10:00:00Z\"}";

        InboxEvent event = ChatHttpClient.parseEvent(json, "7");

        assertEquals(7, event.inboxCursor());
        assertEquals("server", event.sender());
        assertEquals("hi", event.text());
    }
}
