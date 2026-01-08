# iFlowKit CLI

iFlowKit CLI is a **Go-based, cross-platform** command-line tool that standardizes how SAP Cloud Integration (CPI) teams manage:

- **Developer preferences** (`iflowkit config ...`)
- **Customer profiles** (`iflowkit profile ...`)
- **CPI tenant service keys** per environment (`iflowkit tenant ...`)
- **Git ↔ CPI workflows** for an IntegrationPackage (`iflowkit sync ...`)

> **Project status: Beta (not stable)**  
> iFlowKit CLI is currently in **beta**. Commands, flags, file formats, and behavior may change without notice. Use with caution and validate thoroughly before relying on it in production.

## Disclaimer

This software is provided **"AS IS"**, without warranty of any kind, express or implied, including but not limited to the warranties of merchantability, fitness for a particular purpose and noninfringement.

In no event shall the authors, maintainers, or contributors be liable for any claim, damages, or other liability, whether in an action of contract, tort, or otherwise, arising from, out of, or in connection with the software or the use or other dealings in the software.

For the full legal terms, see `LICENSE`.

## Documentation

- Documentation entry point (language selector): `docs/README.md`

English:
- Getting started: `docs/docs-en/getting-started.md`
- Full CLI reference (all commands + flags): `docs/docs-en/cli-reference.md`
- Sync deep dives:
  - `docs/docs-en/sync/README.md`
  - `docs/docs-en/sync/ignore.md`
  - `docs/docs-en/sync/transports.md`
  - `docs/docs-en/sync/troubleshooting.md`

Türkçe:
- Başlangıç: `docs/docs-tr/getting-started.md`
- CLI referansı: `docs/docs-tr/cli-reference.md`
- Sync:
  - `docs/docs-tr/sync/README.md`
  - `docs/docs-tr/sync/ignore.md`
  - `docs/docs-tr/sync/transports.md`
  - `docs/docs-tr/sync/troubleshooting.md`

## License

This project is licensed under the **Apache License 2.0**. See `LICENSE` and `NOTICE`.

## Build

```bash
go build ./cmd/iflowkit
```

This creates a local binary (usually `./iflowkit`).

## Quick start (foundation)

### 1) Initialize developer config (recommended)

```bash
iflowkit config init
iflowkit config show
```

### 2) Create a profile (interactive)

```bash
iflowkit profile init
iflowkit profile list
```

### 3) Select active profile

```bash
iflowkit profile use --id <profileId>
iflowkit profile current
```

### 4) Add tenant service keys

Import an existing CPI service key JSON:

```bash
iflowkit tenant import --file service-key.json --env dev
iflowkit tenant import --file service-key.json --env qas
iflowkit tenant import --file service-key.json --env prd
```

Or set the fields directly:

```bash
iflowkit tenant set --env dev \
  --url https://<cpi-host>/ \
  --token-url https://<oauth-host>/oauth/token \
  --client-id <id> \
  --client-secret <secret>
```

## Quick start (sync)

### Prerequisites

- `git` installed and available on `PATH`
- A profile selected + tenants imported (`dev` always required)
- A Git provider token in the environment:
  - Preferred: `IFLOWKIT_GIT_TOKEN`
  - GitHub fallback: `GITHUB_TOKEN` or `GH_TOKEN`
  - GitLab fallback: `GITLAB_TOKEN` or `GITLAB_PRIVATE_TOKEN`

### 1) Initialize a sync repo from DEV

```bash
iflowkit sync init --id <packageId>
cd <packageId>
```

### 2) Pull CPI → Git (environment branches only)

```bash
# dev branch
iflowkit sync pull

# prd branch (safety confirmation is mandatory)
git checkout prd
iflowkit sync pull --to prd
```

### 3) Push Git → CPI

```bash
# dev/qas/prd (prd needs explicit confirmation)
iflowkit sync push

git checkout prd
iflowkit sync push --to prd

# work branches (mapped to DEV tenant)
git checkout -b feature/new-flow
iflowkit sync push --message "WIP"
```

### 4) Promote between environments

```bash
# 3-tenant setup: dev -> qas
iflowkit sync deliver --to qas

# prd promotion:
# - 2-tenant setup: dev -> prd
# - 3-tenant setup: qas -> prd
iflowkit sync deliver --to prd
```

## Global flags

All commands accept these global flags **before** the command name:

```bash
iflowkit [--profile <profileId>] [--log-level <trace|debug|info|warn|error>] [--log-format <text|json>] <command> [...]
```

- `--profile <profileId>` overrides the currently active profile.
- `--log-level` defaults to `info`.
- `--log-format` defaults to `text`.

## Configuration directory layout

Config root is:

- `os.UserConfigDir()/iflowkit`

Inside:

- `iflowkit/profiles/<profileId>/profile.json`
- `iflowkit/profiles/<profileId>/tenants/<env>.json`
- `iflowkit/config.json`
- `iflowkit/active_profile`
- `iflowkit/logs/YYYY-MM-DD.log`

> Use `iflowkit where` to print the exact paths on your machine.
