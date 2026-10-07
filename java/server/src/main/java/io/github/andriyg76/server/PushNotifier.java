package io.github.andriyg76.server;

import java.util.List;

public interface PushNotifier {

    void notify(List<RegisteredDevice> devices, PushPayload payload);
}
