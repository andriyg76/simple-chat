# Cross-language E2E

| Test | Command |
|------|---------|
| Go client → Go test-server | `go test ./...` (always) |
| Go client → Java test-server | `go test ./...` (needs `java`, `mvn`; skip with `SKIP_JAVA_E2E=1`) |

Java side (`java/e2e`):

| Test | Command |
|------|---------|
| Java client → Java test-server | `mvn -pl e2e -am test` |
| Java client → Go test-server | same (needs `go` on PATH; skip with `SKIP_GO_E2E=1`) |

Shared scenario: create + rename conversation, members add/remove, plain message + delivery.
