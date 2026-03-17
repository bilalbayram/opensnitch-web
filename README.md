# OpenSnitch Web UI

A modern, self-hosted web interface for managing [OpenSnitch](https://github.com/evilsocket/opensnitch) firewall nodes. Monitor connections in real time, manage rules, respond to application prompts, and view traffic statistics — all from your browser.

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)
![License](https://img.shields.io/badge/License-MIT-green)

---

## Features

- **Real-time monitoring** — Live connection feed via WebSocket
- **Multi-node management** — Connect and manage multiple OpenSnitch daemon nodes
- **Rules management** — Create, edit, enable/disable, and delete firewall rules
- **Interactive prompts** — Respond to application connection requests from the browser
- **Blocklists** — Import and manage external blocklists
- **Statistics & charts** — Visualize traffic by host, process, port, protocol, and more
- **Firewall control** — Enable/disable interception and firewall per node
- **Alerts** — View and manage connection alerts
- **Single binary** — Frontend is embedded into the Go binary for easy deployment

## Architecture

```
┌─────────────────────────────────────────────┐
│              Browser (React SPA)            │
│  Vite · Tailwind CSS · Zustand · Recharts  │
│  React Query · React Router · Lucide Icons │
└──────────────────┬──────────────────────────┘
                   │ HTTP / WebSocket
┌──────────────────▼──────────────────────────┐
│           Go HTTP Server (Chi)              │
│  REST API · JWT Auth · SPA Serving          │
├─────────────────────────────────────────────┤
│  SQLite (connections, stats, rules, alerts) │
├─────────────────────────────────────────────┤
│           gRPC Server (TCP + Unix)          │
│  Receives events from OpenSnitch daemons    │
└──────────────────┬──────────────────────────┘
                   │ gRPC
        ┌──────────▼──────────┐
        │  OpenSnitch Daemons │
        │  (one or more nodes)│
        └─────────────────────┘
```

The Go backend embeds the compiled React frontend. In development, it prefers serving from `web/dist/` on disk; in production, the embedded copy is used — resulting in a single self-contained binary.

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Go | 1.22+ | `CGO_ENABLED=1` required (SQLite driver) |
| Node.js | 20+ | For building the frontend |
| GCC / musl-dev | — | Required by CGO for SQLite |
| protoc | — | Only needed if modifying `.proto` files |

## Quick Start

### Docker (recommended)

```bash
docker build -t opensnitch-web .
docker run -p 8080:8080 -p 50051:50051 opensnitch-web
```

### Build from Source

```bash
# Build everything (frontend + embed + Go binary)
make all

# Run the binary
./bin/opensnitch-web -config config.yaml
```

### Run Without Building

```bash
# Build frontend, then run the Go server directly
make frontend
make run
```

Open [http://localhost:8080](http://localhost:8080) and log in with the default credentials (see below).

## Configuration

On first run, if `config.yaml` doesn't exist, it is automatically created from `config.yaml.example` with randomly generated secrets (JWT secret and admin password). The generated admin password is printed to the log.

`config.yaml` is gitignored to prevent secrets from being tracked or overwritten by `git pull`.

```yaml
server:
  http_addr: ":8080"           # HTTP listen address
  grpc_addr: "0.0.0.0:50051"  # gRPC listen address (for daemon nodes)
  grpc_unix: "/tmp/osui.sock"  # Unix socket for local daemon connections

database:
  path: "./opensnitch-web.db"  # SQLite database file
  purge_days: 30               # Auto-purge connections older than N days

auth:
  default_user: "admin"                # Default admin username
  default_password: "opensnitch"       # Default admin password (auto-generated on first run)
  session_ttl: "24h"                   # JWT session duration
  jwt_secret: "change-me-in-production" # JWT signing secret (auto-generated on first run)

ui:
  default_action: "deny"       # Default action for unhandled prompts
  prompt_timeout: 120          # Seconds before a prompt times out
```

Pass a custom path with `-config`:

```bash
./bin/opensnitch-web -config /etc/opensnitch-web/config.yaml
```

## Development

### Full Stack (backend + frontend)

```bash
make dev
```

This runs the Go backend and the Vite dev server concurrently. The Vite dev server provides hot module replacement at [http://localhost:5173](http://localhost:5173).

### Frontend Only

```bash
cd web
npm install
npm run dev
```

### Generate Protobuf Code

```bash
make proto
```

### Lint Frontend

```bash
cd web && npm run lint
```

### Clean All Build Artifacts

```bash
make clean
```

## Project Structure

```
opensnitch-web/
├── cmd/opensnitch-web/     # Main entry point, embedded frontend
├── internal/
│   ├── api/                # HTTP handlers, router, JWT middleware
│   ├── blocklist/          # Blocklist fetching and management
│   ├── config/             # YAML config loading
│   ├── db/                 # SQLite database layer
│   ├── grpcserver/         # gRPC server for daemon communication
│   ├── nodemanager/        # Connected node tracking
│   ├── prompter/           # Interactive prompt handling
│   └── ws/                 # WebSocket hub for real-time events
├── proto/                  # Protobuf definitions
├── web/                    # React frontend source
│   └── src/
│       ├── components/     # Reusable UI components
│       ├── hooks/          # Custom React hooks
│       ├── pages/          # Route pages
│       ├── stores/         # Zustand state stores
│       └── types/          # TypeScript type definitions
├── config.yaml.example     # Example configuration (copied to config.yaml on first run)
├── Dockerfile              # Multi-stage Docker build
└── Makefile                # Build targets
```

## API Overview

All API routes are under `/api/v1/`. Protected routes require a JWT `Authorization: Bearer <token>` header.

| Group | Endpoints | Description |
|---|---|---|
| **Auth** | `POST /auth/login`, `POST /auth/logout`, `GET /auth/me` | Login, logout, current user |
| **Nodes** | `GET /nodes`, `GET /nodes/{addr}`, `PUT /nodes/{addr}/config` | List, inspect, configure nodes |
| **Rules** | `GET /rules`, `POST /rules`, `PUT /rules/{name}`, `DELETE /rules/{name}` | CRUD + enable/disable rules |
| **Connections** | `GET /connections`, `DELETE /connections` | List and purge connection logs |
| **Stats** | `GET /stats`, `GET /stats/{table}` | General and per-table statistics |
| **Firewall** | `GET /firewall`, `POST /firewall/reload` | View and reload firewall chains |
| **Alerts** | `GET /alerts`, `DELETE /alerts/{id}` | View and dismiss alerts |
| **Prompts** | `GET /prompts/pending`, `POST /prompts/{id}/reply` | Handle interactive prompts |
| **WebSocket** | `GET /ws` | Real-time events (connections, prompts, node status) |

## Default Credentials

| Username | Password |
|---|---|
| `admin` | `opensnitch` |

> **Note**: On first run, a unique admin password and JWT secret are auto-generated in `config.yaml`. Check the server log for the generated password.

## License

MIT
