package io.github.andriyg76.server;

import io.github.andriyg76.protocol.DevicePutRequest;
import io.github.andriyg76.protocol.DeviceRecord;
import java.util.List;

public interface DeviceStore {

    DeviceRecord upsert(String profileId, String deviceId, DevicePutRequest request);

    void delete(String profileId, String deviceId);

    void touch(String profileId, String deviceId, boolean foreground);

    List<RegisteredDevice> listActiveByProfiles(List<String> profileIds);
}
