package io.github.andriyg76.testserver;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.event.EventListener;
import org.springframework.core.env.Environment;

@SpringBootApplication
public class TestServerApplication {

    public static void main(String[] args) {
        SpringApplication.run(TestServerApplication.class, args);
    }

    @EventListener(ApplicationReadyEvent.class)
    void logPort(ApplicationReadyEvent event) {
        Environment env = event.getApplicationContext().getEnvironment();
        String port = env.getProperty("local.server.port");
        System.out.printf("simple-chat test-server listening on http://localhost:%s (CHAT_TOKEN=%s)%n",
                port,
                env.getProperty("simplechat.token", "dev"));
        System.out.flush();
    }
}
