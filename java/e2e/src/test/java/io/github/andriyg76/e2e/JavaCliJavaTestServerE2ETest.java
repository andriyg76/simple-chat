package io.github.andriyg76.e2e;

import io.github.andriyg76.client.ChatCliScenario;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.server.LocalServerPort;

@SpringBootTest(
        classes = io.github.andriyg76.testserver.TestServerApplication.class,
        webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT,
        properties = "simplechat.echo-bot=true")
class JavaCliJavaTestServerE2ETest {

    @LocalServerPort
    private int port;

    @Value("${simplechat.enroll-code}")
    private String enrollCode;

    @Test
    void javaCliAgainstJavaTestServer(@TempDir Path config) throws Exception {
        ChatCliScenario.run("http://127.0.0.1:" + port, enrollCode, config.resolve("java-client.properties"));
    }
}
