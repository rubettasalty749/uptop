# Go-Upkeep

Self-hosted uptime monitor with a TUI you can access over SSH. No browser, no install on the client — just `ssh -p 23234 your-server`.

Built on the foundation of [RDGames/go-upkeep](https://github.com/RDGames/go-upkeep).

## What it does

- **6 check types**: HTTP, Push (heartbeat), Ping, Port, DNS, Groups
- **9 alert providers**: Discord, Slack, Email, Ntfy, Webhook, Telegram, PagerDuty, Pushover, Gotify
- **Config as code**: define monitors in YAML, apply declaratively, version control your setup
- **HA clustering**: leader/follower with automatic failover
- **Prometheus metrics**: `/metrics` endpoint for Grafana dashboards
- **Public status page**: HTML + JSON, toggle with an env var
- **SQLite or Postgres**: SQLite for single-node, Postgres for production
- **Uptime Kuma import**: migrate from Kuma with one command

## Quick start

```bash
go run cmd/goupkeep/main.go
ssh -p 23234 localhost
```

Seed some demo data to see it in action:

```bash
go run cmd/goupkeep/main.go -demo
```

## Install

### From source

```bash
go install gitea.lerkolabs.com/lerko/uptime/cmd/goupkeep@latest
```

### Docker

```bash
docker pull lerko/go-upkeep:latest
docker run -p 23234:23234 -p 8080:8080 -v ./data:/data lerko/go-upkeep
```

### Binary

Download from [Releases](https://gitea.lerkolabs.com/lerko/uptime/releases).

## Config as code

Export your current monitors:

```bash
goupkeep export -o monitors.yaml
```

Apply a config file:

```bash
goupkeep apply -f monitors.yaml
goupkeep apply -f monitors.yaml --dry-run   # see what would change
goupkeep apply -f monitors.yaml --prune     # delete anything not in the YAML
```

See [docs/config-as-code.md](docs/config-as-code.md) for the full reference.

## Docker

```yaml
services:
  monitor:
    build: .
    restart: unless-stopped
    stdin_open: true
    tty: true
    ports:
      - "23234:23234"
      - "8080:8080"
    volumes:
      - ./data:/data
      - ./ssh_keys:/app/.ssh
    environment:
      - UPKEEP_DB_TYPE=sqlite
      - UPKEEP_DB_DSN=/data/upkeep.db
      - UPKEEP_STATUS_ENABLED=true
      - UPKEEP_CLUSTER_SECRET=change-me
```

First run: attach to the container (`docker attach go-upkeep`), go to the Users tab, add your SSH public key. Then detach with `Ctrl+P, Ctrl+Q` and connect normally over SSH.

## Environment variables

| Variable | Default | What it does |
|---|---|---|
| `UPKEEP_PORT` | `23234` | SSH server port |
| `UPKEEP_HTTP_PORT` | `8080` | HTTP server port (status page, push, metrics) |
| `UPKEEP_DB_TYPE` | `sqlite` | `sqlite` or `postgres` |
| `UPKEEP_DB_DSN` | `upkeep.db` | Database path or connection string |
| `UPKEEP_STATUS_ENABLED` | `false` | Enable public status page |
| `UPKEEP_STATUS_TITLE` | `System Status` | Status page title |
| `UPKEEP_CLUSTER_MODE` | `leader` | `leader` or `follower` |
| `UPKEEP_PEER_URL` | | Leader URL for follower nodes |
| `UPKEEP_CLUSTER_SECRET` | | Shared key for cluster + API auth |
| `UPKEEP_INSECURE_SKIP_VERIFY` | `false` | Skip TLS verification for checks |

## Migrating from Uptime Kuma

Export your Kuma backup JSON, then:

```bash
curl -X POST http://localhost:8080/api/import/kuma \
  -H "X-Upkeep-Secret: your-secret" \
  -H "Content-Type: application/json" \
  -d @kuma-backup.json
```

## License

MIT — see [LICENSE](LICENSE).
