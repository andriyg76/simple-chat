package io.github.andriyg76.client;

import io.github.andriyg76.protocol.ConversationSession;
import io.github.andriyg76.protocol.EnrollmentExchangeResponse;
import io.github.andriyg76.protocol.InboxEvent;
import io.github.andriyg76.protocol.Profile;
import io.github.andriyg76.protocol.ProfileRole;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.function.Consumer;
import java.util.stream.Stream;

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

    /** Exchanges a one-time enrollment code for a bearer token; this route takes no token. */
    public static EnrollmentExchangeResponse exchange(String baseUrl, String code) throws IOException, InterruptedException {
        String base = baseUrl.endsWith("/") ? baseUrl.substring(0, baseUrl.length() - 1) : baseUrl;
        HttpRequest request = HttpRequest.newBuilder(URI.create(base + "/chat/v1/enrollments/exchange"))
                .timeout(Duration.ofSeconds(35))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString("{\"code\":" + Json.string(code) + "}"))
                .build();
        HttpResponse<String> resp = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build()
                .send(request, HttpResponse.BodyHandlers.ofString());
        expectCreated(resp);
        String body = resp.body();
        int profileKey = body.indexOf("\"profile\"");
        String profile = profileKey >= 0 ? body.substring(profileKey) : body;
        String expires = Json.field(body, "expiresInSeconds");
        return new EnrollmentExchangeResponse(
                Json.field(body, "token"),
                expires == null || expires.isEmpty() ? 0 : Long.parseLong(expires),
                new Profile(Json.field(profile, "code"), Json.field(profile, "displayName"),
                        role(Json.field(profile, "role")), null, null));
    }

    public Profile currentProfile() throws IOException, InterruptedException {
        HttpResponse<String> resp = http.send(
                request(baseUrl + "/chat/v1/profile", "GET", null),
                HttpResponse.BodyHandlers.ofString());
        expect(resp, 200);
        String body = resp.body();
        int profileKey = body.indexOf("\"profile\"");
        String slice = profileKey >= 0 ? body.substring(profileKey) : body;
        return new Profile(
                Json.field(slice, "code"),
                Json.field(slice, "displayName"),
                role(Json.field(slice, "role")),
                null,
                null);
    }

    private static ProfileRole role(String wire) throws IOException {
        for (ProfileRole role : ProfileRole.values()) {
            if (role.wire().equals(wire)) {
                return role;
            }
        }
        throw new IOException("unexpected profile role: " + wire);
    }

    public void health() throws IOException, InterruptedException {
        HttpResponse<String> resp = http.send(request(baseUrl + "/health", "GET", null), HttpResponse.BodyHandlers.ofString());
        if (resp.statusCode() != 200) {
            throw new IOException("health " + resp.statusCode());
        }
    }

    public List<ConversationSession> listConversations() throws IOException, InterruptedException {
        HttpResponse<String> resp = http.send(
                request(baseUrl + "/chat/v1/conversations", "GET", null),
                HttpResponse.BodyHandlers.ofString());
        expect(resp, 200);
        List<ConversationSession> out = new ArrayList<>();
        for (String json : Json.objects(resp.body(), "conversations")) {
            out.add(parseSession(json));
        }
        return out;
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

    /**
     * Delivers inbox events after {@code cursor} until the thread is interrupted. Uses SSE and falls back to
     * long poll when SSE is unavailable or keeps failing. Transient failures are retried; a rejected token is thrown.
     */
    public void subscribeInbox(long cursor, Consumer<InboxEvent> handler) throws IOException, InterruptedException {
        long[] position = {cursor};
        int sseFailures = 0;
        while (!Thread.currentThread().isInterrupted()) {
            try {
                if (sseFailures < 2 && streamInbox(position, handler)) {
                    sseFailures = 0;
                    continue;
                }
                sseFailures = 2;
                pollInbox(position, handler);
            } catch (HttpStatusException e) {
                if (e.status == 401 || e.status == 403) {
                    throw e;
                }
                sseFailures++;
                Thread.sleep(Duration.ofSeconds(3));
            } catch (IOException e) {
                sseFailures++;
                Thread.sleep(sseFailures < 2 ? 200 : 3000);
            }
        }
    }

    /** Returns false when the server has no SSE route; returns true when the stream ends. */
    private boolean streamInbox(long[] position, Consumer<InboxEvent> handler) throws IOException, InterruptedException {
        HttpRequest request = HttpRequest.newBuilder(URI.create(baseUrl + "/chat/v1/inbox/events"))
                .header("Authorization", "Bearer " + token)
                .header("Accept", "text/event-stream")
                .header("Last-Event-ID", Long.toString(position[0]))
                .GET()
                .build();
        HttpResponse<Stream<String>> resp = http.send(request, HttpResponse.BodyHandlers.ofLines());
        try (Stream<String> lines = resp.body()) {
            int status = resp.statusCode();
            if (status == 404 || status == 405 || status == 501) {
                return false;
            }
            if (status != 200) {
                throw new HttpStatusException(status, String.join("\n", lines.limit(20).toList()));
            }
            String id = null;
            StringBuilder data = new StringBuilder();
            Iterator<String> it = lines.iterator();
            while (it.hasNext()) {
                String line = it.next();
                if (line.isEmpty()) {
                    if (!data.isEmpty()) {
                        InboxEvent event = parseEvent(data.toString(), id);
                        if (event.inboxCursor() > position[0]) {
                            position[0] = event.inboxCursor();
                            handler.accept(event);
                        }
                    }
                    id = null;
                    data.setLength(0);
                } else if (line.startsWith("id:")) {
                    id = line.substring(3).trim();
                } else if (line.startsWith("data:")) {
                    if (!data.isEmpty()) {
                        data.append('\n');
                    }
                    data.append(line.substring(line.startsWith("data: ") ? 6 : 5));
                }
            }
        }
        return true;
    }

    private void pollInbox(long[] position, Consumer<InboxEvent> handler) throws IOException, InterruptedException {
        while (!Thread.currentThread().isInterrupted()) {
            HttpResponse<String> resp = http.send(
                    request(baseUrl + "/chat/v1/inbox/feed?cursor=" + position[0] + "&limit=50&wait=25", "GET", null),
                    HttpResponse.BodyHandlers.ofString());
            expect(resp, 200);
            for (String json : Json.inboxEvents(resp.body())) {
                InboxEvent event = parseEvent(json, null);
                if (event.inboxCursor() > position[0]) {
                    position[0] = event.inboxCursor();
                    handler.accept(event);
                }
            }
        }
    }

    static InboxEvent parseEvent(String json, String sseId) {
        String inbox = sseId != null && !sseId.isEmpty() ? sseId : Json.field(json, "inboxCursor");
        String createdAt = Json.field(json, "createdAt");
        return new InboxEvent(
                number(inbox),
                number(Json.field(json, "cursor")),
                Json.field(json, "sessionId"),
                Json.field(json, "requestId"),
                Json.field(json, "messageId"),
                Json.field(json, "sender"),
                Json.field(json, "kind"),
                Json.field(json, "text"),
                createdAt == null || createdAt.isEmpty() ? null : Instant.parse(createdAt));
    }

    private static long number(String value) {
        return value == null || value.isEmpty() ? 0 : Long.parseLong(value);
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
            throw new HttpStatusException(resp.statusCode(), resp.body());
        }
    }

    private static void expectCreated(HttpResponse<String> resp) throws IOException {
        if (resp.statusCode() != 200 && resp.statusCode() != 201) {
            throw new HttpStatusException(resp.statusCode(), resp.body());
        }
    }

    /** Non-success HTTP status from the chat server. */
    public static final class HttpStatusException extends IOException {
        private final int status;

        HttpStatusException(int status, String body) {
            super("HTTP " + status + ": " + body);
            this.status = status;
        }

        public int status() {
            return status;
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
            int i = valueStart(json, name);
            if (i < 0) {
                return null;
            }
            if (json.charAt(i) == '"') {
                return readString(json, i);
            }
            int end = i;
            while (end < json.length() && "0123456789".indexOf(json.charAt(end)) >= 0) {
                end++;
            }
            return json.substring(i, end);
        }

        /** Index of the first value character after {@code "name"} and its colon, or -1. */
        private static int valueStart(String json, String name) {
            String key = "\"" + name + "\"";
            for (int k = json.indexOf(key); k >= 0; k = json.indexOf(key, k + 1)) {
                int i = skipWhitespace(json, k + key.length());
                if (i < json.length() && json.charAt(i) == ':') {
                    i = skipWhitespace(json, i + 1);
                    return i < json.length() ? i : -1;
                }
            }
            return -1;
        }

        private static int skipWhitespace(String json, int i) {
            while (i < json.length() && Character.isWhitespace(json.charAt(i))) {
                i++;
            }
            return i;
        }

        private static String readString(String json, int quote) {
            StringBuilder out = new StringBuilder();
            for (int i = quote + 1; i < json.length(); i++) {
                char c = json.charAt(i);
                if (c == '"') {
                    return out.toString();
                }
                if (c != '\\') {
                    out.append(c);
                    continue;
                }
                char e = json.charAt(++i);
                switch (e) {
                    case 'b' -> out.append('\b');
                    case 'f' -> out.append('\f');
                    case 'n' -> out.append('\n');
                    case 'r' -> out.append('\r');
                    case 't' -> out.append('\t');
                    case 'u' -> {
                        out.append((char) Integer.parseInt(json.substring(i + 1, i + 5), 16));
                        i += 4;
                    }
                    default -> out.append(e);
                }
            }
            throw new IllegalArgumentException("unterminated JSON string");
        }

        static List<String> inboxEvents(String feedJson) {
            return objects(feedJson, "events");
        }

        /** Top-level objects of the array value of {@code name}; brackets inside strings are ignored. */
        static List<String> objects(String json, String name) {
            int i = valueStart(json, name);
            if (i < 0 || json.charAt(i) != '[') {
                return List.of();
            }
            List<String> out = new ArrayList<>();
            int depth = 0;
            int start = -1;
            for (int j = i + 1; j < json.length(); j++) {
                char c = json.charAt(j);
                if (c == '"') {
                    j = stringEnd(json, j);
                } else if (c == '{') {
                    if (depth++ == 0) {
                        start = j;
                    }
                } else if (c == '}') {
                    if (--depth == 0) {
                        out.add(json.substring(start, j + 1));
                    }
                } else if (c == ']' && depth == 0) {
                    break;
                }
            }
            return out;
        }

        private static int stringEnd(String json, int quote) {
            for (int i = quote + 1; i < json.length(); i++) {
                char c = json.charAt(i);
                if (c == '\\') {
                    i++;
                } else if (c == '"') {
                    return i;
                }
            }
            throw new IllegalArgumentException("unterminated JSON string");
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
