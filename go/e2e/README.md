# Cross-language E2E

| Test | Command |
|------|---------|
| Go client → Go test-server | `go test ./...` (always) |
| Go client → Java test-server | `go test ./...` (needs `java`, `mvn`; skip with `SKIP_JAVA_E2E=1`) |

Java side (`java/e2e`):

| Test | Command |
|------|---------|
| Java client → Java test-server | `mvn -pl e2e -am test` |
| Java terminal client → Java test-server | same (test-server runs with `simplechat.echo-bot=true`) |
| Java client + terminal client → Go test-server | same (needs `go` on PATH; skip with `SKIP_GO_E2E=1`; server runs with `CHAT_ECHO_BOT=1`) |

Shared scenario (`ChatV1Scenario`): create + rename conversation, members add/remove, plain message + delivery.

Terminal client scenario (`ChatCliScenario`): drives `SimpleChatClient` over stdin/stdout — enroll and save a profile,
new room, message answered by the echo bot (`sender: server`) over the inbox, `/title`, `/members`, `/exit`,
then a second run that reuses the saved profile.
