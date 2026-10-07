# Go reference stack

Core only: **delivery journal**, **SSE**, **long poll**, **HTTP** — no database or message persistence (products add that separately).

| Package | Role |
|---------|------|
| [pkg/chatcore](pkg/chatcore) | Cursor journal + `Store` access contract |
| [pkg/chatclient](pkg/chatclient) | Embeddable HTTP + SSE / long-poll client |
| [pkg/chatterm](pkg/chatterm) | Embeddable terminal client (used by `go/client`) |
| [pkg/chathttp](pkg/chathttp) | `/chat/v1` transport |
| [pkg/session](pkg/session) | In-memory `chatcore.Store` for demos/tests |
| [test-server](test-server) | Harness (`PORT` via ephemeral listen; like Java test-server) |
| [server](server) | Fixed-port server (`PORT`, default 8080) |
| [e2e](../e2e) | Cross-language E2E tests |
| [pkg/testserver](pkg/testserver) | In-process harness for tests |
| [client](client) | `simple-chat` terminal binary (thin wrapper over `chatterm`) |

See [SYNC.md](../SYNC.md) for copying sources into products.

## Build

```bash
cd go
go build -o bin/simple-chat-server ./server
go build -o bin/simple-chat-client ./client
```

## Run

```bash
set CHAT_TOKEN=dev
set CHAT_ENROLL_CODE=dev
bin\simple-chat-server.exe
```

Client (profiles, Markdown; default send is async — replies on SSE/long poll; use `--plain` for `/messages/plain` only):

```bash
bin\simple-chat-client.exe --profile local --server http://localhost:8080 --enroll dev
```

Or `--token dev` with `--server` after enrolling once. Config: `%AppData%\simple-chat\chat.json` on Windows.
