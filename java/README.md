# Java (Askoldova-ready)

**JDK only** in compile scope — no Jackson, Spring, or Mongo in `protocol`, `core`, or `server`.

| Module | Artifact | Role |
|--------|----------|------|
| [protocol](protocol) | `simple-chat-java-protocol` | Wire POJOs + `ChatEvent.toJson()` |
| [core](core) | `simple-chat-java-core` | `ChatCore`, `ConversationStore` |
| [server](server) | `simple-chat-java-server` | `ChatV1Service`, `ChatSseSupport` (library) |
| [test-server](test-server) | Spring Boot (local only) | Full `/chat/v1` + SSE + long poll; **excluded from `mvn deploy`** |
| [client](client) | stub CLI | |

## Embed in Askoldova

1. Install jars: `mvn -f protocol install && mvn -f core install && mvn -f server install` (or publish to your Maven repo).
2. Implement `ConversationStore` (JPA / existing DB).
3. Implement `ChatAuthResolver` (session JWT / `LoginService`).
4. Register a Spring `@RestController` that delegates to `ChatV1Service` and maps `ChatException` to `ChatErrorEnvelope` JSON (use your existing `ObjectMapper` for responses).

```java
var service = new ChatV1Service(askoldovaStore, askoldovaAuth, enrollmentOptional);
```

SSE: call `ChatSseSupport.stream(...)` from a servlet/async response, or map events yourself from `ChatCore.next`.

## Run test server (SSE + long poll)

```bash
cd java
mvn -pl protocol,core,server install -DskipTests
mvn -pl test-server spring-boot:run
```

Binds to a **random free port** (`server.port=0`); the chosen URL is printed on startup. Default token/enroll code: `dev`.

## Build libraries

```bash
cd java && mvn -pl protocol,core,server test install
```

## Maven Central

See [MAVEN_CENTRAL.md](MAVEN_CENTRAL.md). Release version **0.1.0** — `com.simplechat:simple-chat-java-{protocol,core,server}`.
