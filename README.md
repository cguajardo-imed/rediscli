# rediscli

A Redis CLI tool with an interactive TUI (Terminal User Interface) built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

---

## Features

- **CLI mode** — send any Redis command directly from the terminal
- **TUI mode** — interactive menu driven interface for common workflows
- **Query Redis** — execute arbitrary Redis commands and display results
- **Publish Create** — create and publish test records with default or custom parameters
- **Publish Create & Delete** — create, publish, then delete records (simulates a full lifecycle)
- **Redis Explorer** — full-screen database browser with DB selector, key-value table, filter and value viewer
- **Session logging** — every TUI session is saved to a timestamped log file

---

## Requirements

- Go 1.24+
- A running Redis instance

---

## Installation

### Clone and build

```sh
git clone https://github.com/your-org/rediscli.git
cd rediscli
```

**Linux / macOS**

```sh
make build
```

**Windows**

```bat
build.bat
```

### Run directly with Go

```sh
go run .
```

---

## Configuration

Configuration is loaded from environment variables. Copy `.env.example` to `.env` and fill in your values, or export them directly in your shell.

| Variable              | Required | Default | Description                        |
| --------------------- | -------- | ------- | ---------------------------------- |
| `REDIS_HOST`          | Yes      | —       | Redis server hostname or IP        |
| `REDIS_PORT`          | No       | `6379`  | Redis server port                  |
| `REDIS_PASSWORD`      | No       | —       | Redis password                     |
| `REDIS_DB`            | No       | `0`     | Default database number (0-15)     |
| `REDIS_POOL_SIZE`     | No       | `200`   | Connection pool size               |
| `REDIS_MIN_IDLE_CONNS`| No       | `50`    | Minimum number of idle connections |

**.env example**

```env
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
```

---

## Usage

### TUI mode (default)

Start the interactive interface by running without arguments:

```sh
./rediscli
```

### CLI mode

Pass any Redis command as arguments:

```sh
./rediscli PING
./rediscli SET mykey "Hello Redis"
./rediscli GET mykey
./rediscli KEYS "*"
./rediscli HSET user:1 name "John Doe" age "30"
./rediscli LPUSH mylist item1 item2 item3
```

### Flags

```sh
./rediscli -h, --help      # show help
./rediscli -v, --version   # show version
```

---

## TUI Menu Options

Navigate with `j`/`k` or `↑`/`↓`, confirm with `enter`, cancel with `esc`.

---

### 1. Query Redis

Execute any Redis command interactively and display the result inline.

```
> GET mykey
> HGETALL user:1
> KEYS user:*
```

---

### 2. Publish Create

Creates a test record in Redis and publishes it to a channel.

**Flow:**

1. Select mode — **Default** or **Custom**
2. If **Custom**, enter parameters:
   - `place_code` — numeric, optional (defaults to `0`)
   - `service_name` — required
   - `custom_params` — optional
3. Enter number of **iterations**
4. If iterations > 1, enter the **delay** between each (e.g. `1s`, `500ms`, `2m`)

While the operation is running, a progress screen shows:

- The parameters entered in the previous steps
- A progress bar
- Current iteration count and percentage
- Elapsed time and estimated time remaining

---

### 3. Publish Create & Delete

Same flow as **Publish Create**, but after each publish the key is deleted and a
second publish is fired to signal the deletion — simulating a full record lifecycle.

---

### 5. Update rediscli

Downloads and installs the latest release from GitHub, replacing the current
binary in place.

**Update process:**

1. Queries the GitHub releases API for the latest version tag
2. Compares it against the currently running version — exits early if already up to date
3. Downloads the correct binary for your platform
4. Verifies the **SHA-256 checksum** of the downloaded file
5. Atomically replaces the running binary (backs up the current one first and
   restores it automatically if anything goes wrong)
6. Reports each step live on screen as it progresses

Restart rediscli after a successful update to run the new version.

**Supported platforms:**

| OS      | Architecture |
| ------- | ------------ |
| Linux   | amd64, arm64 |
| macOS   | amd64, arm64 |
| Windows | amd64        |

---

### 4. Redis Explorer

A full-screen TUI browser for your Redis databases.

#### DB Selector

Displays all 16 Redis databases (DB 0–15) in a 4 × 4 grid.

| Key          | Action                              |
| ------------ | ----------------------------------- |
| `↑` / `k`   | move up one row                     |
| `↓` / `j`   | move down one row                   |
| `←` / `h`   | move left one column                |
| `→` / `l`   | move right one column               |
| `enter`      | open the selected database          |
| `q` / `esc`  | return to the main menu             |

The currently open database is highlighted with a `✓` marker.

#### Key-Value Table

Displays all keys in the selected database with their type, TTL and a value preview.
The table stretches to fill the full terminal width and adapts on resize.

| Key          | Action                                             |
| ------------ | -------------------------------------------------- |
| `↑` / `↓`   | move the row cursor                                |
| `enter`      | open the full value viewer for the selected key    |
| `/`          | open the filter bar                                |
| `r`          | refresh — re-scan the current database             |
| `D`          | jump back to the DB selector                       |
| `esc`        | clear active filter, or go back to the DB selector |
| `q`          | exit the explorer                                  |

Supports all Redis data types: `string`, `hash`, `list`, `set`, `zset`.
Key scan is capped at **500 keys** per database.

#### Filter Bar (press `/` to activate)

Filters the table in real time. Matching is **case-insensitive** and checks both
the key name and the full value.

| Key          | Action                               |
| ------------ | ------------------------------------ |
| `type`       | narrow the table live                |
| `backspace`  | delete the last character            |
| `enter`      | confirm and return focus to table    |
| `esc`        | cancel and clear the filter          |

The status bar shows `N / total key(s) match "query"` while a filter is active.

#### Value Viewer

Shows the full, word-wrapped value of the selected key inside a rounded border box.

| Key                      | Action              |
| ------------------------ | ------------------- |
| `esc` / `enter` / `backspace` | back to the table   |
| `q`                      | exit the explorer   |

---

## Updating

The built-in updater can be triggered from the TUI menu (**Update rediscli**) or
you can run it from the command line by launching the TUI and selecting option 5.

The updater pulls releases from:

```
https://github.com/cguajardo-imed/rediscli/releases/latest
```

Binary asset naming convention:

```
rediscli-linux-amd64
rediscli-linux-arm64
rediscli-darwin-amd64
rediscli-darwin-arm64
rediscli-windows-amd64.exe
```

Each asset ships with a `.sha256` checksum file that is verified before the
binary is installed.

---

## Logging

In TUI mode every session is written to a timestamped log file:

```
logs/rediscli_2025-01-15_14-30-00.log
```

The log file path is printed when the session ends. In CLI mode output goes directly to stdout.

---

## Building with version info

```sh
# Linux / macOS
make build VERSION=1.2.0

# Manual
go build -ldflags "-s -w -X main.Version=1.2.0" -o rediscli .
```

---

## Development

```sh
# Run tests
make test

# Run without building a binary
make run

# Clean build artifacts
make clean
```

---

## Project Structure

```
rediscli/
├── main.go          # Entry point, CLI/TUI dispatch, help text
├── views.go         # Main TUI model, Update/View loop, all menu screens
├── explorer.go      # Redis Explorer TUI (DB selector, table, filter, value viewer)
├── connection.go    # Redis client initialisation, commands, health check
├── config.go        # Configuration loading from environment variables
├── logger.go        # Session logger (file in TUI mode, stdout in CLI mode)
├── utils.go         # Shared helpers (formatting, validation, string utilities)
├── main_test.go     # Tests
├── Makefile         # Build, test, clean targets (Linux/macOS)
├── build.bat        # Build script (Windows)
└── logs/            # Auto-created; holds per-session log files
```

---

## License

See [LICENSE](LICENSE).