package io.github.andriyg76.server;

import java.security.SecureRandom;
import java.time.Duration;
import java.time.Instant;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

public final class EnrollmentChallengeStore {

    private final SecureRandom random = new SecureRandom();
    private final Duration ttl;
    private final Map<String, Challenge> byId = new LinkedHashMap<>();
    private final Map<String, String> bySecret = new LinkedHashMap<>();

    public EnrollmentChallengeStore(Duration ttl) {
        this.ttl = ttl == null ? Duration.ofMinutes(10) : ttl;
    }

    public Challenge create() {
        String id = UUID.randomUUID().toString();
        String secret = newSecret();
        Challenge ch = new Challenge(id, secret, Instant.now().plus(ttl));
        byId.put(id, ch);
        bySecret.put(secret, id);
        return ch;
    }

    public Optional<Challenge> get(String id) {
        return Optional.ofNullable(byId.get(id));
    }

    public Optional<Challenge> bySecret(String secret) {
        String id = bySecret.get(secret);
        if (id == null) {
            return Optional.empty();
        }
        return get(id);
    }

    public void complete(Challenge ch, String token, long expiresInSeconds, ChatParticipant profile) {
        ch.complete(token, expiresInSeconds, profile);
    }

    private String newSecret() {
        byte[] bytes = new byte[16];
        random.nextBytes(bytes);
        return HexFormat.of().formatHex(bytes);
    }

    public static final class Challenge {
        private final String id;
        private final String secret;
        private final Instant expiresAt;
        private boolean completed;
        private String token;
        private long expiresInSeconds;
        private ChatParticipant profile;

        Challenge(String id, String secret, Instant expiresAt) {
            this.id = id;
            this.secret = secret;
            this.expiresAt = expiresAt;
        }

        public String id() {
            return id;
        }

        public String secret() {
            return secret;
        }

        public boolean expired() {
            return !completed && Instant.now().isAfter(expiresAt);
        }

        public int remainingSeconds() {
            if (expired()) {
                return 0;
            }
            return (int) Duration.between(Instant.now(), expiresAt).toSeconds();
        }

        public boolean completed() {
            return completed;
        }

        public String token() {
            return token;
        }

        public long expiresInSeconds() {
            return expiresInSeconds;
        }

        public ChatParticipant profile() {
            return profile;
        }

        void complete(String token, long expiresInSeconds, ChatParticipant profile) {
            this.completed = true;
            this.token = token;
            this.expiresInSeconds = expiresInSeconds;
            this.profile = profile;
        }
    }
}
