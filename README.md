# commitflow

AI-powered git commit analysis and upstream sync tool. Analyze commits using Claude to classify, summarize, and assess impact — then selectively apply upstream changes to your fork.

## Features

- **Commit Analysis** — Classify commits into categories (bug_fix, feature, security, etc.) with impact scoring (1-10)
- **Upstream Sync** — Track upstream repositories and assess which commits are applicable to your fork
- **Smart Diff Handling** — 3-tier chunking strategy keeps AI costs low on large diffs
- **Cost Tracking** — Per-call and session-level budget limits with full token/cost logging
- **SQLite Storage** — Local, zero-config persistence with WAL mode for performance
- **Resumable** — Graceful shutdown and automatic resume from last analyzed commit

## Quick Start

### Prerequisites

- Go 1.25+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) installed and authenticated
- Git

### Install

```bash
go install github.com/0xmhha/commitflow@latest
```

Or build from source:

```bash
git clone https://github.com/0xmhha/commitflow.git
cd commitflow
make build
# Binary at ./bin/commitflow
```

### Analyze Commits

```bash
# Analyze the last 20 commits
commitflow analyze /path/to/repo --last 20

# Analyze a specific range
commitflow analyze /path/to/repo --from abc1234 --to def5678

# Dry run (no AI calls, just see prompts)
commitflow analyze /path/to/repo --last 5 --dry-run

# View a specific analysis
commitflow show abc1234 --repo /path/to/repo

# Generate a filtered report
commitflow report /path/to/repo --category security --impact-min 7
```

### Track Upstream Changes

```bash
# Initialize upstream tracking
commitflow upstream init \
  --upstream https://github.com/ethereum/go-ethereum \
  --fork /path/to/my-fork \
  --name geth

# Scan upstream for applicable commits
commitflow upstream scan --name geth --last 100

# Check sync status
commitflow upstream status --name geth

# Apply a specific commit
commitflow upstream apply abc1234 --name geth

# Skip a commit with reason
commitflow upstream skip abc1234 --name geth --reason "consensus layer, using custom impl"

# Auto mode: scan + apply all applicable
commitflow upstream auto --name geth --last 50
```

## Configuration

commitflow loads configuration from three sources (in priority order):

1. CLI flags (highest)
2. Environment variables (`GUT_*` prefix)
3. YAML config file (`~/.config/git-upstream-tracker/config.yaml`)

### Config File

```yaml
db_path: ~/.local/share/git-upstream-tracker/tracker.db
model: sonnet
max_diff_lines: 5000
delay: 1s
budget: 0          # 0 = unlimited
max_budget_per_call: 0.50
max_retries: 3
retry_backoff: 5s
verbose: false
dry_run: false
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `GUT_DB_PATH` | SQLite database path |
| `GUT_MODEL` | AI model (`sonnet`, `opus`, etc.) |
| `GUT_MAX_DIFF_LINES` | Max diff lines sent to AI |
| `GUT_DELAY` | Delay between API calls |
| `GUT_BUDGET` | Total budget limit in USD |
| `GUT_VERBOSE` | Enable verbose logging |
| `GUT_DRY_RUN` | Dry run mode |

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `~/.config/git-upstream-tracker/config.yaml` | Config file path |
| `--db-path` | `~/.local/share/git-upstream-tracker/tracker.db` | SQLite database path |
| `--model` | `sonnet` | AI model to use |
| `--max-diff-lines` | `5000` | Max diff lines to send to AI |
| `--delay` | `1s` | Delay between API calls |
| `--budget` | `0` | Session budget limit in USD (0 = unlimited) |
| `--verbose` | `false` | Verbose output |
| `--dry-run` | `false` | Simulate without AI calls |

## How It Works

### Commit Analysis

Each commit goes through a pipeline:

1. Extract metadata and diff from git
2. Apply diff chunking (full / Go-only / stat-only based on size)
3. Build a prompt with commit context
4. Call Claude CLI with a JSON schema for structured output
5. Validate and store the result in SQLite

The AI classifies each commit with:
- **Category**: `bug_fix`, `feature`, `refactor`, `security`, `performance`, `docs`, `chore`, `breaking_change`
- **Impact score**: 1 (trivial) to 10 (critical)
- **Breaking changes**: boolean with details
- **Security relevance**: boolean with details

### Upstream Sync

The sync engine evaluates upstream commits against your fork:

1. **Pre-filter**: Skip commits that don't touch files in your fork (no AI call needed)
2. **AI Assessment**: Evaluate applicability considering fork-specific customizations
3. **Cherry-pick**: Apply approved commits on isolated branches

Each upstream commit gets a status: `applicable`, `not_applicable`, `needs_review`, `already_applied`, `applied`, `skipped`, or `conflict`.

## Project Structure

```
commitflow/
├── cmd/                    # CLI commands (cobra)
│   ├── root.go             # Global flags and config
│   ├── analyze.go          # Commit analysis command
│   ├── show.go             # Single commit view
│   ├── report.go           # Filtered report generation
│   └── upstream/           # Upstream sync subcommands
├── internal/
│   ├── ai/                 # Claude CLI wrapper (retry, budget, parsing)
│   ├── analysis/           # Commit analysis engine
│   ├── config/             # YAML + env + flag config merging
│   ├── git/                # Git operations (repository, commit, diff)
│   ├── storage/            # SQLite persistence (commits, syncs, migrations)
│   ├── sync/               # Upstream tracking and cherry-pick
│   └── validate/           # Input validation (refs, hashes, SQL escaping)
├── docs/                   # Architecture and analysis docs
├── Makefile
└── go.mod
```

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-cover

# Run static analysis
make lint

# Build
make build

# Clean
make clean
```

## License

See [LICENSE](LICENSE) for details.
