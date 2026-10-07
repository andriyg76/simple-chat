package io.github.andriyg76.e2e;

import io.github.andriyg76.client.ChatV1Scenario;
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIf;

class JavaClientGoTestServerE2ETest {

    private static final Pattern LISTEN = Pattern.compile("simple-chat go test-server listening on (https?://\\S+)");

    @Test
    @EnabledIf("goAvailable")
    void javaClientAgainstGoTestServer() throws Exception {
        String repoRoot = System.getenv("SIMPLE_CHAT_REPO");
        Path repo = repoRoot != null
                ? Path.of(repoRoot)
                : Path.of("").toAbsolutePath().resolve("../..").normalize();
        Path goDir = repo.resolve("go");
        Path binary = goDir.resolve("test-server").resolve("go-test-server-e2e.exe");
        buildGoTestServer(goDir, binary);
        Process process = new ProcessBuilder(binary.toString())
                .directory(goDir.resolve("test-server").toFile())
                .redirectErrorStream(true)
                .start();
        try {
            String baseUrl = waitForListen(process);
            ChatV1Scenario.run(baseUrl, "dev");
        } finally {
            process.destroyForcibly();
            process.waitFor(5, TimeUnit.SECONDS);
        }
    }

    static boolean goAvailable() {
        if ("1".equals(System.getenv("SKIP_GO_E2E"))) {
            return false;
        }
        try {
            Process p = new ProcessBuilder("go", "version").start();
            return p.waitFor(10, TimeUnit.SECONDS) && p.exitValue() == 0;
        } catch (IOException | InterruptedException e) {
            return false;
        }
    }

    private static void buildGoTestServer(Path goDir, Path binary) throws IOException, InterruptedException {
        Process build = new ProcessBuilder(
                "go", "build", "-o", binary.toString(), ".")
                .directory(goDir.resolve("test-server").toFile())
                .inheritIO()
                .start();
        if (!build.waitFor(2, TimeUnit.MINUTES) || build.exitValue() != 0) {
            throw new IOException("go build failed");
        }
        if (!Files.isRegularFile(binary)) {
            throw new IOException("missing " + binary);
        }
    }

    private static String waitForListen(Process process) throws IOException, InterruptedException {
        try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()))) {
            long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(30);
            String line;
            while (System.nanoTime() < deadline && (line = reader.readLine()) != null) {
                Matcher m = LISTEN.matcher(line);
                if (m.find()) {
                    return m.group(1).replaceAll("/$", "");
                }
            }
        }
        throw new IOException("timed out waiting for go test-server listen line");
    }
}
