package io.github.andriyg76.server;

import io.github.andriyg76.core.ChatException;
import io.github.andriyg76.protocol.DevicePutRequest;
import io.github.andriyg76.protocol.DeviceRecord;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class InMemoryDeviceStore implements DeviceStore {

    private static final Duration ACTIVE_TTL = Duration.ofDays(30);

    private final Map<DeviceKey, RegisteredDevice> devices = new LinkedHashMap<>();

    @Override
    public DeviceRecord upsert(String profileId, String deviceId, DevicePutRequest request) {
        if (deviceId == null || deviceId.isBlank()) {
            throw new ChatException("device_id_required", 422, "deviceId is required");
        }
        if (request == null || request.pushToken() == null || request.pushToken().isBlank()) {
            throw new ChatException("push_token_required", 422, "pushToken is required");
        }
        validatePlatform(request.platform(), request.pushChannel());
        Instant now = Instant.now();
        RegisteredDevice stored = new RegisteredDevice(
                deviceId,
                profileId,
                request.platform(),
                request.pushChannel(),
                request.pushToken(),
                request.appBundleId(),
                request.locale(),
                now,
                false);
        devices.put(new DeviceKey(profileId, deviceId), stored);
        return toRecord(stored);
    }

    @Override
    public void delete(String profileId, String deviceId) {
        if (deviceId == null || deviceId.isBlank()) {
            throw new ChatException("device_id_required", 422, "deviceId is required");
        }
        DeviceKey key = new DeviceKey(profileId, deviceId);
        if (!devices.containsKey(key)) {
            throw new ChatException("device_not_found", 404, "device not found");
        }
        devices.remove(key);
    }

    @Override
    public void touch(String profileId, String deviceId, boolean foreground) {
        if (deviceId == null || deviceId.isBlank()) {
            throw new ChatException("device_id_required", 422, "deviceId is required");
        }
        DeviceKey key = new DeviceKey(profileId, deviceId);
        RegisteredDevice existing = devices.get(key);
        if (existing == null) {
            throw new ChatException("device_not_found", 404, "device not found");
        }
        devices.put(
                key,
                new RegisteredDevice(
                        existing.deviceId(),
                        existing.profileId(),
                        existing.platform(),
                        existing.pushChannel(),
                        existing.pushToken(),
                        existing.appBundleId(),
                        existing.locale(),
                        Instant.now(),
                        foreground));
    }

    @Override
    public List<RegisteredDevice> listActiveByProfiles(List<String> profileIds) {
        Instant cutoff = Instant.now().minus(ACTIVE_TTL);
        List<RegisteredDevice> out = new ArrayList<>();
        for (RegisteredDevice d : devices.values()) {
            if (!profileIds.contains(d.profileId())) {
                continue;
            }
            if (d.pushToken() == null || d.pushToken().isBlank() || d.lastSeenAt().isBefore(cutoff)) {
                continue;
            }
            out.add(d);
        }
        return out;
    }

    private static void validatePlatform(String platform, String pushChannel) {
        if (!"ios".equals(platform) && !"macos".equals(platform) && !"android".equals(platform)) {
            throw new ChatException("invalid_platform", 422, "invalid platform");
        }
        if (!"apns".equals(pushChannel) && !"fcm".equals(pushChannel)) {
            throw new ChatException("invalid_push_channel", 422, "invalid pushChannel");
        }
        if (("ios".equals(platform) || "macos".equals(platform)) && !"apns".equals(pushChannel)) {
            throw new ChatException("invalid_push_channel", 422, "invalid pushChannel");
        }
        if ("android".equals(platform) && !"fcm".equals(pushChannel)) {
            throw new ChatException("invalid_push_channel", 422, "invalid pushChannel");
        }
    }

    private static DeviceRecord toRecord(RegisteredDevice d) {
        return new DeviceRecord(
                d.deviceId(),
                d.profileId(),
                d.platform(),
                d.pushChannel(),
                d.lastSeenAt(),
                d.foreground() ? Boolean.TRUE : null);
    }

    private record DeviceKey(String profileId, String deviceId) {
    }
}
