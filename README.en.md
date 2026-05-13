<div align="center">

<img src="https://github.com/openlibrecommunity/material/blob/master/olcrtc.png" width="250" height="250">

# olcrtc-apified

![License](https://img.shields.io/badge/license-WTFPL-0D1117?style=flat-square&logo=open-source-initiative&logoColor=green&labelColor=0D1117)
![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![Docker](https://img.shields.io/badge/-Docker-0D1117?style=flat-square&logo=docker&logoColor=2496ED)

Multi-tenant fork of [olcRTC](https://github.com/openlibrecommunity/olcrtc) with REST API for managing multiple tunnels from a single server.

</div>

## Features

- **Multi-channel management** — one server handles many tunnels simultaneously via REST API
- **Automatic key generation** — unique 256-bit encryption key per channel (ChaCha20-Poly1305)
- **Automatic room creation** — rooms for Jazz are generated on the fly
- **Channel expiration** — configurable TTL per channel with automatic cleanup
- **Persistent storage** — SQLite database, channels survive restarts
- **Bearer token auth** — single master key protects all API endpoints
- **Channel limit** — configurable hard cap on concurrent channels
- **Full channel editing** — update carrier, transport, key, room with automatic tunnel restart
- **Docker-ready** — multi-arch image on Docker Hub (`linux/amd64`, `linux/arm64`)

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/channels` | Create a channel |
| `GET` | `/api/v1/channels` | List all channels |
| `GET` | `/api/v1/channels/{id}` | Get channel by ID |
| `PUT` | `/api/v1/channels/{id}` | Update channel (restarts tunnel) |
| `DELETE` | `/api/v1/channels/{id}` | Delete channel |
| `POST` | `/api/v1/channels/{id}/renew` | Extend channel TTL |
| `GET` | `/api/v1/status` | Server status |

All endpoints require `Authorization: Bearer <OLCRTC_MASTER_KEY>` header.

Full API reference: **[API.md](API.md)** — all parameters for every carrier/transport combination.

## Quick Start (Docker)

### Docker Compose

```bash
export OLCRTC_MASTER_KEY="your-secret-key-here"
docker compose -f docker-compose.api.yml up -d
```

### Docker Run

```bash
docker run -d \
  --name olcrtc-apified \
  -p 8080:8080 \
  -e OLCRTC_MODE=api \
  -e OLCRTC_MASTER_KEY="your-secret-key-here" \
  -e OLCRTC_MAX_CHANNELS=10 \
  -v olcrtc-data:/var/lib/olcrtc \
  burgerbot/olcrtc-apified
```

### Verify

```bash
curl -s -H "Authorization: Bearer your-secret-key-here" \
  http://localhost:8080/api/v1/status
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `OLCRTC_MASTER_KEY` | yes | - | Bearer token for API auth |
| `OLCRTC_MAX_CHANNELS` | no | `10` | Max concurrent channels |
| `OLCRTC_DB_PATH` | no | `/var/lib/olcrtc/channels.db` | SQLite database path |
| `OLCRTC_API_LISTEN` | no | `:8080` | HTTP listen address |
| `OLCRTC_DNS` | no | `1.1.1.1:53` | Default DNS for channels |
| `OLCRTC_DEBUG` | no | `false` | Verbose logging |

## API Usage Examples

### Create a channel (jazz — room auto-generated)

```bash
curl -X POST http://localhost:8080/api/v1/channels \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "carrier": "jazz",
    "transport": "datachannel",
    "client_id": "user-1",
    "ttl_days": 30
  }'
```

### Create a channel (wbstream — room_id required)

```bash
curl -X POST http://localhost:8080/api/v1/channels \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "carrier": "wbstream",
    "transport": "datachannel",
    "client_id": "user-1",
    "room_id": "existing-room-id",
    "ttl_days": 30
  }'
```

### Renew a channel

```bash
curl -X POST http://localhost:8080/api/v1/channels/<id>/renew \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"ttl_days": 60}'
```

### Delete a channel

```bash
curl -X DELETE http://localhost:8080/api/v1/channels/<id> \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY"
```

## Build from Source

```bash
# install mage
go install github.com/magefile/mage@latest

# build cli
mage buildCLI

# cross-compile for linux / windows / darwin
mage cross

# android aar via gomobile
mage mobile

# container image
mage docker

# run tests
mage test

# lint
mage lint
```

## Community

- Telegram: [@openlibrecommunity](https://t.me/openlibrecommunity)
- Android client: [alananisimov/olcbox](https://github.com/alananisimov/olcbox)
- Issues: [GitHub Issues](https://github.com/Cutieeileen/olcrtc-apified/issues)

---

<div align="center">

### Credits

Based on [olcRTC](https://github.com/openlibrecommunity/olcrtc) by **zarazaex**

Telegram: [zarazaex](https://t.me/zarazaexe) | Email: [zarazaex@tuta.io](mailto:zarazaex@tuta.io)

Made for: [olcNG](https://github.com/zarazaex69/olcng)

</div>
