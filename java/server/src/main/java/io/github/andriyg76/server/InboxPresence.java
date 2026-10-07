package io.github.andriyg76.server;

import java.util.LinkedHashMap;
import java.util.Map;

public final class InboxPresence {

    private final Map<String, Integer> counts = new LinkedHashMap<>();

    public synchronized void connect(String profileId) {
        counts.merge(profileId, 1, Integer::sum);
    }

    public synchronized void disconnect(String profileId) {
        Integer n = counts.get(profileId);
        if (n == null) {
            return;
        }
        if (n <= 1) {
            counts.remove(profileId);
        } else {
            counts.put(profileId, n - 1);
        }
    }

    public synchronized boolean isConnected(String profileId) {
        Integer n = counts.get(profileId);
        return n != null && n > 0;
    }
}
