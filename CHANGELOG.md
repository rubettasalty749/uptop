# Changelog

## [2026.05.2] — 2026-05-23

### Added
- Comprehensive test suite (94 tests across monitor, server, cluster)
- golangci-lint config with CI enforcement
- Gitea Actions CI pipeline (test + lint)
- Graceful shutdown for HTTP and SSH servers
- Context-aware alert delivery with timeout
- Request size limits on all POST endpoints
- Constant-time secret comparison
- Check interval jitter to prevent thundering herd
- `--version` flag with build metadata injection

### Fixed
- Silent JSON unmarshal failures in alert settings
- Panic on crypto/rand failure replaced with error return
- Alert delivery errors now logged instead of swallowed
- log.Fatalf in goroutines replaced with log.Printf
- Deprecated LineUp/LineDown API calls

### Security
- Cluster secret compared with crypto/subtle (timing-safe)
- http.MaxBytesReader on all JSON endpoints
- ReadHeaderTimeout added to HTTP server

## [2026.05.1] — 2026-05-14

### Added
- Distributed probing with leader + probe nodes
- Config-as-code (YAML apply/export with dry-run, prune)
- TUI visual polish (zebra striping, sparklines, breadcrumbs)
- Incident management and maintenance windows
- 9 alert providers (Discord, Slack, Email, Ntfy, Telegram, PagerDuty, Pushover, Gotify, Webhook)

## [2026.04.1] — Initial independent fork

### Added
- SSH-accessible TUI (Bubble Tea + Wish)
- 6 check types (HTTP, Push, Ping, Port, DNS, Group)
- SQLite and PostgreSQL support
- HA clustering with automatic failover
- Prometheus metrics endpoint
- Public status page
- Uptime Kuma import
