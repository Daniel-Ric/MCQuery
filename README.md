# MCQuery — Minecraft Server Query CLI

> **Go (1.25+)** terminal app for querying Minecraft **Bedrock** and **Java** servers directly or via an interactive **domain lookup** mode. Includes raw-terminal navigation, spinner progress UI, and concurrency-aware host probing.

[![Runtime](https://img.shields.io/badge/runtime-Go_1.25%2B-00ADD8?logo=go)](#)
[![Type](https://img.shields.io/badge/type-CLI-000)](#)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-informational)](#)
[![Status](https://img.shields.io/badge/stability-stable-success)](#)

---

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Quickstart](#quickstart)
- [Usage](#usage)
  - [Direct Query (UWP/TCP)](#direct-query-uwptcp)
  - [IP/Domain Lookup](#ipdomain-lookup)
- [Output Details](#output-details)
- [Directory Layout](#directory-layout)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

---

## Overview

**MCQuery** is a focused CLI tool for checking Minecraft server status. It supports both **Bedrock** (UDP ping with RakNet unconnected ping) and **Java** (status + latency handshake) editions, and ships with an interactive lookup mode that tries combinations of subdomains and domain endings to find reachable servers.

The UI is terminal-native: selection menus use raw input, progress is shown with a spinner, and results are formatted for quick copy/paste.

---

## Key Features

- 🎯 **Direct server query** for Bedrock and Java editions.
- 🔎 **Lookup mode** to probe subdomain + domain ending combinations.
- ⚡ **Concurrent lookup** with automatic concurrency sizing.
- 🧭 **Interactive terminal UI** with keyboard navigation and live progress.
- 🧼 **Clean MOTD rendering** (strips Minecraft color codes).

---

## Quickstart

```bash
# Build
go build -o uwp-tcp-con ./cmd/uwp-tcp-con

# Run
./uwp-tcp-con
```

Or run directly:

```bash
go run ./cmd/uwp-tcp-con
```

---

## Usage

On launch you’ll choose a mode and edition using the arrow keys (or W/S) and Enter.

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

---

## Output Details

- **Bedrock**
  - Game ID, MOTD, protocol/game versions, and player counts.
- **Java**
  - Version name, protocol version, player counts, and **latency (ms)**.

Both editions include a **clean MOTD** with Minecraft formatting stripped.

---

## Directory Layout

```
cmd/uwp-tcp-con/     # CLI entrypoint
internal/cli/        # terminal UI, prompts, lookup pools
internal/ping/       # Bedrock/Java protocols + lookup engine
```

---

## Troubleshooting

- **Timeouts**: ensure the server is reachable and the port is open.
- **No IPv4 address**: Bedrock pings require an IPv4 resolution for the host.
- **Terminal input issues**: try running in a real TTY (not a basic shell emulator).

---

## Contributing

1. Fork the repo and create a feature branch.
2. Keep changes minimal and consistent with existing style.
3. Update README if user-facing behavior changes.

---

## License

No license file is currently included in the repository. Please add one if you plan to distribute this project.
