# tg-tui

Terminal Telegram client focused on fast private-chat workflows.

## v0.1.0 Highlights

- MTProto auth with phone/code and optional 2FA password.
- QR login flow support.
- Private chat list with periodic refresh.
- Message history loading with load-more pagination.
- Send messages and inline reply workflow.
- Local cache for faster startup and session continuity.
- Keyboard-first TUI with mouse click support.

## Requirements

- Go 1.26.3+
- Telegram API credentials from https://my.telegram.org:
  - `api_id`
  - `api_hash`

## Quick Start

1. Copy env template:

```bash
cp .env.example .env
```

2. Fill your credentials in `.env`:

```dotenv
TG_TUI_API_ID=123456
TG_TUI_API_HASH=your_api_hash_here
TG_TUI_CREDENTIAL_MODE=strict
```

3. Run:

```bash
go run .
```

## Build

```bash
go build ./...
go test ./...
```

To build a local binary with embedded version metadata:

```bash
VERSION=v0.1.0
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo dev)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
go build -ldflags "-X 'main.version=$VERSION' -X 'main.commit=$COMMIT' -X 'main.date=$DATE'" -o bin/tg-tui .
```

## Install

```bash
go install github.com/FeeeLyX/tg-tui@v0.1.0
```

## Version Info

```bash
go run . --version
```

When built with `go build -ldflags`, version metadata is embedded via linker flags.

## Keybindings

- `Enter`: submit auth, open chat, or send message (context dependent)
- `q` / `Ctrl+C`: quit
- `g`: start/regenerate QR login on auth screen
- `Right`: open selected chat messages
- `Left` / `Esc`: return from message view to chat list
- `Up` / `Down`:
  - message view: scroll history
  - chat list view: move selected chat
- `Ctrl+Up` / `Ctrl+Down`: move selected message
- `Ctrl+R`:
  - auth code step: request a new login code
  - message view: set reply target to selected message
- `Ctrl+U`: clear reply target

## Security Notes

- Credentials are loaded from environment variables (`.env` or shell env).
- Data directory is created with private permissions (`0700`).
- Cache/session/log files are enforced as private regular files (`0600`) with symlink checks.
- Telegram transport security is provided by MTProto via `gotd/td`.

## Project Layout

- `main.go`: app entrypoint
- `internal/app/`: config/state and use-cases
- `internal/ports/`: inbound/outbound interfaces
- `internal/telegram/`: Telegram adapter
- `internal/storage/`: local cache adapter
- `internal/ui/`: Bubble Tea model/view/update logic

## Scope

v0.1.0 targets private chats and text messaging. Not included yet:

- media/files
- reactions/stickers
- message edits/deletes
- advanced search
- multi-account support
