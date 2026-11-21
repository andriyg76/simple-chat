package io.github.andriyg76.testserver;

import io.github.andriyg76.core.ChatCore;
import io.github.andriyg76.core.InboxDelivery;
import io.github.andriyg76.protocol.EventKind;
import io.github.andriyg76.protocol.InboxEvent;
import io.github.andriyg76.protocol.MessageSender;
import jakarta.annotation.PostConstruct;
import jakarta.annotation.PreDestroy;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Component;

/** Replies {@code echo: <text>} as sender {@code server} to every message in the participant's conversations. */
@Component
@ConditionalOnProperty(name = "simplechat.echo-bot", havingValue = "true")
class EchoBot {

    private static final Logger log = LoggerFactory.getLogger(EchoBot.class);

    private final ChatCore core;
    private final String participantId;
    private Thread thread;

    EchoBot(ChatCore core, @Value("${simplechat.participant-id}") String participantId) {
        this.core = core;
        this.participantId = participantId;
    }

    @PostConstruct
    void start() {
        thread = Thread.ofVirtual().name("echo-bot").start(this::run);
        log.info("Echo bot replying as server to messages of {}", participantId);
    }

    @PreDestroy
    void stop() throws InterruptedException {
        thread.interrupt();
        thread.join(5_000);
    }

    private void run() {
        long cursor = core.inboxCursor(participantId);
        while (!Thread.currentThread().isInterrupted()) {
            try {
                InboxDelivery.InboxNextResult next = core.inboxNext(participantId, cursor, 50, 25_000);
                cursor = next.inboxCursor();
                for (InboxEvent event : next.events()) {
                    if (EventKind.MESSAGE.equals(event.kind()) && !MessageSender.SERVER.equals(event.sender())) {
                        core.publish(participantId, event.sessionId(), MessageSender.SERVER, "echo: " + event.text(), null);
                    }
                }
            } catch (RuntimeException e) {
                log.warn("Echo bot skipped inbox events after cursor {}: {}", cursor, e.toString());
                cursor = core.inboxCursor(participantId);
            }
        }
    }
}
