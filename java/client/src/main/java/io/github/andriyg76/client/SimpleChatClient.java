package io.github.andriyg76.client;

import io.github.andriyg76.protocol.ConversationSession;
import io.github.andriyg76.protocol.EnrollmentExchangeResponse;
import io.github.andriyg76.protocol.EventKind;
import io.github.andriyg76.protocol.InboxEvent;
import io.github.andriyg76.protocol.MessageSender;
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.io.Reader;
import java.io.Writer;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.List;
import java.util.Map;
import java.util.Properties;
import java.util.concurrent.ConcurrentHashMap;

/**
 * JDK-only terminal client for /chat/v1: enroll once, pick or create a room, send plain messages and
 * print replies from the profile inbox (SSE with long-poll fallback).
 */
public final class SimpleChatClient {

    private static final String USAGE = """
            Usage: simple-chat-java [--profile NAME] [--server URL (--enroll CODE | --token TOKEN)]

              --server URL --enroll CODE   exchange a one-time enrollment code and save the profile
              --server URL --token TOKEN   save an existing bearer token as the profile
              --profile NAME               profile to use or save (default: last used)

            Profiles are stored in %s
            """;
    private static final String COMMANDS =
            "Commands: /list, /use N, /new [TITLE], /title NAME, /members, /add ID, /remove ID, /exit";

    private final ChatHttpClient client;
    private final PrintStream out;
    private final PrintStream err;
    private final Map<String, String> titles = new ConcurrentHashMap<>();
    private List<ConversationSession> rooms = List.of();
    private ConversationSession active;

    private SimpleChatClient(ChatHttpClient client, PrintStream out, PrintStream err) {
        this.client = client;
        this.out = out;
        this.err = err;
    }

    public static void main(String[] args) {
        try {
            System.exit(run(args, System.in, new PrintStream(System.out, true, StandardCharsets.UTF_8),
                    new PrintStream(System.err, true, StandardCharsets.UTF_8), configPath()));
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            System.exit(130);
        }
    }

    /** Runs one CLI session and returns the process exit code. */
    public static int run(String[] args, InputStream stdin, PrintStream out, PrintStream err, Path configPath)
            throws InterruptedException {
        String profile = null;
        String server = null;
        String enroll = null;
        String token = null;
        for (int i = 0; i < args.length; i++) {
            String arg = args[i];
            if (arg.equals("-h") || arg.equals("--help")) {
                out.printf(USAGE, configPath);
                return 0;
            }
            if (i + 1 >= args.length) {
                err.printf("missing value for %s%n" + USAGE, arg, configPath);
                return 2;
            }
            String value = args[++i];
            switch (arg) {
                case "--profile" -> profile = value;
                case "--server" -> server = value;
                case "--enroll" -> enroll = value;
                case "--token" -> token = value;
                default -> {
                    err.printf("unknown option %s%n" + USAGE, arg, configPath);
                    return 2;
                }
            }
        }
        if ((server == null) != (enroll == null && token == null) || (enroll != null && token != null)) {
            err.printf("--server needs exactly one of --enroll or --token%n" + USAGE, configPath);
            return 2;
        }
        try {
            Properties config = loadConfig(configPath);
            String name = profile != null ? profile : config.getProperty("default", "default");
            if (server != null) {
                URI uri = URI.create(server);
                if (!"http".equals(uri.getScheme()) && !"https".equals(uri.getScheme()) || uri.getHost() == null) {
                    err.println("--server must be an http(s) URL, e.g. https://chat.example.com");
                    return 2;
                }
                String displayName;
                if (enroll != null) {
                    EnrollmentExchangeResponse enrollment = ChatHttpClient.exchange(server, enroll);
                    token = enrollment.token();
                    displayName = enrollment.profile().displayName();
                } else {
                    displayName = new ChatHttpClient(server, token).currentProfile().displayName();
                }
                config.setProperty(name + ".server", server);
                config.setProperty(name + ".token", token);
                config.setProperty(name + ".displayName", displayName);
            }
            String baseUrl = config.getProperty(name + ".server");
            String savedToken = config.getProperty(name + ".token");
            if (baseUrl == null || savedToken == null) {
                err.printf("no credentials for profile %s: run with --server URL --enroll CODE%n", name);
                return 2;
            }
            config.setProperty("default", name);
            saveConfig(configPath, config);
            out.printf("Profile %s (%s) · %s%n", name, config.getProperty(name + ".displayName", name), baseUrl);
            return new SimpleChatClient(new ChatHttpClient(baseUrl, savedToken), out, err).session(stdin);
        } catch (ChatHttpClient.HttpStatusException e) {
            err.println(e.status() == 401 || e.status() == 403
                    ? "server rejected the credentials; enroll again with --server URL --enroll CODE"
                    : e.getMessage());
            return 1;
        } catch (IOException | IllegalArgumentException e) {
            err.println("error: " + e.getMessage());
            return 1;
        }
    }

    private int session(InputStream stdin) throws IOException, InterruptedException {
        BufferedReader in = new BufferedReader(new InputStreamReader(stdin, StandardCharsets.UTF_8));
        refreshRooms();
        printRooms();
        out.print("Open number or Enter for a new room: ");
        out.flush();
        String choice = in.readLine();
        if (choice == null) {
            return 0;
        }
        ConversationSession picked = pick(choice.trim());
        active = picked != null ? picked : create("");

        long cursor = client.inboxCursor();
        Thread inbox = Thread.ofVirtual().name("inbox").start(() -> listen(cursor));
        out.printf("Active room: %s. %s%n", label(active), COMMANDS);
        try {
            for (String line = in.readLine(); line != null; line = in.readLine()) {
                String text = line.strip();
                if (text.isEmpty()) {
                    continue;
                }
                try {
                    if (text.startsWith("/")) {
                        if (!command(text)) {
                            break;
                        }
                    } else {
                        client.postPlain(active.code(), line);
                    }
                } catch (IOException e) {
                    err.println("error: " + e.getMessage());
                }
            }
        } finally {
            inbox.interrupt();
        }
        return 0;
    }

    /** Returns false on /exit. */
    private boolean command(String text) throws IOException, InterruptedException {
        int space = text.indexOf(' ');
        String name = space < 0 ? text : text.substring(0, space);
        String arg = space < 0 ? "" : text.substring(space + 1).strip();
        switch (name) {
            case "/exit", "/quit" -> {
                return false;
            }
            case "/list" -> {
                refreshRooms();
                printRooms();
            }
            case "/use" -> {
                ConversationSession picked = pick(arg);
                if (picked == null) {
                    err.println("unknown room; use /list and /use N");
                } else {
                    active = picked;
                    out.println("Active room: " + label(active));
                }
            }
            case "/new" -> {
                active = create(arg);
                out.println("Active room: " + label(active));
            }
            case "/title" -> {
                if (arg.isEmpty()) {
                    err.println("usage: /title NAME");
                } else {
                    active = client.updateTitle(active.code(), arg);
                    titles.put(active.code(), active.title());
                    out.println("Renamed to " + label(active));
                }
            }
            case "/members" -> out.println("Members: " + String.join(", ", client.listMembers(active.code())));
            case "/add" -> {
                client.addMember(active.code(), arg);
                out.println("Added " + arg);
            }
            case "/remove" -> {
                client.removeMember(active.code(), arg);
                out.println("Removed " + arg);
            }
            default -> err.println("unknown command; " + COMMANDS);
        }
        return true;
    }

    private void listen(long cursor) {
        try {
            client.subscribeInbox(cursor, this::print);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        } catch (IOException e) {
            err.println("inbox delivery stopped: " + e.getMessage());
        }
    }

    private synchronized void print(InboxEvent event) {
        String room = titles.getOrDefault(event.sessionId(), event.sessionId());
        switch (event.kind()) {
            case EventKind.MESSAGE, EventKind.REPLY -> {
                if (!MessageSender.CLIENT.equals(event.sender())) {
                    out.printf("[%s] %s%n", room, event.text());
                }
            }
            case EventKind.ERROR -> err.printf("[%s] error: %s%n", room, event.text());
            default -> {
            }
        }
    }

    private void refreshRooms() throws IOException, InterruptedException {
        rooms = client.listConversations();
        rooms.forEach(room -> titles.put(room.code(), room.title()));
    }

    private void printRooms() {
        if (rooms.isEmpty()) {
            out.println("No conversations yet.");
            return;
        }
        out.println("Conversations:");
        for (int i = 0; i < rooms.size(); i++) {
            out.printf("  %d. %s%n", i + 1, label(rooms.get(i)));
        }
    }

    private ConversationSession pick(String number) {
        try {
            int index = Integer.parseInt(number) - 1;
            return index >= 0 && index < rooms.size() ? rooms.get(index) : null;
        } catch (NumberFormatException e) {
            return null;
        }
    }

    private ConversationSession create(String title) throws IOException, InterruptedException {
        ConversationSession room = client.createWithTitle(title);
        titles.put(room.code(), room.title());
        refreshRooms();
        return room;
    }

    private static String label(ConversationSession room) {
        return room.title() == null || room.title().isBlank() ? room.code() : room.title();
    }

    private static Path configPath() {
        String xdg = System.getenv("XDG_CONFIG_HOME");
        Path base = xdg != null && !xdg.isEmpty() ? Path.of(xdg) : Path.of(System.getProperty("user.home"), ".config");
        return base.resolve("simple-chat").resolve("java-client.properties");
    }

    private static Properties loadConfig(Path path) throws IOException {
        Properties config = new Properties();
        if (Files.exists(path)) {
            try (Reader reader = Files.newBufferedReader(path, StandardCharsets.UTF_8)) {
                config.load(reader);
            }
        }
        return config;
    }

    /** The file holds bearer tokens, so it is created readable by the owner only where POSIX permissions exist. */
    private static void saveConfig(Path path, Properties config) throws IOException {
        Files.createDirectories(path.getParent());
        if (!Files.exists(path)) {
            Files.createFile(path);
            try {
                Files.setPosixFilePermissions(path, PosixFilePermissions.fromString("rw-------"));
            } catch (UnsupportedOperationException e) {
                // Non-POSIX file systems (Windows) rely on the user profile directory ACLs.
            }
        }
        try (Writer writer = Files.newBufferedWriter(path, StandardCharsets.UTF_8)) {
            config.store(writer, "simple-chat Java client profiles");
        }
    }
}
