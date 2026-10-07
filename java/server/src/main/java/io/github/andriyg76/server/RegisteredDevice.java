package io.github.andriyg76.server;

import java.time.Instant;

/** Internal push target including secret token (never log full value). */
public record RegisteredDevice(
        String deviceId,
        String profileId,
        String platform,
        String pushChannel,
        String pushToken,
        String appBundleId,
        String locale,
        Instant lastSeenAt,
        boolean foreground) {
}
