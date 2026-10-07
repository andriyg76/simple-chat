package io.github.andriyg76.protocol;

public record EnrollmentExchangeResponse(
        String token,
        long expiresInSeconds,
        Profile profile
) {
}
