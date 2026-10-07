# Duplicate sources (not import, not vendor)

**simple-chat** owns the canonical Go chat implementation under [`go/pkg/`](go/pkg/). Other products (games-buddy, Askoldova, …) keep **copies** of the same files when they need the protocol in-tree — not a Go module dependency and not `vendor/`.

## Rules

- Edit protocol in [`protocol.md`](protocol.md) first, then implement in `go/pkg/`.
- No `require github.com/.../games-buddy` and no `vendor/` tree in this repo.
- Module path for library code: `github.com/andriyg76/simple-chat/go/pkg/...`.

## Refreshing a duplicate in another repo

1. Diff the package trees (`chatcore`, `chatclient`, `chathttp`, `chatterm`, optional `session`).
2. Copy changed `.go` files into the product repo.
3. Rewrite import paths to that product’s module (e.g. games-buddy `internal/chatclient`, `internal/chatterm` with `AppName` set to the product config dir).

## Refreshing from a product back into simple-chat

Same steps in reverse: copy into `go/pkg/`, imports must stay `github.com/andriyg76/simple-chat/go/pkg/...`, run `go test ./pkg/...`.
