package io.github.andriyg76.e2e;

import io.github.andriyg76.client.ChatV1Scenario;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.server.LocalServerPort;

@SpringBootTest(classes = io.github.andriyg76.testserver.TestServerApplication.class, webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class JavaClientJavaTestServerE2ETest {

    @LocalServerPort
    private int port;

    @Value("${simplechat.token}")
    private String token;

    @Test
    void javaClientAgainstJavaTestServer() throws Exception {
        ChatV1Scenario.run("http://127.0.0.1:" + port, token);
    }
}
