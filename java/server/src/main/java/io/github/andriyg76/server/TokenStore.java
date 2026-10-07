package io.github.andriyg76.server;

import io.github.andriyg76.protocol.ProfileRole;
import java.security.SecureRandom;
import java.time.Duration;
import java.time.Instant;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Optional;

public final class TokenStore {

    private final SecureRandom random = new SecureRandom();
    private final Duration expiresIn;
    private final Duration grace;
    private final Map<String, TokenRecord> byToken = new LinkedHashMap<>();
    private final Map<String, String> primary = new LinkedHashMap<>();
    private final Map<String, Instant> graceUntil = new LinkedHashMap<>();

    public TokenStore(Duration expiresIn, Duration grace) {
        this.expiresIn = expiresIn == null ? Duration.ofDays(30) : expiresIn;
        this.grace = grace == null ? Duration.ofMinutes(2) : grace;
    }

    public void installDevToken(String token, String profileId, String displayName) {
        byToken.put(token, new TokenRecord(profileId, displayName, ProfileRole.USER, Instant.now()));
        primary.put(profileId, token);
    }

    public String issue(String profileId, String displayName, String role) {
        String token = newToken();
        byToken.put(token, new TokenRecord(profileId, displayName, parseRole(role), Instant.now()));
        primary.put(profileId, token);
        return token;
    }

    public long expiresInSeconds() {
        return expiresIn.toSeconds();
    }

    public Optional<ChatParticipant> resolve(String token) {
        Instant now = Instant.now();
        graceUntil.entrySet().removeIf(e -> now.isAfter(e.getValue()));
        for (var entry : graceUntil.entrySet()) {
            if (entry.getKey().equals(token) && now.isAfter(entry.getValue())) {
                byToken.remove(token);
            }
        }
        TokenRecord rec = byToken.get(token);
        if (rec == null) {
            return Optional.empty();
        }
        return Optional.of(new ChatParticipant(rec.profileId, rec.displayName, rec.role()));
    }

    public Rotation rotate(String profileId) {
        String old = primary.get(profileId);
        TokenRecord rec = old == null
                ? new TokenRecord(profileId, profileId, ProfileRole.USER, Instant.now())
                : byToken.get(old);
        String token = newToken();
        String rotationId = newToken();
        byToken.put(token, new TokenRecord(rec.profileId(), rec.displayName(), rec.role(), Instant.now()));
        if (old != null) {
            graceUntil.put(old, Instant.now().plus(grace));
        }
        primary.put(profileId, token);
        return new Rotation(token, rotationId, expiresIn.toSeconds());
    }

    public ChatParticipant profileFor(String profileId) {
        String tok = primary.get(profileId);
        if (tok == null) {
            return new ChatParticipant(profileId, profileId, ProfileRole.USER);
        }
        TokenRecord rec = byToken.get(tok);
        return new ChatParticipant(rec.profileId(), rec.displayName(), rec.role());
    }

    private static ProfileRole parseRole(String role) {
        if (role == null) {
            return ProfileRole.USER;
        }
        for (ProfileRole value : ProfileRole.values()) {
            if (value.wire().equalsIgnoreCase(role)) {
                return value;
            }
        }
        return ProfileRole.USER;
    }

    private String newToken() {
        byte[] bytes = new byte[24];
        random.nextBytes(bytes);
        return HexFormat.of().formatHex(bytes);
    }

    public record Rotation(String token, String rotationId, long expiresInSeconds) {
    }

    private record TokenRecord(String profileId, String displayName, ProfileRole role, Instant issuedAt) {
    }
}
