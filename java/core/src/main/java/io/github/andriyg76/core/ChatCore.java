package io.github.andriyg76.core;

import io.github.andriyg76.protocol.ChatEvent;
import io.github.andriyg76.protocol.ConversationSession;
import io.github.andriyg76.protocol.EventKind;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.locks.Condition;
import java.util.concurrent.locks.ReentrantLock;

/**
 * Bounded in-memory delivery journal (last 100 events per conversation). Thread-safe.
 */
public final class ChatCore {

    private static final int JOURNAL_CAPACITY = 100;

    private final ConversationStore store;
    private ConversationMembersStore membersStore;
    private final InboxDelivery inbox = new InboxDelivery();
    private final Map<String, Journal> feeds = new LinkedHashMap<>();
    private final ReentrantLock feedsLock = new ReentrantLock();

    public ChatCore(ConversationStore store) {
        this.store = store;
    }

    public void attachMembersStore(ConversationMembersStore membersStore) {
        this.membersStore = membersStore;
    }

    public List<ConversationSession> list(String participantId, boolean archived) {
        return store.list(participantId, archived);
    }

    public ConversationSession create(String participantId) {
        return store.create(participantId);
    }

    public long cursor(String participantId, String conversationCode) {
        requireSession(participantId, conversationCode);
        Journal journal = journal(conversationCode);
        journal.lock.lock();
        try {
            return journal.next;
        } finally {
            journal.lock.unlock();
        }
    }

    public ChatEvent publish(String participantId, String conversationCode, String sender, String text, String requestId) {
        if (text == null || text.trim().isEmpty()) {
            throw ChatException.messageRequired();
        }
        requireSession(participantId, conversationCode);
        ChatEvent event = emit(conversationCode, new ChatEvent(
                0L,
                conversationCode,
                requestId,
                UUID.randomUUID().toString(),
                sender,
                EventKind.MESSAGE,
                text,
                Instant.now()));
        inbox.fanOut(store, membersStore, participantId, conversationCode, event);
        return event;
    }

    public long inboxCursor(String profileId) {
        return inbox.inboxCursor(profileId);
    }

    public InboxDelivery.InboxNextResult inboxNext(String profileId, long inboxCursor, int limit, long timeoutMillis) {
        return inbox.inboxNext(profileId, inboxCursor, limit, timeoutMillis);
    }

    public void emitInboxToMembers(String actor, String conversationCode, String kind, String text) {
        inbox.emitToMembers(store, membersStore, actor, conversationCode, kind, text);
    }

    /**
     * Revokes a removed member's access to buffered events: redacts the conversation from their inbox and wakes
     * readers waiting on the conversation so they re-check membership.
     */
    public void memberRemoved(String conversationCode, String profileId) {
        inbox.redactConversation(profileId, conversationCode);
        Journal journal = journal(conversationCode);
        journal.lock.lock();
        try {
            journal.signalAll();
        } finally {
            journal.lock.unlock();
        }
    }

    public List<String> deliveryRecipients(String actor, String conversationCode) {
        return inbox.deliveryRecipients(store, membersStore, actor, conversationCode);
    }

    public ChatEvent emit(String conversationCode, ChatEvent template) {
        Journal journal = journal(conversationCode);
        journal.lock.lock();
        try {
            journal.next++;
            ChatEvent event = new ChatEvent(
                    journal.next,
                    conversationCode,
                    template.requestId(),
                    template.messageId() != null ? template.messageId() : UUID.randomUUID().toString(),
                    template.sender(),
                    template.kind(),
                    template.text(),
                    template.createdAt() != null ? template.createdAt() : Instant.now());
            journal.events.add(event);
            if (journal.events.size() > JOURNAL_CAPACITY) {
                journal.events = new ArrayList<>(journal.events.subList(journal.events.size() - JOURNAL_CAPACITY, journal.events.size()));
            }
            journal.signalAll();
            return event;
        } finally {
            journal.lock.unlock();
        }
    }

    /**
     * @return event and true when found; cursor-only stub and false on timeout; throws on expired/invalid cursor
     */
    public NextResult next(String participantId, String conversationCode, long cursor, long timeoutMillis) {
        requireSession(participantId, conversationCode);
        if (cursor < 0) {
            throw ChatException.invalidCursor("invalid cursor");
        }
        Journal journal = journal(conversationCode);
        long deadline = timeoutMillis > 0 ? System.nanoTime() + timeoutMillis * 1_000_000L : 0L;
        journal.lock.lock();
        try {
            while (true) {
                if (!journal.events.isEmpty() && cursor < journal.events.getFirst().cursor() - 1) {
                    throw ChatException.cursorExpired();
                }
                if (cursor > journal.next) {
                    throw ChatException.invalidCursor("invalid cursor");
                }
                for (ChatEvent event : journal.events) {
                    if (event.cursor() > cursor) {
                        // Membership may have changed while this reader waited.
                        requireSession(participantId, conversationCode);
                        return new NextResult(event, true);
                    }
                }
                long current = journal.next;
                if (timeoutMillis <= 0) {
                    return new NextResult(new ChatEvent(current, conversationCode, null, null, null, null, null, null), false);
                }
                long waitNanos = deadline - System.nanoTime();
                if (waitNanos <= 0) {
                    return new NextResult(new ChatEvent(current, conversationCode, null, null, null, null, null, null), false);
                }
                try {
                    journal.wake.awaitNanos(waitNanos);
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    return new NextResult(
                            new ChatEvent(current, conversationCode, null, null, null, null, null, null),
                            false);
                }
            }
        } finally {
            journal.lock.unlock();
        }
    }

    private void requireSession(String participantId, String conversationCode) {
        store.get(participantId, conversationCode);
    }

    private Journal journal(String conversationCode) {
        feedsLock.lock();
        try {
            return feeds.computeIfAbsent(conversationCode, ignored -> new Journal());
        } finally {
            feedsLock.unlock();
        }
    }

    public record NextResult(ChatEvent event, boolean found) {
    }

    private static final class Journal {
        final ReentrantLock lock = new ReentrantLock();
        final Condition wake = lock.newCondition();
        long next;
        List<ChatEvent> events = new ArrayList<>();

        void signalAll() {
            wake.signalAll();
        }
    }
}
