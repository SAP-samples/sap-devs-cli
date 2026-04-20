# Privacy Statement for sap-devs CLI

**Effective Date:** April 2026

## 1. Introduction

SAP SE and its affiliates ("SAP") are committed to protecting your privacy. This privacy statement describes how sap-devs CLI ("the Tool") handles data when you install and use it.

The Tool is an open-source command-line utility that injects SAP developer knowledge into AI coding tools. It is designed to operate locally on your machine with minimal data exchange.

## 2. Data Collected

### 2.1 Data Stored Locally

The Tool stores the following data on your local machine only:

- **Configuration** — your selected developer profile, active packs, and tool preferences (stored in your OS config directory)
- **Cached content** — downloaded SAP developer content packs, tutorials, and learning journey metadata (stored in your OS cache directory)
- **Credentials** — optional GitHub tokens for authenticated API access, stored in your OS keychain or a local file with restricted permissions (0600)

This data is not transmitted to SAP or any third party by the Tool itself.

### 2.2 Network Requests

The Tool makes network requests to the following services during normal operation:

| Operation | Destination | Data Sent |
|-----------|-------------|-----------|
| `sap-devs sync` | GitHub API (`api.github.com`) | Repository download request; GitHub token if configured |
| `sap-devs discovery` | SAP Discovery Center OData API | Service/mission queries (no personal data) |
| `sap-devs tutorial` | GitHub API and CDN | Tutorial metadata requests; GitHub token if configured |
| `sap-devs learning` | SAP Learning (`learning.sap.com`) | Catalog and search queries (no personal data) |
| `sap-devs news` | YouTube RSS / SAP Community | Public feed requests (no personal data) |
| `sap-devs events` | SAP events API | Event listing queries (no personal data) |
| `sap-devs update` | GitHub API | Version check request (no personal data) |

No telemetry, analytics, or usage tracking data is collected or transmitted by the Tool.

### 2.3 AI Tool Integration

When you run `sap-devs inject`, the Tool writes SAP developer context into configuration files of your locally installed AI coding tools (e.g., Claude Code, Cursor, GitHub Copilot). This context is processed entirely on your machine. How your AI coding tools handle that context is governed by each tool's own privacy policy.

## 3. GitHub API Usage

If you configure a GitHub token (via `sap-devs config token` or environment variables), that token is used solely to authenticate requests to the GitHub API for content synchronization and tutorial access. The token is stored locally using your OS keychain where available, with a file-based fallback. SAP does not receive or store your GitHub token.

Your use of the GitHub API is subject to [GitHub's Privacy Statement](https://docs.github.com/en/site-policy/privacy-policies/github-general-privacy-statement).

## 4. Third-Party Services

The Tool interacts with the following third-party services. Your use of these services is governed by their respective privacy policies:

- **GitHub** — [GitHub Privacy Statement](https://docs.github.com/en/site-policy/privacy-policies/github-general-privacy-statement)
- **SAP Community** — [SAP Community Privacy Statement](https://pages.community.sap.com/resources/sap-community-privacy-statement)
- **SAP Learning** — [SAP Privacy Statement](https://www.sap.com/about/legal/privacy.html)
- **YouTube** (RSS feeds) — [Google Privacy Policy](https://policies.google.com/privacy)

## 5. Data Retention

All data stored by the Tool resides on your local machine. You can remove it at any time by:

- Deleting the configuration directory (`~/.config/sap-devs` on Linux, `~/Library/Application Support/sap-devs` on macOS, `%APPDATA%/sap-devs` on Windows)
- Deleting the cache directory (`~/.cache/sap-devs` on Linux, `~/Library/Caches/sap-devs` on macOS, `%LOCALAPPDATA%/sap-devs/cache` on Windows)
- Removing any credentials via `sap-devs config token --delete` or your OS keychain manager
- Removing injected content from AI tool configuration files via your editor

## 6. Children's Privacy

The Tool is intended for use by software developers in a professional or educational capacity. It is not directed at children under the age of 16.

## 7. Changes to This Statement

This privacy statement may be updated from time to time. Changes will be reflected in this file within the repository, with the effective date updated accordingly.

## 8. Contact

If you have questions about this privacy statement or the Tool's data practices, please [open an issue](https://github.com/SAP-samples/sap-devs-cli/issues) in the repository.

For general SAP privacy inquiries, visit [SAP's Privacy Portal](https://www.sap.com/about/legal/privacy.html) or contact [privacy@sap.com](mailto:privacy@sap.com).
