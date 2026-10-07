package io.github.andriyg76.protocol;

public enum ProfileRole {
    USER("user"),
    ADMIN("admin");

    private final String wire;

    ProfileRole(String wire) {
        this.wire = wire;
    }

    public String wire() {
        return wire;
    }
}
