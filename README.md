# tcpwsbridge

A **Go** tool that tunnels **TCP** over **HTTP Upgrade → WebSocket**: a **client** accepts local TCP connections and opens one WebSocket per connection to a **bridge server**, which dials a **downstream TCP** target and relays bytes both ways.

For architecture and security notes, see [DESIGN.md](DESIGN.md).

## Requirements

- [Go](https://go.dev/dl/) **1.21** or newer

## Build

```bash
go build -o bin/server ./cmd/server
go build -o bin/client ./cmd/client
```

## Quick start

**Terminal 1 — bridge server**

```bash
./bin/server -listen :9000 -path /ws
```

`GET /` (and non-upgrade `GET` on the WebSocket path) returns a small HTML landing page.

**Terminal 2 — client**

The client listens for TCP and, for each connection, dials the bridge with **`h`** (host) and **`p`** (port) query parameters so the server knows what to dial.

```bash
./bin/client -listen :8080 -bridge ws://127.0.0.1:9000/ws -host 127.0.0.1 -port 9999
```

Flow: `upstream TCP → client :8080 → WebSocket → server → TCP 127.0.0.1:9999` (and reverse).

## Server (`cmd/server`)

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `:9000` | HTTP listen address |
| `-path` | `/ws` | WebSocket bridge path |
| `-token` | *(empty)* | If set, require `Authorization: Bearer <token>` on upgrade |
| `-allow-all-origins` | `false` | Allow any `Origin` (development only; unsafe with browser clients) |
| `-dial-timeout` | `15s` | Timeout when dialing downstream TCP |
| `-tls-cert` / `-tls-key` | *(empty)* | If both set, serve HTTPS/WSS |

## Client (`cmd/client`)

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `:8080` | Local TCP listen address |
| `-bridge` | `ws://127.0.0.1:9000/ws` | Bridge WebSocket URL; **`h`** and **`p`** query params are added for the server |
| `-host` | `127.0.0.1` | Downstream host the **server** will dial |
| `-port` | `0` | Downstream port (**required**, 1–65535) |
| `-token` | *(empty)* | If set, send `Authorization: Bearer <token>` |
| `-max-conns` | `0` | Max concurrent upstream TCP connections (`0` = unlimited) |
| `-dial-timeout` | `15s` | WebSocket dial timeout to the bridge |

## Bridge URL (query string)

The server reads **`h`** (hostname or IP) and **`p`** (port, 1–65535) from the WebSocket request query string, for example:

```text
wss://bridge.example/ws?h=10.0.0.5&p=3306
```

The client sets these from `-host` and `-port`.


