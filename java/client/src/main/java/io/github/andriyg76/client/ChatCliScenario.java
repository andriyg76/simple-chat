package io.github.andriyg76.client;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.io.PipedInputStream;
import java.io.PipedOutputStream;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.time.Duration;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.FutureTask;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

/**
 * Cross-language E2E for the terminal client. Drives {@link SimpleChatClient} through its stdin/stdout against a
 * test-server whose echo bot replies {@code echo: <text>} as sender {@code server}.
 */
public final class ChatCliScenario {

    private static final Duration STEP = Duration.ofSeconds(15);

    private ChatCliScenario() {
    }

    public static void run(String baseUrl, String enrollCode, Path configFile) throws Exception {
        String text = "hello [cli] {e2e}";
        try (Session cli = new Session(configFile, "--profile", "e2e", "--server", baseUrl, "--enroll", enrollCode)) {
            cli.await("Enter for a new room:");
            cli.send("");
            cli.await("Active room:");
            cli.send(text);
            cli.await("echo: " + text);
            cli.send("/title CLI Room");
            cli.await("Renamed to CLI Room");
            cli.send("/members");
            cli.await("Members: ");
            cli.send("/exit");
            cli.expectExit(0);
        }
        try (Session cli = new Session(configFile)) {
            cli.await("Profile e2e");
            cli.await("CLI Room");
            cli.await("Enter for a new room:");
            cli.closeInput();
            cli.expectExit(0);
        }
    }

    /** One in-process CLI run with piped stdin and captured stdout/stderr. */
    private static final class Session implements AutoCloseable {
        private final ByteArrayOutputStream out = new ByteArrayOutputStream();
        private final ByteArrayOutputStream err = new ByteArrayOutputStream();
        private final OutputStream stdin;
        private final FutureTask<Integer> exit;
        private int consumed;

        Session(Path configFile, String... args) throws IOException {
            PipedOutputStream input = new PipedOutputStream();
            InputStream cliIn = new PipedInputStream(input);
            stdin = input;
            PrintStream cliOut = new PrintStream(out, true, StandardCharsets.UTF_8);
            PrintStream cliErr = new PrintStream(err, true, StandardCharsets.UTF_8);
            exit = new FutureTask<>(() -> SimpleChatClient.run(args, cliIn, cliOut, cliErr, configFile));
            Thread.ofPlatform().name("cli-e2e").start(exit);
        }

        void send(String line) throws IOException {
            stdin.write((line + "\n").getBytes(StandardCharsets.UTF_8));
            stdin.flush();
        }

        void closeInput() throws IOException {
            stdin.close();
        }

        /** Waits for {@code needle} in output written after the previous match. */
        void await(String needle) throws IOException, InterruptedException {
            long deadline = System.nanoTime() + STEP.toNanos();
            while (System.nanoTime() < deadline) {
                String text = out.toString(StandardCharsets.UTF_8);
                int at = text.indexOf(needle, consumed);
                if (at >= 0) {
                    consumed = at + needle.length();
                    return;
                }
                if (exit.isDone()) {
                    break;
                }
                Thread.sleep(50);
            }
            throw new IOException("CLI did not print \"" + needle + "\"" + transcript());
        }

        void expectExit(int code) throws IOException, InterruptedException {
            try {
                int actual = exit.get(STEP.toSeconds(), TimeUnit.SECONDS);
                if (actual != code) {
                    throw new IOException("CLI exited with " + actual + transcript());
                }
            } catch (ExecutionException | TimeoutException e) {
                throw new IOException("CLI did not exit cleanly: " + e + transcript(), e);
            }
        }

        private String transcript() {
            return "\n--- stdout ---\n" + out.toString(StandardCharsets.UTF_8)
                    + "\n--- stderr ---\n" + err.toString(StandardCharsets.UTF_8);
        }

        @Override
        public void close() throws IOException {
            stdin.close();
            exit.cancel(true);
        }
    }
}
