# Security Policy

## Project
MCQuery  
Repository: https://github.com/Daniel-Ric/MCQuery

---

## Supported Versions

Security fixes are provided for the current `master` branch. Forks and modified copies are not directly supported, but upstream reports are still welcome.

---

## What Is Considered a Security Issue

Please report issues that could affect the confidentiality, integrity, or availability of users, systems, or networks, including:

- command injection, arbitrary code execution, or unsafe process invocation
- server targets or API inputs bypassing intended validation or network restrictions
- malformed Bedrock or Java responses causing panics, excessive allocation, or unsafe parsing
- unbounded concurrency, reads, retries, or requests that enable a realistic denial of service
- terminal escape-sequence or response injection through hostile MOTDs or server metadata
- unintended exposure of configuration, internal errors, local addresses, or private network data
- unsafe default API exposure or missing authorization where the documented deployment model requires it

## Out of Scope

The following are generally not security vulnerabilities:

- public server status, MOTD, version, latency, or player-count information exposed by a server
- a remote server blocking, rate-limiting, or refusing queries
- typos, documentation issues, or ordinary display bugs
- theoretical attacks without a practical path or impact
- deliberately scanning systems without permission by ignoring the documented safe-use requirements
- third-party dependency advisories that are not reachable in this project

---

## How To Report a Vulnerability

**Do not open a public GitHub issue with exploit details.**

Preferred private contact:

`discord.com/users/252138923952832523`

If private contact is not possible, open an issue titled `SECURITY:` with a way to contact you, but do not include exploit details, target addresses, secrets, or proof-of-concept payloads publicly.

Include:

- the affected command, endpoint, protocol, or parser
- exact reproduction steps
- expected and actual behavior
- affected platform and Go version
- potential impact
- a minimal proof of concept, if safe

---

## Disclosure Policy

- Allow reasonable time to investigate and release a fix before public disclosure.
- Test only systems and Minecraft servers you own or are authorized to assess.
- Do not access, disrupt, or scan unrelated infrastructure.
- Public credit can be provided on request; anonymous reports are also respected.

---

## Handling of Sensitive Data

Do not publish private server addresses, credentials, tokens, internal network details, or configuration values in issues or pull requests. Report accidental exposure privately so affected data can be removed or rotated.

---

## Security Practices in This Project

- Minecraft packets, MOTDs, hostnames, and API inputs are treated as untrusted.
- Network operations should use timeouts, bounded reads, cancellation, and limited concurrency.
- Parsers should reject malformed lengths and unexpected payloads without panicking.
- The API should be bound and exposed deliberately for its deployment environment.
- Dependencies should be kept reasonably current.

---

## Proof-of-Concept Testing

Proofs of concept are allowed only against local fixtures and systems you own or have explicit authorization to test. Do not perform denial-of-service testing or broad scans against third-party infrastructure.

Thank you for helping keep MCQuery safe.
