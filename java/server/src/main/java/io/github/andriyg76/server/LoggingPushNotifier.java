package io.github.andriyg76.server;

import java.util.List;
import java.util.logging.Logger;

/** Reference implementation: log counts only, no APNs/FCM network I/O. */
public final class LoggingPushNotifier implements PushNotifier {

    private static final Logger LOG = Logger.getLogger(LoggingPushNotifier.class.getName());

    @Override
    public void notify(List<RegisteredDevice> devices, PushPayload payload) {
        if (devices == null || devices.isEmpty()) {
            return;
        }
        LOG.fine(() -> "chat push type=" + payload.type() + " session=" + payload.sessionId()
                + " devices=" + devices.size());
    }
}
