package io.github.andriyg76.protocol;

import java.time.Instant;

public record DeviceRecord(
        String deviceId,
        String profileId,
        String platform,
        String pushChannel,
        Instant lastSeenAt,
        Boolean foreground) {
}
