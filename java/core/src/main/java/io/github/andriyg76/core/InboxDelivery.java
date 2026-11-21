package io.github.andriyg76.core;

import io.github.andriyg76.protocol.ChatEvent;
import io.github.andriyg76.protocol.InboxEvent;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.locks.Condition;
import java.util.concurrent.locks.ReentrantLock;

public final class InboxDelivery {

    private static final int JOURNAL_CAPACITY = 100;

    private final Map<String, InboxJournal> feeds = new LinkedHashMap<>();
    private final ReentrantLock feedsLock = new ReentrantLock();

    long inboxCursor(String profileId) {
        InboxJournal journal = journal(profileId);
        journal.lock.lock();
        try {
            return journal.next;
        } finally {
            journal.lock.unlock();
        }
    }

    void fanOut(ConversationStore store, ConversationMembersStore members, String actor, String conversationCode, ChatEvent event) {
        List<String> recipients = recipients(store, members, actor, conversationCode);
        InboxEvent template = toInbox(event);
        for (String profileId : recipients) {
            append(profileId, template);
        }
    }

    void emitToMembers(
            ConversationStore store,
            ConversationMembersStore members,
            String actor,
            String conversationCode,
            String kind,
            String text) {
        emitTo(recipients(store, members, actor, conversationCode), conversationCode, kind, text);
    }

    void emitTo(List<String> recipients, String conversationCode, String kind, String text) {
        InboxEvent template = new InboxEvent(0, 0, conversationCode, null, null, null, kind, text, Instant.now());
        for (String profileId : recipients) {
            append(profileId, template);
        }
    }

    InboxNextResult inboxNext(String profileId, long inboxCursor, int limit, long timeoutMillis) {
        if (inboxCursor < 0) {
            throw ChatException.invalidInboxCursor("invalid cursor");
        }
        if (limit <= 0) {
            limit = 1;
        }
        if (limit > 50) {
            limit = 50;
        }
        InboxJournal journal = journal(profileId);
        long deadline = timeoutMillis > 0 ? System.nanoTime() + timeoutMillis * 1_000_000L : 0L;
        journal.lock.lock();
        try {
            while (true) {
                if (!journal.events.isEmpty() && inboxCursor < journal.events.getFirst().inboxCursor() - 1) {
                    throw ChatException.inboxCursorExpired();
                }
                if (inboxCursor > journal.next) {
                    throw ChatException.invalidInboxCursor("invalid cursor");
                }
                List<InboxEvent> batch = new ArrayList<>();
                long advanced = inboxCursor;
                for (InboxEvent event : journal.events) {
                    if (event.inboxCursor() <= inboxCursor) {
                        continue;
                    }
                    advanced = event.inboxCursor();
                    // Redacted entries keep cursor positions stable but are never delivered.
                    if (event.kind() != null) {
                        batch.add(event);
                        if (batch.size() >= limit) {
                            break;
                        }
                    }
                }
                if (advanced > inboxCursor) {
                    return new InboxNextResult(batch, advanced);
                }
                if (timeoutMillis <= 0) {
                    return new InboxNextResult(List.of(), inboxCursor);
                }
                long waitNanos = deadline - System.nanoTime();
                if (waitNanos <= 0) {
                    return new InboxNextResult(List.of(), inboxCursor);
                }
                try {
                    journal.wake.awaitNanos(waitNanos);
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    return new InboxNextResult(List.of(), inboxCursor);
                }
            }
        } finally {
            journal.lock.unlock();
        }
    }

    /**
     * Removes a conversation's undelivered and replayable entries from a former member's journal. Entries become
     * cursor-only placeholders so existing cursors stay valid.
     */
    void redactConversation(String profileId, String conversationCode) {
        InboxJournal journal = journal(profileId);
        journal.lock.lock();
        try {
            journal.events.replaceAll(event -> conversationCode.equals(event.sessionId())
                    ? new InboxEvent(event.inboxCursor(), 0, null, null, null, null, null, null, null)
                    : event);
        } finally {
            journal.lock.unlock();
        }
    }

    public List<String> deliveryRecipients(
            ConversationStore store, ConversationMembersStore members, String actor, String conversationCode) {
        return recipients(store, members, actor, conversationCode);
    }

    private List<String> recipients(
            ConversationStore store, ConversationMembersStore members, String actor, String conversationCode) {
        if (members != null) {
            try {
                List<String> list = members.listMembers(actor, conversationCode);
                if (list != null && !list.isEmpty()) {
                    return list;
                }
            } catch (RuntimeException ignored) {
                // fall through
            }
        }
        store.get(actor, conversationCode);
        return List.of(actor);
    }

    private InboxEvent toInbox(ChatEvent event) {
        return new InboxEvent(
                0,
                event.cursor(),
                event.sessionId(),
                event.requestId(),
                event.messageId(),
                event.sender(),
                event.kind(),
                event.text(),
                event.createdAt());
    }

    private InboxEvent append(String profileId, InboxEvent template) {
        InboxJournal journal = journal(profileId);
        journal.lock.lock();
        try {
            journal.next++;
            InboxEvent event = new InboxEvent(
                    journal.next,
                    template.cursor(),
                    template.sessionId(),
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
            journal.wake.signalAll();
            return event;
        } finally {
            journal.lock.unlock();
        }
    }

    private InboxJournal journal(String profileId) {
        feedsLock.lock();
        try {
            return feeds.computeIfAbsent(profileId, ignored -> new InboxJournal());
        } finally {
            feedsLock.unlock();
        }
    }

    public record InboxNextResult(List<InboxEvent> events, long inboxCursor) {
    }

    private static final class InboxJournal {
        final ReentrantLock lock = new ReentrantLock();
        final Condition wake = lock.newCondition();
        long next;
        List<InboxEvent> events = new ArrayList<>();
    }
}
