package com.simplechat.server;

import com.sun.net.httpserver.HttpServer;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;

public final class SimpleChatServer {

    public static void main(String[] args) throws IOException {
        int port = 8081;
        String portEnv = System.getenv("SIMPLE_CHAT_PORT");
        if (portEnv != null && !portEnv.isEmpty()) {
            port = Integer.parseInt(portEnv);
        }
        HttpServer server = HttpServer.create(new InetSocketAddress(port), 0);
        server.createContext("/health", exchange -> {
            byte[] body = "ok".getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(200, body.length);
            try (OutputStream out = exchange.getResponseBody()) {
                out.write(body);
            }
        });
        server.start();
        System.out.printf("simple-chat java server listening on :%d (stub; see ../../protocol.md)%n", port);
    }
}
