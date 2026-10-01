# Simple Chat Protocol

**Status:** v1 (aligned with [games-buddy](https://github.com/andriyg76/games-buddy) `/chat/v1`)  
**Transport:** HTTP JSON + **SSE** (`text/event-stream`) + **long-poll** fallback — not WebSocket.

This file is the cross-product contract. Reference behavior: `games-buddy/internal/chathttp`, `chatcore`, `chatclient`. Product-specific features (AI, WhatsApp, admin bootstrap) are **extensions** and may be omitted in minimal servers (e.g. Askoldova).

---

## Authentication

- Header: `Authorization: Bearer <token>`
- Enrollment (optional per product): `POST /chat/v1/enrollments/exchange` with `{"code":"<enrollment>"}` → `token`, `expiresInSeconds`, `profile`.

**Profile**

| Field | Type |
|-------|------|
| `code` | string |
| `displayName` | string |
| `role` | `user` \| `admin` |
| `createdAt`, `updatedAt` | RFC3339 |

---

## Conversations

A **conversation** is identified by `code` (JSON field `code` on list items; events use `sessionId` for the same id).

| Method | Path | Body / query | Response |
|--------|------|--------------|----------|
| `GET` | `/chat/v1/conversations` | `?archived=true` optional | `[Session, …]` |
| `POST` | `/chat/v1/conversations` | (empty or product-specific) | `Session` |

**Session (list item)**

| Field | Type |
|-------|------|
| `code` | string |
| `title` | string |
| `lastMessageAt` | RFC3339 |
| `reviewSummary` | string, optional |

---

## Sending messages

| Method | Path | Body | Notes |
|--------|------|------|--------|
| `POST` | `/chat/v1/conversations/{code}/messages/plain` | `{"message":"<text>"}` | **Core:** persist + deliver event; no AI |
| `POST` | `/chat/v1/conversations/{code}/messages` | `{"message":"<text>"}` | **Extension:** sync AI reply in HTTP response |
| `POST` | `/chat/v1/conversations/{code}/messages/async` | `{"message":"<text>"}` | **Extension:** `{"messageId","cursor"}` then events on feed/SSE |

Empty or whitespace-only `message` → **422** `message_required`.

---

## Delivery event (JSON)

Used in SSE `data`, long-poll `/feed` body, and plain-message responses.

| Field | Type | Notes |
|-------|------|--------|
| `cursor` | int64 | Monotonic per conversation |
| `sessionId` | string | Conversation `code` |
| `requestId` | string | Optional; correlates async / AI turns |
| `messageId` | string | UUID |
| `sender` | string | `client` \| `server` |
| `kind` | string | `message`, `progress`, `error`, `reply` (legacy) |
| `text` | string | Body or aggregated progress |
| `createdAt` | RFC3339 | UTC |

---

## Real-time: SSE

`GET /chat/v1/conversations/{code}/events`

- `Accept: text/event-stream`
- Resume: `Last-Event-ID: <cursor>` or `?cursor=<cursor>`
- Frames:
  - `id: {cursor}\nevent: {kind}\ndata: {JSON Event}\n\n`
  - Idle: `: heartbeat\n\n` about every **15s**
  - Expired cursor before stream: `event: error\ndata: {"error":"cursor_expired"}\n\n`

**SSE note:** `kind: "reply"` may be suppressed when a matching server `message` exists; clients should dedupe (see `chatclient.Subscribe`).

---

## Real-time: long poll

`GET /chat/v1/conversations/{code}/feed?cursor=<n>`

- Waits up to ~**30s** for the next event after `cursor`.
- Response: full **Event** JSON, or `{"cursor":<n>}` on timeout with no new event.

---

## Cursor

`GET /chat/v1/conversations/{code}/cursor` → current delivery cursor.

**Journal (reference server):** in-memory ring of last **100** events per conversation. Cursor older than retained window → **409** `cursor_expired` (`invalid_cursor` for malformed values). Durable history is a separate persistence concern (`msgstore` in games-buddy).

---

## Errors

HTTP JSON envelope:

```json
{"error":{"code":"<machine_code>","message":"<text>"}}
```

Common codes: `unauthorized`, `conversation_not_found`, `message_required`, `invalid_cursor`, `cursor_expired`.

---

## Optional extensions (games-buddy)

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/chat/v1/conversations/{code}/archive` | Archive + review (204) |
| `POST` | `/admin/chat/bootstrap` | First admin + enrollment (`ADMIN_TOKEN`) |

Minimal implementations (Java/Go stubs in this repo) implement **plain messages + feed + events + cursor** first; wire AI and admin when needed.

---

## Implementations in this repo

| Component | Target parity |
|-----------|----------------|
| `go/client` | `games-buddy/internal/chatclient` (SSE → poll fallback) |
| `go/server` | `chatcore` + `chathttp` subset |
| `java/client` | HTTP + SSE consumer |
| `java/server` | JDK `HttpServer` or equivalent |

Breaking wire changes require a new API prefix or documented migration; do not rename JSON fields without updating all clients.
