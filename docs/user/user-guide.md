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
# macOS / Linux
sha256sum --check checksums.txt

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
sap-devs --version
```

---

## First-Time Setup

Run the setup wizard:

```bash
sap-devs init
```

The wizard will:
1. Ask you to select a developer profile (e.g. `cap-developer`, `btp-developer`, `abap-developer`)
2. Run an initial content sync
3. Inject SAP context into all detected AI tools

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

**Example:**
```bash
sap-devs inject --tool claude-code --dry-run
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
| `--category` | Sync a single category only |

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

---

### `tip`

Print a random SAP developer tip from your active profile's packs. Add to your shell profile for a tip on every new terminal:

```bash
# ~/.bashrc or ~/.zshrc
sap-devs tip
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

**Example:**
```bash
sap-devs doctor --fix
```

Output:
```
TOOL       REQUIRED    FOUND      STATUS
Node.js    >=18.0.0    20.11.0    ok
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
| `search <query>` | Search resources by keyword |
| `open <id>` | Open a resource URL in the default browser |

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
