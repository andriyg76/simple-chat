package io.github.andriyg76.testserver;

import io.github.andriyg76.core.ChatCore;
import io.github.andriyg76.core.InMemoryConversationStore;
import io.github.andriyg76.protocol.ProfileRole;
import io.github.andriyg76.server.ChatAuthResolver;
import io.github.andriyg76.server.ChatParticipant;
import io.github.andriyg76.server.ChatV1Service;
import io.github.andriyg76.server.DeviceStore;
import io.github.andriyg76.server.EnrollmentSettings;
import io.github.andriyg76.server.InMemoryDeviceStore;
import io.github.andriyg76.server.InboxPresence;
import io.github.andriyg76.server.LoggingPushNotifier;
import io.github.andriyg76.server.PushNotifier;
import com.fasterxml.jackson.annotation.JsonValue;
import java.util.Optional;
import org.springframework.boot.autoconfigure.jackson.Jackson2ObjectMapperBuilderCustomizer;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class ChatTestServerConfig {

    /** The protocol module is JDK-only; serialize ProfileRole as its wire value ("user", "admin"). */
    @Bean
    Jackson2ObjectMapperBuilderCustomizer profileRoleWire() {
        return builder -> builder.mixIn(ProfileRole.class, ProfileRoleWire.class);
    }

    private abstract static class ProfileRoleWire {
        @JsonValue
        abstract String wire();
    }

    @Bean
    InboxPresence inboxPresence() {
        return new InboxPresence();
    }

    @Bean
    DeviceStore deviceStore() {
        return new InMemoryDeviceStore();
    }

    @Bean
    PushNotifier pushNotifier() {
        return new LoggingPushNotifier();
    }

    @Bean
    ChatV1Service chatV1Service(
            @Value("${simplechat.token}") String token,
            @Value("${simplechat.enroll-code}") String enrollCode,
            @Value("${simplechat.participant-id}") String participantId,
            InboxPresence inboxPresence,
            DeviceStore deviceStore,
            PushNotifier pushNotifier) {
        ChatAuthResolver auth = bearer -> bearer.equals(token)
                ? Optional.of(new ChatParticipant(participantId, participantId, ProfileRole.USER))
                : Optional.empty();
        EnrollmentSettings enrollment = new EnrollmentSettings(enrollCode, token, 30L * 24 * 60 * 60);
        InMemoryConversationStore store = new InMemoryConversationStore();
        return new ChatV1Service(
                new ChatCore(store), auth, enrollment, store, deviceStore, inboxPresence, pushNotifier);
    }
}
