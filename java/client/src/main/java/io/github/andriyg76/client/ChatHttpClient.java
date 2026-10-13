package io.github.andriyg76.client;

import io.github.andriyg76.protocol.ConversationSession;
import io.github.andriyg76.protocol.Profile;
import io.github.andriyg76.protocol.ProfileRole;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;

/** JDK-only HTTP client for /chat/v1 (reference + E2E). */
public final class ChatHttpClient {

    private final String baseUrl;
    private final String token;
    private final HttpClient http;

    public ChatHttpClient(String baseUrl, String token) {
        this.baseUrl = baseUrl.endsWith("/") ? baseUrl.substring(0, baseUrl.length() - 1) : baseUrl;
        this.token = token;
        this.http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build();
    }

    public Profile currentProfile() throws IOException, InterruptedException {
        HttpResponse<String> resp = http.send(
                request(baseUrl + "/chat/v1/profile", "GET", null),
                HttpResponse.BodyHandlers.ofString());
        expect(resp, 200);
        String body = resp.body();
        int profileKey = body.indexOf("\"profile\":");
        String slice = profileKey >= 0 ? body.substring(profileKey) : body;
        String role = Json.field(slice, "role");
        ProfileRole profileRole = "admin".equals(role) ? ProfileRole.ADMIN : ProfileRole.USER;
        return new Profile(
                Json.field(slice, "code"),
                Json.field(slice, "displayName"),
                profileRole,
                null,
                null);
    }

    public void health() throws IOException, InterruptedException {
        HttpResponse<String> resp = http.send(request(baseUrl + "/health", "GET", null), HttpResponse.BodyHandlers.ofString());
        if (resp.statusCode() != 200) {
            throw new IOException("health " + resp.statusCode());
        }
    }

    public ConversationSession createWithTitle(String title) throws IOException, InterruptedException {
        HttpResponse<String> resp = http.send(
                request(baseUrl + "/chat/v1/conversations", "POST", "{}"),
                HttpResponse.BodyHandlers.ofString());
        expectCreated(resp);
        ConversationSession session = parseSession(resp.body());
        if (title != null && !title.isBlank()) {
            return updateTitle(session.code(), title);
        }
        return session;
    }

    public ConversationSession updateTitle(String code, String title) throws IOException, InterruptedException {
        String body = "{\"title\":" + Json.string(title) + "}";
        HttpResponse<String> resp = http.send(
                request(baseUrl + "/chat/v1/conversations/" + code, "PATCH", body),
                HttpResponse.BodyHandlers.ofString());
        expect(resp, 200);
        return parseSession(resp.body());
    }

    public void addMember(String code, String profileId) throws IOException, InterruptedException {
        String body = "{\"profileId\":" + Json.string(profileId) + "}";
        HttpResponse<String> resp = http.send(
                request(baseUrl + "/chat/v1/conversations/" + code + "/members", "POST", body),
                HttpResponse.BodyHandlers.ofString());
        expect(resp, 201);
    }

    public List<String> listMembers(String code) throws IOException, InterruptedException {
        HttpResponse<String> resp = http.send(
                request(baseUrl + "/chat/v1/conversations/" + code + "/members", "GET", null),
                HttpResponse.BodyHandlers.ofString());
        expect(resp, 200);
        return Json.stringArray(resp.body(), "members");
    }

    public void removeMember(String code, String profileId) throws IOException, InterruptedException {
        HttpResponse<String> resp = http.send(
                request(baseUrl + "/chat/v1/conversations/" + code + "/members/" + profileId, "DELETE", null),
                HttpResponse.BodyHandlers.ofString());
        expect(resp, 204);
    }

    public void postPlain(String code, String message) throws IOException, InterruptedException {
        String body = "{\"message\":" + Json.string(message) + "}";
        HttpResponse<String> resp = http.send(
                request(baseUrl + "/chat/v1/conversations/" + code + "/messages/plain", "POST", body),
                HttpResponse.BodyHandlers.ofString());
        expectCreated(resp);
    }

    public String waitForInboxMessage(String conversationCode, long inboxCursor, String text, Duration timeout)
            throws IOException, InterruptedException {
        long deadline = System.nanoTime() + timeout.toNanos();
        while (System.nanoTime() < deadline) {
            HttpResponse<String> resp = http.send(
                    request(baseUrl + "/chat/v1/inbox/feed?cursor=" + inboxCursor + "&limit=20&wait=25", "GET", null),
                    HttpResponse.BodyHandlers.ofString());
            expect(resp, 200);
            String advanced = Json.field(resp.body(), "inboxCursor");
            if (advanced != null && !advanced.isEmpty()) {
                inboxCursor = Long.parseLong(advanced);
            }
            for (String eventJson : Json.inboxEvents(resp.body())) {
                String sessionId = Json.field(eventJson, "sessionId");
                String kind = Json.field(eventJson, "kind");
                String got = Json.field(eventJson, "text");
                if (conversationCode.equals(sessionId) && "message".equals(kind) && text.equals(got)) {
                    return got;
                }
            }
        }
        throw new IOException("timeout waiting for message " + text);
    }

    public long inboxCursor() throws IOException, InterruptedException {
        HttpResponse<String> resp = http.send(
                request(baseUrl + "/chat/v1/inbox/cursor", "GET", null),
                HttpResponse.BodyHandlers.ofString());
        expect(resp, 200);
        return Long.parseLong(Json.field(resp.body(), "inboxCursor"));
    }

    private HttpRequest request(String uri, String method, String body) {
        HttpRequest.Builder builder = HttpRequest.newBuilder(URI.create(uri))
                .timeout(Duration.ofSeconds(35))
                .header("Authorization", "Bearer " + token);
        if (body == null) {
            builder.method(method, HttpRequest.BodyPublishers.noBody());
        } else {
            builder.header("Content-Type", "application/json")
                    .method(method, HttpRequest.BodyPublishers.ofString(body));
        }
        return builder.build();
    }

    private static void expect(HttpResponse<String> resp, int status) throws IOException {
        if (resp.statusCode() != status) {
            throw new IOException("HTTP " + resp.statusCode() + ": " + resp.body());
        }
    }

    private static void expectCreated(HttpResponse<String> resp) throws IOException {
        if (resp.statusCode() != 200 && resp.statusCode() != 201) {
            throw new IOException("HTTP " + resp.statusCode() + ": " + resp.body());
        }
    }

    private static ConversationSession parseSession(String json) {
        return new ConversationSession(
                Json.field(json, "code"),
                Json.field(json, "title"),
                null,
                null);
    }

    static final class Json {
        static String string(String value) {
            return "\"" + value.replace("\\", "\\\\").replace("\"", "\\\"") + "\"";
        }

        static String field(String json, String name) {
            String key = "\"" + name + "\":";
            int i = json.indexOf(key);
            if (i < 0) {
                return null;
            }
            i += key.length();
            if (i < json.length() && json.charAt(i) == '"') {
                int end = json.indexOf('"', i + 1);
                while (end > 0 && json.charAt(end - 1) == '\\') {
                    end = json.indexOf('"', end + 1);
                }
                return json.substring(i + 1, end);
            }
            int end = i;
            while (end < json.length() && "0123456789".indexOf(json.charAt(end)) >= 0) {
                end++;
            }
            return json.substring(i, end);
        }

        static List<String> inboxEvents(String feedJson) {
            int eventsKey = feedJson.indexOf("\"events\":");
            if (eventsKey < 0) {
                return List.of();
            }
            int start = feedJson.indexOf('[', eventsKey);
            int end = feedJson.indexOf(']', start);
            if (start < 0 || end < 0) {
                return List.of();
            }
            String inner = feedJson.substring(start + 1, end).trim();
            if (inner.isEmpty()) {
                return List.of();
            }
            List<String> out = new ArrayList<>();
            int i = 0;
            while (i < inner.length()) {
                if (inner.charAt(i) != '{') {
                    i++;
                    continue;
                }
                int depth = 0;
                int j = i;
                for (; j < inner.length(); j++) {
                    char c = inner.charAt(j);
                    if (c == '{') {
                        depth++;
                    } else if (c == '}') {
                        depth--;
                        if (depth == 0) {
                            out.add(inner.substring(i, j + 1));
                            i = j + 1;
                            break;
                        }
                    }
                }
                if (depth != 0) {
                    break;
                }
            }
            return out;
        }

        static List<String> stringArray(String json, String name) {
            String key = "\"" + name + "\":";
            int start = json.indexOf(key);
            if (start < 0) {
                return List.of();
            }
            start = json.indexOf('[', start);
            int end = json.indexOf(']', start);
            String inner = json.substring(start + 1, end);
            List<String> out = new ArrayList<>();
            for (String part : inner.split(",")) {
                part = part.trim();
                if (part.startsWith("\"") && part.endsWith("\"")) {
                    out.add(part.substring(1, part.length() - 1));
                }
            }
            return out;
        }
    }
}
