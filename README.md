# simple-chat

Shared text-chat **protocol** and reference **client/server** implementations in Go and Java.

| Path | Role |
|------|------|
| [protocol.md](protocol.md) | Wire format, transports, event types (source of truth) |
| [go/server](go/server) | Go reference server |
| [go/client](go/client) | Go reference client |
| [java/server](java/server) | Java reference server |
| [java/client](java/client) | Java reference client |

Products (Askoldova, games-buddy, …) implement the same protocol; language folders are interchangeable test beds.

**Reference implementation:** games-buddy `internal/chathttp` + `chatclient` (SSE + long poll, `/chat/v1`). No WebSocket for text chat.

```text
simple-chat/
  protocol.md          # wire contract (start here)
  go/client|server/    # games-buddy chathttp + chatclient (see go/README.md)
  java/client|server/  # plain JDK — no Spring Boot
```

Clone [games-buddy](https://github.com/andriyg76/games-buddy) next to this repo for Go `replace` paths.
