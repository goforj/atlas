# GoForj Atlas

<p align="center">
  <img src="https://raw.githubusercontent.com/goforj/atlas/main/docs/assets/banner.png" alt="Atlas Banner">
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

- concise project guidance for Codex, Claude Code, GitHub Copilot, and Gemini CLI
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
make build
make release-check
make test
make vet
```

At runtime, Atlas reads docs from `GOFORJ_DOCS_PATH` when set. Otherwise it
clones or refreshes `github.com/goforj/docs` in the user's cache directory,
loads the Markdown tree into memory, and serves MCP docs tools from memory.
Atlas uses the `git` executable when it is available and silently falls back to
native Go git support when it is not.

Atlas is consumed by GoForj as a Go module, not as a prebuilt binary. A release
should run `make release-check`, tag the module, and then bump GoForj to that
tag. The normal docs path is a local git cache loaded into memory by the MCP
server, so Atlas does not need to commit a copied docs tree.

Equivalent direct validation:

```bash
GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./...
```
