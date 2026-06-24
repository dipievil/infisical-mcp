# infisical-mcp

A lightweight **MCP (Model Context Protocol) server** written in Go that exposes [Infisical](https://infisical.com/) secrets to AI assistants running in your LAN.

It communicates over **stdio** using JSON-RPC 2.0, so it plugs directly into Claude Desktop, Cursor, VS Code GitHub Copilot, and any other MCP-capable client.

## Features

| Tool | Description |
|------|-------------|
| `list_secrets` | List all secret keys (and optional comments) in a project/environment |
| `get_secret` | Retrieve the plaintext value of a single secret |
| `set_secret` | Update (or create) a secret value – **disabled** when `INFISICAL_SAFE_MODE=true` |

> **Safe Mode** – By default `set_secret` is enabled. Set `INFISICAL_SAFE_MODE=true` to make the server **read-only**. This is recommended for most AI assistant integrations.

## Requirements

- Go 1.22 or later
- A running [Infisical](https://infisical.com/) instance (self-hosted on your LAN or cloud)
- A **Universal Auth** machine identity (client ID + client secret) with at least `read` access to the target project/environment

## Installation

### From source

```bash
git clone https://github.com/dipievil/infisical-mcp.git
cd infisical-mcp
go build -o infisical-mcp ./cmd/server
```

### Using `go install`

```bash
go install github.com/dipievil/infisical-mcp/cmd/server@latest
```

The binary will be placed in `$(go env GOPATH)/bin/infisical-mcp` (or `~/go/bin/infisical-mcp`).

## Configuration

All configuration is done via environment variables.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `INFISICAL_HOST` | ✅ | – | Base URL of your Infisical instance, e.g. `http://192.168.1.10:8080` |
| `INFISICAL_CLIENT_ID` | ✅ | – | Universal Auth machine identity client ID |
| `INFISICAL_CLIENT_SECRET` | ✅ | – | Universal Auth machine identity client secret |
| `INFISICAL_PROJECT_ID` | ✅ | – | Infisical project (workspace) ID |
| `INFISICAL_ENVIRONMENT` | ✅ | – | Environment slug, e.g. `dev`, `staging`, `prod` |
| `INFISICAL_SAFE_MODE` | ❌ | `false` | Set to `true` to disable `set_secret` (read-only mode) |

### Quick test

```bash
export INFISICAL_HOST=http://192.168.1.10:8080
export INFISICAL_CLIENT_ID=<your-client-id>
export INFISICAL_CLIENT_SECRET=<your-client-secret>
export INFISICAL_PROJECT_ID=<your-project-id>
export INFISICAL_ENVIRONMENT=dev
export INFISICAL_SAFE_MODE=true   # optional, enables read-only mode

./infisical-mcp
```

The server starts and waits for MCP JSON-RPC messages on stdin.

## AI Tool Configuration Examples

### Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "infisical": {
      "command": "/path/to/infisical-mcp",
      "env": {
        "INFISICAL_HOST": "http://192.168.1.10:8080",
        "INFISICAL_CLIENT_ID": "your-client-id",
        "INFISICAL_CLIENT_SECRET": "your-client-secret",
        "INFISICAL_PROJECT_ID": "your-project-id",
        "INFISICAL_ENVIRONMENT": "dev",
        "INFISICAL_SAFE_MODE": "true"
      }
    }
  }
}
```

### Cursor

Edit `~/.cursor/mcp.json` (or via **Cursor → Settings → MCP**):

```json
{
  "mcpServers": {
    "infisical": {
      "command": "/path/to/infisical-mcp",
      "env": {
        "INFISICAL_HOST": "http://192.168.1.10:8080",
        "INFISICAL_CLIENT_ID": "your-client-id",
        "INFISICAL_CLIENT_SECRET": "your-client-secret",
        "INFISICAL_PROJECT_ID": "your-project-id",
        "INFISICAL_ENVIRONMENT": "dev",
        "INFISICAL_SAFE_MODE": "true"
      }
    }
  }
}
```

### VS Code (GitHub Copilot)

Add to your `.vscode/mcp.json` workspace file or user settings (`settings.json`):

```json
{
  "mcp": {
    "servers": {
      "infisical": {
        "type": "stdio",
        "command": "/path/to/infisical-mcp",
        "env": {
          "INFISICAL_HOST": "http://192.168.1.10:8080",
          "INFISICAL_CLIENT_ID": "your-client-id",
          "INFISICAL_CLIENT_SECRET": "your-client-secret",
          "INFISICAL_PROJECT_ID": "your-project-id",
          "INFISICAL_ENVIRONMENT": "dev",
          "INFISICAL_SAFE_MODE": "true"
        }
      }
    }
  }
}
```

### Zed

Edit `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "infisical": {
      "command": {
        "path": "/path/to/infisical-mcp",
        "env": {
          "INFISICAL_HOST": "http://192.168.1.10:8080",
          "INFISICAL_CLIENT_ID": "your-client-id",
          "INFISICAL_CLIENT_SECRET": "your-client-secret",
          "INFISICAL_PROJECT_ID": "your-project-id",
          "INFISICAL_ENVIRONMENT": "dev",
          "INFISICAL_SAFE_MODE": "true"
        }
      }
    }
  }
}
```

### Windsurf

Edit `~/.codeium/windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "infisical": {
      "command": "/path/to/infisical-mcp",
      "env": {
        "INFISICAL_HOST": "http://192.168.1.10:8080",
        "INFISICAL_CLIENT_ID": "your-client-id",
        "INFISICAL_CLIENT_SECRET": "your-client-secret",
        "INFISICAL_PROJECT_ID": "your-project-id",
        "INFISICAL_ENVIRONMENT": "dev",
        "INFISICAL_SAFE_MODE": "true"
      }
    }
  }
}
```

## Available MCP Tools

### `list_secrets`

Lists all secret keys available in the configured project and environment.

**Parameters:** none

**Returns:** JSON array of `{ key, comment?, version? }` objects.

**Example response:**
```json
[
  { "key": "DATABASE_URL", "version": 3 },
  { "key": "API_KEY",      "comment": "Third-party API key", "version": 1 }
]
```

---

### `get_secret`

Retrieves the plaintext value of a single secret.

**Parameters:**

| Name | Type   | Required | Description |
|------|--------|----------|-------------|
| `key`| string | ✅ | Secret name to look up |

**Returns:** JSON object `{ key, value, comment?, version? }`.

**Example response:**
```json
{ "key": "DATABASE_URL", "value": "******db:5432/app", "version": 3 }
```

---

### `set_secret`

Updates (or creates) the value of a secret. Requires `INFISICAL_SAFE_MODE=false` (the default).

**Parameters:**

| Name    | Type   | Required | Description |
|---------|--------|----------|-------------|
| `key`   | string | ✅ | Secret name to update |
| `value` | string | ✅ | New plaintext value |

**Returns:** JSON object `{ status: "ok", key }` on success.

**Example response:**
```json
{ "status": "ok", "key": "DATABASE_URL" }
```

## Development

```bash
# Run tests
go test ./...

# Lint (requires golangci-lint)
golangci-lint run

# Build
go build -o infisical-mcp ./cmd/server
```

## Security Considerations

- Store credentials in a secrets manager (or OS keychain), **not** in plain-text config files checked into version control.
- Use `INFISICAL_SAFE_MODE=true` for any integration where the AI assistant does not need to modify secrets.
- Create a dedicated machine identity in Infisical with minimal required permissions (e.g. `secrets:read` only when safe mode is on).
- Use HTTPS (`https://`) when connecting to Infisical over an untrusted network.

## License

[MIT](LICENSE)
