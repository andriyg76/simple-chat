package io.github.andriyg76.server;

import java.util.Optional;

/**
 * Optional dev-style enrollment. Askoldova typically disables this and uses its own login.
 */
public record EnrollmentSettings(String enrollCode, String issuedToken, long expiresInSeconds) {

    public Optional<EnrollmentSettings> optional() {
        if (enrollCode == null || enrollCode.isEmpty()) {
            return Optional.empty();
        }
        return Optional.of(this);
    }
}
