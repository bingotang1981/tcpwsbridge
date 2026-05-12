# TCP–WebSocket Bridge — Design

## 1. Overview

This tool builds a transparent byte-stream pipe between **upstream TCP** and **downstream TCP**, with the middle hop carried on a long-lived **HTTP Upgrade to WebSocket** connection. The system has two parts — **client** and **server (bridge)** — implemented in **Go** (`cmd/client`, `cmd/server`).

---

## 2. Components

| Component | Role |
|-----------|------|
| **Client** | After startup, **listens on TCP**; for each accepted connection, performs a **WebSocket handshake** with the bridge server; **bidirectionally relays** bytes between **upstream TCP** and the **bridge WebSocket**. |
| **Server** | Serves **HTTP**; for **non-WebSocket** HTTP requests, returns a **default info page**; on the WebSocket path, completes **Upgrade**; parses downstream **host and port** from **URL query parameters**; **dials** downstream TCP; **bidirectionally relays** between the **client WebSocket** and **downstream TCP**. |

---

## 3. Connection Model

### 3.1 One TCP maps to one WebSocket (1:1)

- Each TCP connection from `Accept` maps to **one** WebSocket session.
- When TCP or WebSocket closes, the **other side should be closed** and resources released to avoid orphaned connections.

### 3.2 Downstream target from URL parameters

- When opening the WebSocket, the client puts the **downstream host and port** the bridge must dial into the **HTTP request URL query string**.
- **Semantics**: the parameters denote the **TCP target the bridge server will dial**, not the client’s local listen address.
- The server parses parameters during the handshake; invalid parameters yield **HTTP 4xx** with no Upgrade.

### 3.3 URL parameter convention (recommended)

Use a query string with fixed field names in the implementation, for example:

- `h`: hostname or IP (encode IPv6 per URL rules).
- `p`: port number as a string, `1`–`65535`.

Example:

```text
wss://bridge.example/ws?h=10.0.0.5&p=3306
```

Alternative: a single `target=10.0.0.5:3306` parameter (requires consistent IPv6 syntax and parsing rules).

---

## 4. Protocol Stack

| Link | Protocol |
|------|----------|
| Upstream ↔ client | TCP byte stream |
| Client ↔ bridge server | HTTP **Upgrade** → **WebSocket** (**BinaryMessage** carries TCP bytes) |
| Bridge server ↔ downstream | TCP byte stream |

---

## 5. HTTP default page (non-WebSocket)

- **`GET /`** (and other paths not matched by a more specific route, per `ServeMux` behavior) returns a **fixed HTML info page** (`200`, `text/html`) for health checks and human browsing.
- For the **WebSocket path** (e.g. `/ws`): if the request **does not qualify for a WebSocket upgrade** (not an Upgrade request), return the **same default page** so ordinary browser visits to `/ws` do not get an unreadable raw error body.
- **WebSocket handshake** runs only with standard upgrade headers and after auth / parameter checks pass.

---

## 6. End-to-end data flow

1. Client starts and **TCP Listen**s on the configured address.
2. Upstream establishes **TCP** to the client.
3. For that TCP, the client **builds the WebSocket URL** (including downstream `h`, `p` query parameters) and completes **HTTP → WebSocket** handshake.
4. Bridge server parses parameters → **Dial** resolved host:port → pipes that WebSocket to downstream TCP.
5. **Upstream → client TCP → WebSocket → bridge → downstream TCP**.
6. **Downstream TCP → bridge → WebSocket → client TCP → upstream**.
7. When either side ends → close related connections and clean up.

---

## 7. Go implementation notes

### 7.1 Layout (suggested)

```text
cmd/client/main.go
cmd/server/main.go
internal/relay/      # TCP ↔ WebSocket bidirectional copy
internal/server/     # default page, Upgrade, Dial, piping
internal/target/     # h/p query parsing
```

### 7.2 Dependencies

- Standard library: `net`, `net/http`, `context`, `crypto/tls`, etc.
- WebSocket: third-party [`github.com/gorilla/websocket`](https://github.com/gorilla/websocket) (`websocket.Dialer`, `websocket.Upgrader`, `IsWebSocketUpgrade`, `ReadMessage` / `WriteMessage`); **do not** hand-roll RFC 6455 framing.
- Root `go.mod` uses **`replace github.com/gorilla/websocket => ./third_party/gorilla/websocket`**, vendoring sources matching **v1.5.3** so the project still builds when `proxy.golang.org` is unreachable. If the public proxy is available, the `replace` line may be removed and the same version fetched into the module cache.

### 7.3 Client

- `net.Listen` + per-connection goroutine: `websocket.Dialer.DialContext` to the bridge URL.
- Bidirectional: `TCP Read` → `WriteMessage(BinaryMessage)`; `ReadMessage` → `TCP Write`.
- Use `context` and handshake timeout; optional `-max-conns` for concurrency limits.

### 7.4 Server

- `http.ServeMux`: **register** the WebSocket path (e.g. `/ws`) **before** **`/`** for the default page, so the root handler does not shadow the bridge path.
- WebSocket path: if `websocket.IsWebSocketUpgrade` is false, serve the **default page**; otherwise validate Token, `Origin` (per `-allow-all-origins`), and query parameters, then **`Upgrader.Upgrade`**.
- `net.DialTimeout("tcp", net.JoinHostPort(...))` to reach downstream.
- **Concurrent writes**: only **one** goroutine per relay calls `WriteMessage`, satisfying gorilla’s writer constraints.

### 7.5 Security and operations

- Use **WSS** on the public internet; authenticate at handshake (e.g. **Token** via `Authorization: Bearer`).
- **Avoid open-proxy behavior**: **whitelist** downstreams that may be relayed (subnets, ports).
- Structured logging: client IP, resolved downstream (redacted), duration, close reason.

---

## 8. Testing recommendations

- Unit tests: URL parsing, port validation, ACL.
- Integration tests: local `Listen` for simulated upstream/downstream; `-race` for data races.

---

## 9. Requirements traceability

| ID | Requirement | Where documented |
|----|-------------|------------------|
| R1 | Program includes client and server | §2 |
| R2 | Client listens on TCP; talks to server via **HTTP Upgrade to WebSocket**; **one TCP per WebSocket**; **URL parameters carry downstream address and port** | §3, §4, §6 |
| R3 | Server connects downstream from parameters, forwards TCP data, returns replies over WebSocket to the client | §2, §6 |
| R4 | Non-WebSocket HTTP requests return the **default page** | §2, §5 |

---

## 10. Open questions

- Whether to add or standardize on a single `target` field instead of `h`+`p`.
- Whether downstream may be hostname-only, and DNS timeout policy.
- Whether the client’s downstream parameters are a single static target in config, or must vary per connection (the latter may need a protocol extension).

---

## 11. Build and run examples

```bash
go build -o bin/server ./cmd/server
go build -o bin/client ./cmd/client
```

Terminal 1 (bridge): `./bin/server -listen :9000 -path /ws`  
Browser: `http://127.0.0.1:9000/` or a plain GET on `/ws` shows the **default info page**.

Terminal 2 (local TCP on `:8080`, bridge to downstream `127.0.0.1:9999`): `./bin/client -listen :8080 -bridge ws://127.0.0.1:9000/ws -host 127.0.0.1 -port 9999`

Optional: `-token <secret>` (same on both ends), server `-allow-all-origins` (browser WebSocket clients), `-tls-cert`/`-tls-key` (HTTPS/WSS).
