package io.github.andriyg76.server;

import io.github.andriyg76.protocol.Profile;
import io.github.andriyg76.protocol.ProfileRole;

/**
 * Resolved identity for one chat request. Askoldova maps JWT/session to this type.
 */
public record ChatParticipant(String id, String displayName, ProfileRole role) {

    public Profile toProfile() {
        return new Profile(id, displayName, role, null, null);
    }
}
