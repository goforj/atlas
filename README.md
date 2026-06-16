# GoForj Atlas

<p align="center">
  <img src="./docs/assets/banner.png" alt="Atlas Banner">
</p>

Agent-native project navigation and MCP tooling for GoForj.

Atlas helps local coding agents understand GoForj projects without guessing
framework conventions. It installs concise project guidance, synchronizes
skills or agent-native instruction files, configures one project-level MCP
server, and exposes safe read-only project inspection tools.

Users should normally reach Atlas through the GoForj CLI:

```bash
forj atlas:install
forj atlas:mcp
```

This repository contains the reusable Atlas library. The GoForj CLI exposes it
through `forj atlas:*` commands so projects do not need to install a separate
binary.

## What Atlas Provides

Atlas gives local agents a framework-aware view of a GoForj project:

- concise project guidance for Codex, Claude Code, and GitHub Copilot
- synchronized skills and agent-native instruction files
- one project-level MCP server
- app-aware project layout, route, schedule, and command inspection
- version-aware docs search and section reads
- safe database, log, browser, URL, and metrics inspection hooks

## Safety Model

Atlas starts read-only. It does not expose arbitrary shell execution or
write-capable MCP tools in the MVP.

When source scaffolding is needed, agents should use normal GoForj commands:

```bash
forj make:controller users
forj marketplace make:job sync-catalog
```

## Development

```bash
GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./...
```
