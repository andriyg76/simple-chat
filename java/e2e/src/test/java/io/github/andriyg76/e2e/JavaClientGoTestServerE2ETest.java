package io.github.andriyg76.e2e;

import io.github.andriyg76.client.ChatCliScenario;
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
import org.junit.jupiter.api.io.TempDir;

class JavaClientGoTestServerE2ETest {

    private static final Pattern LISTEN = Pattern.compile("simple-chat go test-server listening on (https?://\\S+)");

    @Test
    @EnabledIf("goAvailable")
    void javaClientAndCliAgainstGoTestServer(@TempDir Path config) throws Exception {
        String repoRoot = System.getenv("SIMPLE_CHAT_REPO");
        Path repo = repoRoot != null
                ? Path.of(repoRoot)
                : Path.of("").toAbsolutePath().resolve("../..").normalize();
        Path goDir = repo.resolve("go");
        Path binary = goDir.resolve("test-server").resolve("go-test-server-e2e.exe");
        buildGoTestServer(goDir, binary);
        ProcessBuilder server = new ProcessBuilder(binary.toString())
                .directory(goDir.resolve("test-server").toFile())
                .redirectErrorStream(true);
        server.environment().put("CHAT_ECHO_BOT", "1");
        Process process = server.start();
        try {
            String baseUrl = waitForListen(process);
            ChatV1Scenario.run(baseUrl, "dev");
            ChatCliScenario.run(baseUrl, "dev", config.resolve("java-client.properties"));
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

    /** Keeps draining server output after the listen line so later log writes never hit a closed pipe. */
    private static String waitForListen(Process process) throws IOException, InterruptedException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()));
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(30);
        String line;
        while (System.nanoTime() < deadline && (line = reader.readLine()) != null) {
            Matcher m = LISTEN.matcher(line);
            if (m.find()) {
                Thread.ofVirtual().name("go-test-server-output").start(() -> reader.lines().forEach(l -> { }));
                return m.group(1).replaceAll("/$", "");
            }
        }
        reader.close();
        throw new IOException("timed out waiting for go test-server listen line");
    }
}
