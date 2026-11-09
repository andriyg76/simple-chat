# simple-chat

Shared text-chat **protocol** and reference **client/server** implementations in Go and Java.

| Path | Role |
|------|------|
| [protocol.md](protocol.md) | Wire format, transports, event types (source of truth) |
| [go/pkg](go/pkg) | **Canonical Go chat library** (duplicate-friendly sources) |
| [go/server](go/server) | Go reference server |
| [go/client](go/client) | Default terminal client (`simple-chat`) |
| [go/pkg/chatterm](go/pkg/chatterm) | Embeddable terminal UI (profiles, readline, SSE) |
| [java/protocol](java/protocol) | Wire POJOs (JDK only) |
| [java/core](java/core) | `ChatCore` + `ConversationStore` |
| [java/server](java/server) | Embeddable `ChatV1Service` |
| [java/client](java/client) | Java reference client stub |
| [SYNC.md](SYNC.md) | How to copy sources between products (no import / no vendor) |
| [RELEASE.md](RELEASE.md) | `main` vs `release/X.Y` bugfix branches and Maven tags |
| [java/MAVEN_CENTRAL.md](java/MAVEN_CENTRAL.md) | Publish `protocol` / `core` / `server` to Maven Central |

Products implement the same `/chat/v1` contract; they may **duplicate** files from `go/pkg/` into their own tree rather than depending on this module.

```text
simple-chat/
  protocol.md
  go/pkg/              # chatcore, chatclient, chat, chathttp, …
  go/server|client/
  java/server|client/
```

Transport: HTTP JSON + **SSE** + **long poll** (not WebSocket for text chat). See [go/README.md](go/README.md).

### Mobile push (iOS, macOS, Android)

After login, register the install with `PUT /chat/v1/devices/{deviceId}` (`platform` + `pushChannel` + OS `pushToken`). Use **APNs** (`apns`) for `ios`/`macos` and **FCM** (`fcm`) for `android`. Refresh the token on OS callbacks; send `POST …/heartbeat` periodically and `DELETE` on logout. On new messages, the host pushes to other members’ devices unless they have an open **inbox SSE** on that host. Wire APNs/FCM in the product layer; reference servers use a logging notifier only. See [protocol.md](protocol.md#mobile-devices-and-push).
