package io.github.andriyg76.server;

/**
 * Library module â€” no HTTP server here. Run the Spring test harness: {@code java/test-server}.
 */
public final class SimpleChatServer {

    private SimpleChatServer() {
    }

    public static void main(String[] args) {
        System.err.println("server artifact is a library. Use module test-server:");
        System.err.println("  cd java/test-server && mvn spring-boot:run");
        System.exit(1);
    }
}
