package io.github.andriyg76.server;

import io.github.andriyg76.core.ChatCore;
import io.github.andriyg76.core.InMemoryConversationStore;
import io.github.andriyg76.protocol.ChatEvent;
import io.github.andriyg76.protocol.EventKind;
import io.github.andriyg76.protocol.MessageSender;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;

class MessagePushCoordinatorTest {

    @Test
    void suppressesPushWhenInboxConnected() {
        InMemoryConversationStore store = new InMemoryConversationStore();
        ChatCore core = new ChatCore(store);
        core.attachMembersStore(store);
        InboxPresence presence = new InboxPresence();
        InMemoryDeviceStore devices = new InMemoryDeviceStore();
        AtomicInteger notifications = new AtomicInteger();
        PushNotifier notifier = (devs, payload) -> notifications.incrementAndGet();
        MessagePushCoordinator coordinator =
                new MessagePushCoordinator(core, devices, presence, notifier, Runnable::run);

        var session = core.create("alice");
        store.addMember("alice", session.code(), "bob");
        devices.upsert(
                "bob",
                "d1",
                new io.github.andriyg76.protocol.DevicePutRequest("ios", "apns", "tok", null, null));

        presence.connect("bob");
        ChatEvent event = new ChatEvent(
                1L,
                session.code(),
                null,
                UUID.randomUUID().toString(),
                MessageSender.CLIENT,
                EventKind.MESSAGE,
                "hi",
                Instant.now());
        coordinator.schedule("alice", session.code(), event);
        assertEquals(0, notifications.get());

        presence.disconnect("bob");
        coordinator.schedule("alice", session.code(), event);
        assertEquals(1, notifications.get());
    }
}
