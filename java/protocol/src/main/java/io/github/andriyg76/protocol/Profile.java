package io.github.andriyg76.protocol;

import java.time.Instant;

public record Profile(
        String code,
        String displayName,
        ProfileRole role,
        Instant createdAt,
        Instant updatedAt
) {
}
