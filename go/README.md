# Go reference stack

Vendored from [games-buddy](https://github.com/andriyg76/games-buddy) `internal/chat*` (same `/chat/v1` wire format; see [protocol.md](../protocol.md)).

| Path | Role |
|------|------|
| [pkg/chatcore](pkg/chatcore) | Cursor journal + delivery |
| [pkg/chatclient](pkg/chatclient) | HTTP + SSE / long-poll client |
| [pkg/chat](pkg/chat) | Profiles, sessions, Mongo conversations |
| [pkg/chathttp](pkg/chathttp) | HTTP routes (plain chat; no AI/WhatsApp) |
| [pkg/msgstore](pkg/msgstore) | Message persistence |
| [server](server) | `main` — Mongo + `/chat/v1` |
| [client](client) | Terminal client using `chatclient` |

Sync changes from games-buddy by copying updated files into `pkg/` and fixing imports to `github.com/agushuley/simple-chat/go/pkg/...`.

## Build

```bash
cd go
go build -o bin/simple-chat-server ./server
go build -o bin/simple-chat-client ./client
```

## Run

Server (MongoDB required):

```bash
set MONGO_URI=mongodb://127.0.0.1:27017
set ADMIN_TOKEN=dev-admin-token
bin\simple-chat-server.exe
```

Bootstrap once:

```bash
curl -s -X POST http://localhost:8080/admin/chat/bootstrap ^
  -H "Authorization: Bearer dev-admin-token" ^
  -H "Content-Type: application/json" ^
  -d "{\"displayName\":\"Admin\"}"
```

Client:

```bash
bin\simple-chat-client.exe --server http://localhost:8080 --enroll <code>
```
