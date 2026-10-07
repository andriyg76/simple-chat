package io.github.andriyg76.protocol;

public record DevicePutRequest(
        String platform,
        String pushChannel,
        String pushToken,
        String appBundleId,
        String locale) {
}
