# MCQuery — Minecraft Server Query CLI

> **Go (1.25+)** terminal app for querying Minecraft **Bedrock** and **Java** servers directly or via an interactive **domain lookup** mode. Includes raw-terminal navigation, spinner progress UI, and concurrency-aware host probing.

![Runtime](https://img.shields.io/badge/runtime-Go_1.25%2B-00ADD8?logo=go)
![Type](https://img.shields.io/badge/type-CLI-000)
![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-informational)
![Status](https://img.shields.io/badge/stability-stable-success)

> [!IMPORTANT]
> **Project Disclaimer**  
> "MCQuery" by SpindexGFX is an independent project. It is not affiliated with, endorsed by, sponsored by, or otherwise connected to Mojang AB, Microsoft Corporation, or any of their subsidiaries or affiliates. No partnership, approval, or official relationship with Mojang AB or Microsoft is implied.  
> All names, logos, brands, trademarks, service marks, and registered trademarks are the property of their respective owners and are used strictly for identification and reference purposes only. This project does not claim ownership of third-party intellectual property and does not grant any license to use it.

---

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Quickstart](#quickstart)
- [Usage](#usage)
  - [Direct Query (UWP/TCP)](#direct-query-uwptcp)
  - [IP/Domain Lookup](#ipdomain-lookup)
  - [Status API](#status-api)
- [Output Details](#output-details)
- [Directory Layout](#directory-layout)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [Release Automation](#release-automation)
- [License](#license)

---

## Overview

**MCQuery** is a focused CLI tool for checking Minecraft server status. It supports both **Bedrock** (UDP ping with RakNet unconnected ping) and **Java** (status + latency handshake) editions, and ships with an interactive lookup mode that tries combinations of subdomains and domain endings to find reachable servers.

The UI is terminal-native: MCQuery runs in a dedicated full-screen operations console with responsive action/detail panels, live progress telemetry, and scrollable result pages.

---

## Key Features

- 🎯 **Direct server query** for Bedrock and Java editions.
- 🔎 **Lookup mode** to probe subdomain + domain ending combinations.
- ⚡ **Concurrent lookup** with automatic concurrency sizing.
- 🧭 **Interactive terminal UI** with keyboard navigation and live progress.
- 🖥️ **Responsive operations console** with contextual action details and a clean alternate-screen session.
- 📜 **Scrollable result viewer** for large lookups, batches, scans, and verbose output.
- 🧼 **Clean MOTD rendering** (strips Minecraft color codes).

---

## Quickstart

Download `MCQuery.exe` from the latest GitHub release and open it to start the interactive console. The same executable also accepts command-line flags, so no separate CLI binary is required.

Build the Windows executable locally:

```powershell
go build -o MCQuery.exe ./cmd/uwp-tcp-con
.\MCQuery.exe
.\MCQuery.exe --version
```

Build on Linux or macOS:

```bash
go build -o mcquery ./cmd/uwp-tcp-con

./mcquery
```

Or run directly:

```bash
go run ./cmd/uwp-tcp-con
```

Start the API server:

```bash
go run ./cmd/uwp-tcp-con --api --api-addr 127.0.0.1:8080 --api-timeout-ms 1000 --api-concurrency 32
```

Or use the quickstart scripts:

```powershell
.\scripts\start-api.ps1
```

```cmd
scripts\start-api.cmd
```

---

## Usage

On launch you’ll enter the operations console and choose a mode with the arrow keys (or W/S) and Enter. Wide terminals show contextual details beside the selected action; narrow terminals automatically use a stacked layout.

Common controls:

- `Up` / `Down`, `W` / `S`, or `J` / `K`: move or scroll.
- `1`-`9`: open a visible menu action directly.
- `Enter`: open or confirm; `Q`: go back.
- `?` or `F1`: show keyboard help.
- `Page Up` / `Page Down`, `Home` / `End`: navigate long result pages.
- `P` or `Space`: pause/resume supported long-running scans; `Q` cancels them.

### Direct Query (UWP/TCP)

1. Select **UWP/TCP query**.
2. Choose **Bedrock** or **Java**.
3. Enter the host and port (leave empty for the default: 19132 for Bedrock, 25565 for Java).

This sends the protocol-specific ping and renders a formatted status page.

### IP/Domain Lookup

1. Select **IP lookup**.
2. Choose **Bedrock** or **Java**.
3. Pick subdomains:
   - Custom
   - Built-in pool
   - Custom + pool
4. Enter the base host (e.g., `example` for `play.example.com`).
5. Pick domain endings:
   - Custom
   - Built-in pool
   - Custom + pool
6. Enter the port (or leave empty for the default).

The lookup will probe each combination concurrently and report matches.

### Status API

The API keeps the normal CLI untouched and adds fast JSON/SSE endpoints for dashboards, bots, and tools that need player counts from one or more servers at the same time.

Start it with:

```bash
./mcquery --api --api-addr 127.0.0.1:8080
```

Or from a config file:

```bash
./mcquery --api-config config/api.example.json
```

For local changes, copy `config/api.example.json` to `config/api.local.json`. The quickstart scripts will use `api.local.json` automatically when it exists.

Config fields:

```json
{
  "addr": "127.0.0.1:8080",
  "timeout_ms": 1000,
  "concurrency": 32,
  "enable_srv": true,
  "ip_mode": "auto"
}
```

Single server snapshot:

```bash
curl "http://127.0.0.1:8080/api/status?edition=bedrock&host=play.example.com&port=19132&timeout_ms=1000"
```

Group snapshot with parallel queries:

```bash
curl "http://127.0.0.1:8080/api/status?server=bedrock,play.example.com,19132,lobby&server=java,mc.example.com,25565,survival&timeout_ms=1000&concurrency=32"
```

Group snapshot via JSON:

```bash
curl -X POST "http://127.0.0.1:8080/api/status" \
  -H "Content-Type: application/json" \
  -d "{\"timeout_ms\":1000,\"concurrency\":32,\"servers\":[{\"name\":\"lobby\",\"edition\":\"bedrock\",\"host\":\"play.example.com\",\"port\":19132},{\"name\":\"survival\",\"edition\":\"java\",\"host\":\"mc.example.com\",\"port\":25565}]}"
```

Live stream:

```bash
curl "http://127.0.0.1:8080/api/stream?server=bedrock,play.example.com,19132,lobby&server=java,mc.example.com,25565,survival&interval_ms=1000&timeout_ms=1000"
```

The stream uses Server-Sent Events and sends a fresh grouped status payload every interval.

---

## Output Details

- **Bedrock**
  - Edition code, both MOTD lines/world name, protocol/game versions, and player counts.
  - Game mode, server ID, RakNet GUID, advertised IPv4/IPv6 ports, response size, extension fields, and UDP round-trip latency.
- **Java**
  - Version name, protocol version, player counts, advertised player sample, and **latency (ms)**.
  - Server-icon type/dimensions, secure-chat flags, custom status fields, and Forge/FML loader, mod, channel, and truncation metadata when advertised.

Both editions include a **clean MOTD** with Minecraft formatting stripped.

These values come from the public server-list ping. They do not prove that login/authentication succeeds and cannot provide TPS, complete player lists, or world health without a separate authenticated/query-capable server integration.

---

## Directory Layout

```
cmd/uwp-tcp-con/     # CLI entrypoint
config/              # API quickstart config
internal/cli/        # terminal UI, prompts, lookup pools
internal/ping/       # Bedrock/Java protocols + lookup engine
internal/web/        # local link server and status API
scripts/             # quickstart launch scripts
```

---

## Troubleshooting

- **Timeouts**: ensure the server is reachable and the port is open.
- **No IPv4 address**: Bedrock pings require an IPv4 resolution for the host.
- **Terminal input issues**: try running in a real TTY (not a basic shell emulator).
- **Visible sequences such as `←[2m` on Windows**: replace older builds with the latest `MCQuery.exe`; current builds enable Windows virtual-terminal output automatically.

---

## Contributing

1. Fork the repo and create a feature branch.
2. Keep changes minimal and consistent with existing style.
3. Update README if user-facing behavior changes.

---

## Release Automation

CI runs vet, tests, and builds on Linux and Windows. Pushes to `master` also run the release checks. When `appVersion` in `internal/cli/update_check.go` contains a version that has not been published yet, GitHub Actions creates the matching `vX.Y.Z` tag and a GitHub release with generated notes. It then builds `MCQuery.exe` for Windows x64 and `MCQuery-arm64.exe` for Windows ARM64, creates SHA-256 checksums, and attaches all files to the release.

Existing versions are skipped safely, and a configured version older than the latest tag fails instead of publishing a version regression. Add `[skip release]` to a commit message when a push should run without release automation.

---

## License

No license file is currently included in the repository. Please add one if you plan to distribute this project.
