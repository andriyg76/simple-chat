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

    @Test
    void removedMemberLosesBufferedConversationEvents() {
        var store = new InMemoryConversationStore();
        var core = new ChatCore(store);
        core.attachMembersStore(store);
        var shared = core.create("owner");
        var own = core.create("guest");
        store.addMember("owner", shared.code(), "guest");
        core.publish("owner", shared.code(), MessageSender.CLIENT, "secret 1", null);
        core.publish("guest", own.code(), MessageSender.CLIENT, "mine", null);
        core.publish("owner", shared.code(), MessageSender.CLIENT, "secret 2", null);

        store.removeMember("owner", shared.code(), "guest");
        core.memberRemoved(shared.code(), "guest");

        var feed = core.inboxNext("guest", 0L, 50, 0L);
        assertEquals(1, feed.events().size());
        assertEquals("mine", feed.events().getFirst().text());
        assertEquals(3L, feed.inboxCursor());
        assertTrue(core.inboxNext("guest", feed.inboxCursor(), 50, 0L).events().isEmpty());
        assertThrows(ChatException.class, () -> core.next("guest", shared.code(), 0L, 0L));
    }
}
