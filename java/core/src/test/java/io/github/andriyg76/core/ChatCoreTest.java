package io.github.andriyg76.core;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.andriyg76.protocol.MessageSender;
import org.junit.jupiter.api.Test;

class ChatCoreTest {

    @Test
    void publishAndNext() {
        var store = new InMemoryConversationStore();
        var core = new ChatCore(store);
        var session = core.create("user-1");
        var first = core.publish("user-1", session.code(), MessageSender.CLIENT, "hello", null);
        var second = core.publish("user-1", session.code(), MessageSender.SERVER, "hi", null);
        assertEquals(1L, first.cursor());
        assertEquals(2L, second.cursor());
        var next = core.next("user-1", session.code(), 1L, 0L);
        assertTrue(next.found());
        assertEquals("hi", next.event().text());
    }

    @Test
    void deniesForeignParticipant() {
        var store = new InMemoryConversationStore();
        var core = new ChatCore(store);
        var session = core.create("user-1");
        assertThrows(ChatException.class, () -> core.publish("other", session.code(), MessageSender.CLIENT, "x", null));
    }
}
