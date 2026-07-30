# Contributing

Thank you for contributing to MCQuery.

This repository contains a Go CLI and status API for querying Minecraft Bedrock and Java servers. Contributions should preserve protocol correctness, bounded network behavior, portability, and clear terminal or API output.

## Before You Start

- Read [README.md](README.md) for supported modes, configuration, and API behavior.
- Read [SECURITY.md](SECURITY.md) before reporting vulnerabilities.
- Discuss large features, protocol changes, or breaking CLI/API changes before implementing them.

## Development Setup

Requirements:

- Go 1.25 or newer

Download dependencies:

```bash
go mod download
```

Run tests and static checks:

```bash
go test ./...
go vet ./...
```

Build the application:

```bash
go build ./cmd/uwp-tcp-con
```

Run it directly:

```bash
go run ./cmd/uwp-tcp-con
```

## Repository Structure

- `cmd/uwp-tcp-con`: application entry point
- `internal/cli`: terminal interaction and rendering
- `internal/ping`: Minecraft Bedrock and Java query protocols
- `internal/web`: HTTP status and stream API
- `config`: runtime configuration examples
- `scripts`: launch helpers

## Contribution Guidelines

- Keep changes focused and avoid unrelated refactors.
- Preserve CLI and API contracts unless a change is intentional and documented.
- Bound network operations with timeouts, cancellation, and sensible concurrency.
- Validate target addresses, configuration, packets, MOTDs, and upstream lengths.
- Avoid panics on malformed or hostile server responses.
- Add tests for protocol parsing, validation, and error paths.
- Update `README.md` when commands, configuration, endpoints, or response formats change.

## Coding Expectations

- Format changed Go files with `gofmt`.
- Keep platform-specific behavior isolated and buildable on supported platforms.
- Handle returned errors explicitly.
- Avoid unbounded goroutines, reads, allocations, and retry loops.
- Keep output deterministic where practical.

## Testing Expectations

At minimum:

- run `go test ./...`
- run `go vet ./...`
- build `./cmd/uwp-tcp-con`
- test both Bedrock and Java paths when changing shared query behavior

Network-dependent tests should use controlled local fixtures rather than unrelated public servers.

## Commit Messages

Follow the existing concise, imperative style, for example:

```text
Handle malformed Bedrock pong responses
Add validation for API server targets
Document Java query configuration
```

## Pull Requests

Describe the problem, implementation, CLI/API impact, platform considerations, tests performed, and any remaining follow-up work.

## Security Reporting

Do not disclose vulnerability details in public issues or pull requests. Follow [SECURITY.md](SECURITY.md).

## Conduct

By participating in this project, you agree to follow [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
