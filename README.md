<p align="center">
  <img src="https://img.shields.io/github/actions/workflow/status/reznik99/cloud-storage-api/ci.yml?branch=main&logo=github&label=CI" alt="CI" />
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26" />
  <img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white" alt="PostgreSQL" />
  <img src="https://img.shields.io/badge/Gin-Framework-00ADD8" alt="Gin" />
  <img src="https://img.shields.io/badge/License-Private-lightgrey" alt="License" />
</p>

# GoriniDrive — Backend

REST API and WebSocket signaling server for [GoriniDrive](https://github.com/reznik99/cloud-storage-ui), a self-hosted cloud storage platform with end-to-end encryption and peer-to-peer file transfer.

Built with Go + Gin, backed by PostgreSQL, and designed to run on a single Linux box behind a reverse proxy.

---

## Features

- **End-to-end encrypted storage** — the server never sees plaintext file keys. Clients wrap per-file keys with a master key derived on their side.
- **Shareable links** — public download links with access counters, no auth required for recipients.
- **WebRTC signaling** — WebSocket relay for peer-to-peer file transfer negotiation (offer/answer/ICE).
- **TURN credential generation** — dynamic HMAC-based TURN credentials for NAT traversal.
- **Prometheus metrics** — request counts, error rates, upload/download throughput, shared link downloads.
- **Rate limiting** — in-memory per-IP limiter on authentication routes (brute-force protection).
- **Argon2id password hashing** — 64 MB memory cost, with zxcvbn strength validation on signup.

---

## Architecture

```
┌──────────────┐          ┌──────────────────────────────────┐
│              │  HTTPS   │          GoriniDrive API         │
│   React UI   │────────▶│                                  │
│  (Vite/TS)   │          │  Gin Router                      │
│              │◀────────│   ├─ Rate Limiter (auth routes)  │
└──────────────┘          │   ├─ Session Middleware (cookies)│
                          │   ├─ CORS                        │
┌──────────────┐          │   └─ Prometheus Logger           │
│   WebRTC     │   WS     │                                  │
│  P2P Relay   │◀──────▶│  WebSocket Signaling             │
└──────────────┘          └──────┬───────────┬───────────────┘
                                 │           │
                          ┌──────▼──┐  ┌─────▼─────┐
                          │ Postgres│  │   Disk    │
                          │ (meta)  │  │  (files)  │
                          └─────────┘  └───────────┘
```

File metadata, user accounts, and share links live in PostgreSQL. Actual file blobs are stored on disk with randomized hex filenames — the original name and MIME type are kept in the database (necessary since encrypted content can't be sniffed).

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.26 |
| HTTP Framework | [Gin](https://github.com/gin-gonic/gin) |
| Database | PostgreSQL + [lib/pq](https://github.com/lib/pq) |
| Sessions | [gorilla/sessions](https://github.com/gorilla/sessions) (signed cookies) |
| WebSockets | [gorilla/websocket](https://github.com/gorilla/websocket) |
| Password Hashing | Argon2id via `golang.org/x/crypto` |
| Password Strength | [zxcvbn-go](https://github.com/nbutton23/zxcvbn-go) |
| Rate Limiting | [ulule/limiter](https://github.com/ulule/limiter) (in-memory) |
| Monitoring | [Prometheus client](https://github.com/prometheus/client_golang) |
| Email | [go-mail](https://github.com/wneessen/go-mail) (sendmail) |

---

## Project Structure

```
.
├── main.go                          # Entry point, route registration
├── internal/
│   ├── handler.go                   # HTTP handlers (auth, files, links, sharing)
│   ├── structs.go                   # Request/response types
│   ├── crypto.go                    # Argon2 hashing, password validation, TURN creds
│   ├── cookies.go                   # Session store & CORS setup
│   ├── email.go                     # Password reset emails
│   ├── socket.go                    # WebSocket connection management & relay
│   ├── database/
│   │   ├── database.go              # PostgreSQL connection
│   │   ├── queries.go               # SQL query functions
│   │   └── objects.go               # DB models
│   └── middleware/
│       ├── authentication.go        # Protected route guard
│       ├── ratelimit.go             # Per-IP rate limiter
│       └── logging.go               # Request logger + Prometheus metrics
├── schema/
│   └── new.sql                      # Database schema (4 tables)
├── operations/
│   └── storage-api.service          # Systemd unit file
├── cloud/                           # File storage on disk
└── dist/                            # Build output
```

---

## API Overview

### Authentication
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `POST` | `/api/signup` | — | Create account |
| `POST` | `/api/login` | — | Login, returns session cookie |
| `POST` | `/api/logout` | — | Clear session |
| `GET` | `/api/client_random_value` | — | Get CRV for key derivation |
| `GET` | `/api/reset_password` | — | Request password reset email |
| `POST` | `/api/reset_password` | — | Complete reset with code |

### Account
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/api/session` | Session | Current user info & storage quota |
| `POST` | `/api/change_password` | Session | Change password |
| `POST` | `/api/delete_account` | Session | Delete account (requires password) |

### Files
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/api/files` | Session | List all files |
| `POST` | `/api/file` | Session | Upload (multipart + wrapped key) |
| `GET` | `/api/file` | Session | Download (supports Range requests) |
| `DELETE` | `/api/file` | Session | Delete file |

### Sharing
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `POST` | `/api/link` | Session | Create shareable link |
| `GET` | `/api/link` | Session | Get link for a file |
| `DELETE` | `/api/link` | Session | Delete link |
| `GET` | `/api/link_preview` | — | Preview shared file metadata |
| `GET` | `/api/link_download` | — | Download via share link |

### Other
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/ws` | — | WebSocket signaling relay |
| `GET` | `/api/turn_credentials` | — | TURN server credentials |
| `GET` | `/api/metrics` | Basic | Prometheus metrics |

---

## Getting Started

### Prerequisites

- Go 1.25+
- PostgreSQL
- A sendmail-compatible MTA (for password reset emails)

### Setup

1. **Clone and configure**

   ```bash
   git clone https://github.com/reznik99/GoriniDrive-Backend.git
   cd GoriniDrive-Backend
   cp .env.example .env   # Edit with your values
   ```

2. **Create the database**

   ```bash
   createdb gdrive
   psql -d gdrive -f schema/new.sql
   ```

3. **Run**

   ```bash
   go run .
   ```

   The server starts on the address and port defined in your `.env` (`LISTEN_ADDR` / `LISTEN_PORT`).

### Environment Variables

| Variable | Description |
|----------|-------------|
| `LISTEN_ADDR` | Bind address (e.g. `127.0.0.1`) |
| `LISTEN_PORT` | Bind port (e.g. `8080`) |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | PostgreSQL connection |
| `FILE_STORAGE_PATH` | Directory for uploaded files |
| `COOKIE_NAME` | Session cookie name |
| `COOKIE_AUTH_KEY` | HMAC signing key for sessions |
| `COOKIE_DURATION` | Session TTL (e.g. `1h`) |
| `ALLOWED_ORIGINS` | Comma-separated CORS origins |
| `ENVIROMENT` | `DEV` or `PROD` |
| `EMAIL_ADDRESS` | Sender for password reset emails |
| `METRICS_PASSWORD` | `user:password` for `/api/metrics` basic auth |
| `TURN_SERVER_SECRET` | Shared secret for TURN credential generation |

### Build & Lint

```bash
make build          # Linux amd64 static binary → dist/storage-api
make test           # Run tests
make lint           # Run golangci-lint
make all            # All of the above
```

### Deploy (systemd)

```bash
cp dist/storage-api /home/Storage-Backend/
cp operations/storage-api.service /etc/systemd/system/
systemctl enable --now storage-api
```

---

## Database Schema

Four tables — `users`, `files`, `links`, and `password_reset_codes`. See [`schema/new.sql`](schema/new.sql) for the full DDL.

Key relationships:
- `files.user_id → users.id` (cascade delete)
- `links.file_id → files.id` (cascade delete)
- `links.created_by → users.id` (cascade delete)
- Default storage quota: 1 GB per user

---

## Related

- **Frontend** — [cloud-storage-ui](https://github.com/reznik99/cloud-storage-ui) (React / TypeScript / Vite)
