# sap-devs User Guide

`sap-devs` injects up-to-date SAP developer knowledge into your AI coding tools (Claude Code, Cursor, GitHub Copilot, and more), wires SAP MCP servers, and keeps content current automatically.

---

## Installation

### Download

Go to the [GitHub Releases page](https://github.tools.sap/developer-relations/sap-devs-cli/releases) and download the archive for your platform:

| Platform | Architecture | File |
|---|---|---|
| Windows | x64 | `sap-devs_<version>_windows_amd64.zip` |
| macOS | Intel | `sap-devs_<version>_darwin_amd64.tar.gz` |
| macOS | Apple Silicon | `sap-devs_<version>_darwin_arm64.tar.gz` |
| Linux | x64 | `sap-devs_<version>_linux_amd64.tar.gz` |
| Linux | ARM64 | `sap-devs_<version>_linux_arm64.tar.gz` |

### Verify checksum (recommended)

Download `checksums.txt` from the same release and verify:

```bash
# Linux
sha256sum --check checksums.txt

# macOS
shasum -a 256 --check checksums.txt

# Windows (PowerShell)
Get-FileHash sap-devs_<version>_windows_amd64.zip -Algorithm SHA256
# Compare output against checksums.txt
```

### Install

**macOS / Linux:**
```bash
tar -xzf sap-devs_<version>_<os>_<arch>.tar.gz
sudo mv sap-devs /usr/local/bin/
# or without sudo:
mkdir -p ~/.local/bin && mv sap-devs ~/.local/bin/
```

If using `~/.local/bin/`, ensure it is on your `PATH`:
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

**Windows:**
1. Extract the ZIP file.
2. Move `sap-devs.exe` to a folder on your `PATH`, or add its folder to `PATH`:
   - Open **System Properties** → **Environment Variables**
   - Under **User variables**, edit `Path` and add the folder containing `sap-devs.exe`
   - Open a new terminal for the change to take effect

### Verify

```bash
sap-devs version
```

---

## First-Time Setup

Run the setup wizard:

```bash
sap-devs init
```

The wizard will:

1. Download SAP developer content (initial sync)
2. Ask you to select a developer profile (e.g. `cap-developer`, `btp-developer`, `abap-developer`)
3. Inject SAP context into all detected AI tools
4. Optionally add `sap-devs tip` to your shell profile so you see a tip on every new terminal

---

## Core Workflow

### Keep content current

```bash
sap-devs sync
```

Fetches the latest SAP developer content from the official repo. Run this periodically or after major SAP releases.

### Inject context into AI tools

```bash
# Inject into all detected tools at user (global) scope
sap-devs inject

# Inject into the current project only
sap-devs inject --project

# Preview what would be written without making changes
sap-devs inject --dry-run
```

### Choose your developer profile

```bash
sap-devs profile list           # see available profiles
sap-devs profile set cap-developer  # set the active profile
sap-devs profile show           # show active profile and pack weights
```

---

## Command Reference

### `inject`

Push SAP context into all detected AI tools.

```
sap-devs inject [flags]
```

| Flag | Description |
|---|---|
| `--project` | Inject at project scope (writes to project config files in the current directory) |
| `--tool <id>` | Inject into a specific tool only (e.g. `claude-code`, `cursor`) |
| `--dry-run` | Preview changes without writing files |
| `--stats` | Show a per-adapter table of packs included, approximate token count, and budget status |
| `--sync` | Force a content sync before injecting (no prompt) |
| `--no-sync` | Skip the freshness check; use cached content as-is |

**Example:**
```bash
sap-devs inject --tool claude-code --dry-run
sap-devs inject --stats --dry-run
```

**`--stats` output example:**

```text
Adapter       Packs included          Tokens (approx)   Budget         Status
claude-code   cap, btp-core, abap     ~750              unconstrained
cursor        cap, btp-core           ~500              2000 tokens    trimmed
```

---

### Authentication

`sap-devs sync` fetches content from `github.tools.sap`, which requires a Personal Access Token if you are inside the SAP corporate network.

**When you need a token:** Only when syncing from `github.tools.sap` on the SAP corporate network. If you are outside SAP, no token is needed.

**Token resolution order** (first match wins):

1. `GITHUB_TOOLS_SAP_TOKEN` environment variable
2. `GH_TOKEN` environment variable
3. `GITHUB_TOKEN` environment variable
4. Token stored with `sap-devs config token`

**Storing a token (interactive — recommended for developer machines):**

```sh
sap-devs config token
# Prompts: Enter GitHub token (input hidden, will not appear in shell history):
```

**Storing a token (non-interactive — scripted or CI):**

```sh
sap-devs config token ghp_yourtoken
# Warning: token passed as argument may be saved in shell history.
```

For CI/CD, set `GITHUB_TOOLS_SAP_TOKEN` as a pipeline secret instead — no local storage needed.

**Where tokens are stored:** The OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service). On headless systems without a keychain, a credentials file at `~/.config/sap-devs/credentials` (Linux) with restricted permissions (owner read/write only). Tokens are **never** stored in `config.yaml`.

**Removing a stored token:**

```sh
sap-devs config token --delete
```

**Viewing token status:**

```sh
sap-devs config show
# github_token:    ghp_****
```

---

### `sync`

Pull the latest SAP developer content from the official repo.

```
sap-devs sync [flags]
```

| Flag | Description |
|---|---|
| `--force` | Re-sync all content regardless of TTL |
| `--category <name>` | Sync a single category only (e.g. `tips`, `tools`, `resources`, `context`, `mcp`, `advocates`) |

---

### `profile`

Manage your developer profile.

```
sap-devs profile list
sap-devs profile set <profile-id>
sap-devs profile show
```

| Subcommand | Description |
|---|---|
| `list` | List all available profiles |
| `set <id>` | Set the active profile |
| `show` | Show the active profile and pack weights |

**Example:**
```bash
sap-devs profile set btp-developer
```

---

### `config`

View and edit `sap-devs` configuration.

```
sap-devs config show
sap-devs config set <key> <value>
sap-devs config company <git-url>
```

| Subcommand | Description |
|---|---|
| `show` | Display the current configuration |
| `set <key> <value>` | Set a configuration value |
| `company <url>` | Configure the company content repo URL (HTTPS) |

**Common config keys:**

| Key | Description | Example |
|---|---|---|
| `language` | Language tag for CLI output and content | `de`, `en` |

> The company content repo is configured via `sap-devs config company <url>` rather than `config set`.

---

### `tip`

Print a SAP developer tip from your active profile's packs. The tip rotates daily.

```bash
sap-devs tip
```

#### `tip install`

Add `sap-devs tip` to your shell profile(s).

```bash
sap-devs tip install
```

#### `tip uninstall`

Remove `sap-devs tip` from your shell profile(s).

```bash
sap-devs tip uninstall
```

#### Adding a daily tip to your terminal startup

During `sap-devs init` you can opt in to this automatically. If you skipped it, run:

```bash
sap-devs tip install
```

This adds `sap-devs tip` to every shell profile found on your system (`.zshrc`, `.bashrc`, `.bash_profile`, `.zprofile` on Linux/macOS; PowerShell profile and Git Bash profiles on Windows).

To remove it:

```bash
sap-devs tip uninstall
```

Open a new terminal and you will see a tip on startup.

**Manual setup (if `tip install` doesn't find your profile):**

```bash
# bash — ~/.bashrc or ~/.bash_profile
echo -e '\n# SAP developer tips\nsap-devs tip' >> ~/.bashrc

# zsh — ~/.zshrc
echo -e '\n# SAP developer tips\nsap-devs tip' >> ~/.zshrc
```

PowerShell — add to your `$PROFILE`:

```powershell
Add-Content $PROFILE "`n# SAP developer tips`nsap-devs tip"
```

---

### `completion`

Generate shell completion scripts so you can tab-complete `sap-devs` commands and flags in your terminal. The completion script is printed to stdout — you need to source or install it yourself.

```
sap-devs completion <shell>
```

Supported shells: `bash`, `zsh`, `fish`, `powershell`

#### bash

```bash
# Current session only
source <(sap-devs completion bash)

# Permanent — add to your ~/.bashrc or ~/.bash_profile
echo 'source <(sap-devs completion bash)' >> ~/.bashrc
source ~/.bashrc
```

#### zsh

```zsh
# Current session only
source <(sap-devs completion zsh)

# Permanent — write to a file on your $fpath
sap-devs completion zsh > "${fpath[1]}/_sap-devs"
```

If you see `command not found: compdef`, enable completions first:

```zsh
echo 'autoload -U compinit; compinit' >> ~/.zshrc
```

#### fish

```fish
sap-devs completion fish | source

# Permanent
sap-devs completion fish > ~/.config/fish/completions/sap-devs.fish
```

#### PowerShell

```powershell
# Current session only
sap-devs completion powershell | Out-String | Invoke-Expression

# Permanent — add to your $PROFILE
Add-Content $PROFILE "`nsap-devs completion powershell | Out-String | Invoke-Expression"
```

---

### `doctor`

Check that the tools required by your active profile are installed and meet version requirements.

```
sap-devs doctor [flags]
```

| Flag | Description |
|---|---|
| `--fix` | Print install commands for failed or missing tools |
| `--profile <id>` | Check a specific profile (`@active` for the configured profile) |

> **Note:** Without `--profile`, `doctor` checks tools from **all packs**, not just your active profile. Use `sap-devs doctor --profile @active` to check only the tools required by your configured profile.

**Example:**
```bash
sap-devs doctor --fix
```

Output:
```
TOOL       REQUIRED    FOUND      STATUS
nodejs     >=18.0.0    20.11.0    ok
cds-dk     >=7.0.0     -          MISSING

Install commands:
  cds-dk: npm install -g @sap/cds-dk
```

---

### `mcp`

Manage SAP MCP (Model Context Protocol) servers. MCP servers give AI tools direct access to SAP APIs and documentation.

```
sap-devs mcp list [--all]
sap-devs mcp status
sap-devs mcp install [id] [--all] [--dry-run]
```

| Subcommand | Description |
|---|---|
| `list` | List available SAP MCP servers (active profile by default; `--all` for all) |
| `status` | Show which SAP MCP servers are registered in your AI tool configs |
| `install [id]` | Wire an SAP MCP server into your AI tools; `--all` installs all for active profile |

**Example:**
```bash
sap-devs mcp list
sap-devs mcp install cap-mcp-server
```

---

### `resources`

Browse curated SAP developer resources from your active profile's packs.

```
sap-devs resources list
sap-devs resources search <query>
sap-devs resources open <id>
```

| Subcommand | Description |
|---|---|
| `list` | List all resources for the active profile |
| `search <query>` | Search resources by keyword (searches across all packs, not just the active profile) |
| `open <id>` | Open a resource URL in the default browser |

> **Note:** `resources list` requires an active profile. Run `sap-devs profile set <id>` first if you haven't done so.

---

### `update`

Update `sap-devs` to the latest release.

```bash
sap-devs update
```

Checks GitHub for a newer release and installs it if found.

---

### `init`

First-time setup wizard. Run once after installation.

```bash
sap-devs init
```

---

## Configuration

The configuration file is at:

| OS | Path |
|---|---|
| Linux | `~/.config/sap-devs/config.yaml` |
| macOS | `~/Library/Application Support/sap-devs/config.yaml` |
| Windows | `%APPDATA%/sap-devs/config.yaml` |

View with `sap-devs config show`. Edit with `sap-devs config set <key> <value>`.

---

## MCP Servers

MCP (Model Context Protocol) servers extend AI tools with direct access to external APIs and data. `sap-devs` can configure SAP MCP servers in your AI tool settings automatically.

```bash
sap-devs mcp list          # see what's available
sap-devs mcp status        # see what's already configured
sap-devs mcp install <id>  # wire a server into your AI tools
```

Supported AI tools include Claude Code, Cursor, and others detected on your system.

---

## Keeping Up to Date

On every command invocation, `sap-devs` checks GitHub for a newer release in the background (at most once per 7 days). If a new version is available, you'll see a notification in the terminal after the command completes.

To update immediately:

```bash
sap-devs update
```

---

## Troubleshooting

**"No tips available" or context appears empty**
→ Run `sap-devs sync` to download the latest content.

**AI tool not detected by `inject`**
→ Ensure the tool is installed and its CLI is on your `PATH`. Check with `sap-devs doctor`.

**`doctor` shows FAIL or MISSING**
→ Run `sap-devs doctor --fix` to see install commands for the missing tools.

**Windows: `sap-devs` not found after installation**
→ Open a new terminal after adding the folder to `PATH`. Environment variable changes require a new shell session.

**Inject writes the wrong language**
→ Set your language: `sap-devs config set language en` (or `de`, etc.).

---

## Platform Notes

Config, cache, and data directories per OS:

| Purpose | Linux | macOS | Windows |
|---|---|---|---|
| Config | `~/.config/sap-devs` | `~/Library/Application Support/sap-devs` | `%APPDATA%/sap-devs` |
| Cache | `~/.cache/sap-devs` | `~/Library/Caches/sap-devs` | `%LOCALAPPDATA%/sap-devs/cache` |
| Data (user content) | `~/.local/share/sap-devs` | `~/Library/Application Support/sap-devs/data` | `%LOCALAPPDATA%/sap-devs/data` |

On Linux, XDG environment variables (`XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `XDG_DATA_HOME`) are honoured.
