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
import java.util.Optional;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class ChatTestServerConfig {

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
