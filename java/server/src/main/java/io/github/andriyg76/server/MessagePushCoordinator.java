package io.github.andriyg76.server;

import io.github.andriyg76.core.ChatCore;
import io.github.andriyg76.protocol.ChatEvent;
import io.github.andriyg76.protocol.EventKind;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Executor;

final class MessagePushCoordinator {

    private static final int PREVIEW_MAX = 120;

    private final ChatCore core;
    private final DeviceStore devices;
    private final InboxPresence inboxPresence;
    private final PushNotifier pushNotifier;
    private final Executor executor;

    MessagePushCoordinator(
            ChatCore core,
            DeviceStore devices,
            InboxPresence inboxPresence,
            PushNotifier pushNotifier,
            Executor executor) {
        this.core = core;
        this.devices = devices;
        this.inboxPresence = inboxPresence;
        this.pushNotifier = pushNotifier;
        this.executor = executor;
    }

    void schedule(String sender, String conversationCode, ChatEvent event) {
        if (devices == null || pushNotifier == null || event == null) {
            return;
        }
        if (!EventKind.MESSAGE.equals(event.kind())) {
            return;
        }
        List<String> recipients = core.deliveryRecipients(sender, conversationCode);
        List<String> filtered = new ArrayList<>();
        for (String profileId : recipients) {
            if (profileId.equals(sender)) {
                continue;
            }
            if (inboxPresence != null && inboxPresence.isConnected(profileId)) {
                continue;
            }
            filtered.add(profileId);
        }
        if (filtered.isEmpty()) {
            return;
        }
        List<RegisteredDevice> targets = devices.listActiveByProfiles(filtered);
        if (targets.isEmpty()) {
            return;
        }
        String preview = truncate(event.text());
        executor.execute(() -> {
            Map<String, List<RegisteredDevice>> byProfile = new LinkedHashMap<>();
            for (RegisteredDevice d : targets) {
                byProfile.computeIfAbsent(d.profileId(), ignored -> new ArrayList<>()).add(d);
            }
            for (Map.Entry<String, List<RegisteredDevice>> entry : byProfile.entrySet()) {
                long cursor = core.inboxCursor(entry.getKey());
                PushPayload payload = new PushPayload(
                        "chat_message",
                        conversationCode,
                        event.messageId(),
                        cursor > 0 ? cursor : null,
                        event.sender(),
                        preview);
                pushNotifier.notify(entry.getValue(), payload);
            }
        });
    }

    private static String truncate(String text) {
        if (text == null) {
            return "";
        }
        if (text.length() <= PREVIEW_MAX) {
            return text;
        }
        return text.substring(0, PREVIEW_MAX);
    }
}
