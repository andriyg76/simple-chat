package com.simplechat.client;

public final class SimpleChatClient {

    public static void main(String[] args) {
        String base = System.getenv().getOrDefault("SIMPLE_CHAT_URL", "http://localhost:8081");
        System.out.printf("simple-chat java client (stub); target %s — see ../../protocol.md%n", base);
    }
}
