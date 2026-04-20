# sap-devs cli

[![REUSE status](https://api.reuse.software/badge/github.com/SAP-samples/sap-devs-cli)](https://api.reuse.software/info/github.com/SAP-samples/sap-devs-cli)

`sap-devs` injects up-to-date SAP developer knowledge into your AI coding tools, wires SAP MCP servers, and keeps content current automatically.

## Documentation

- [User Guide](docs/user/user-guide.md) — Install, configure, and use `sap-devs`
- [Content Guide](docs/content/content-guide.md) — Add, update, and translate packs
- [Developer Guide](docs/developer/developer-guide.md) — Build, test, and release the CLI
- [Backlog](TODO.md) — Planned features and ideas

## Quick Start

```bash
# Install: download from GitHub Releases, extract, add to PATH
sap-devs version

# First-time setup
sap-devs init

# Keep content current
sap-devs sync

# Inject SAP context into your AI tools
sap-devs inject
```

## Executive Summary

AI coding assistants have become the default way developers write software. Tools like GitHub Copilot, Cursor, and Claude sit open in the background while developers work, answering questions and generating code on demand. By 2026 this shift is well underway, and it is reshaping how developers choose which technologies to build with.

The 2026 SAP Developer Survey shows that 31% of external developers are now using GenAI on production SAP projects - up from essentially zero just two years ago. More striking: 75% of those using these tools report a "profound positive impact" on their effectiveness. When asked about help resources, "consulting an LLM" ranked third, ahead of many traditional SAP channels.
This isn't just developers using a new tool. This represents a fundamental shift in how technical decisions get made.
The problem for SAP is straightforward. These AI tools were trained on general internet content, and their knowledge of SAP technologies is outdated, incomplete, and often wrong. A developer who asks their assistant how to build an SAP application gets bad advice, follows it, and wastes hours debugging code that should have worked. That experience is not neutral. It creates the impression that SAP is difficult, and it quietly steers developers toward alternatives.

sap-devs cli addresses this directly.

It is a small utility that developers install once. A single command delivers SAP's current best practices, recommended tools, and up-to-date guidance straight into the AI assistant the developer already uses, without changing how that tool looks or works. SAP knowledge becomes part of the developer's daily environment rather than a portal they have to remember to visit.

For developers already working in the SAP ecosystem, the benefit is immediate productivity. Accurate, opinionated guidance replaces the friction of correcting AI-generated mistakes. Curated resources and the latest release notes surface at the moment they are relevant, inside the coding environment, without any extra steps.

For developers new to SAP, this changes the first impression entirely. Historically, getting started with SAP required navigating extensive documentation and a steep learning curve. With sap-devs, a developer evaluating SAP for the first time receives clear, correct guidance from their AI assistant from day one. They do not need to know what questions to ask. The right answers show up anyway. That experience builds confidence and accelerates adoption.

For SAP's developer relations work, this opens a new distribution channel. SAP produces high-quality content: tutorials, blog posts, developer news, and curated learning paths. The challenge has always been getting that content in front of developers who are actively working on a problem. sap-devs solves that. SAP's best content reaches developers inside the tools they are already using, at the moment it matters most.

The broader opportunity is that AI assistants are fast becoming one of the most influential factors in which platforms developers choose. Platforms that integrate well into this workflow feel modern and worth learning. Platforms that do not feel like obstacles. Shipping accurate, current SAP knowledge into the AI tools developers use every day is one of the most direct ways SAP can influence that perception in 2026.

## Contributing

If you wish to contribute code, offer fixes or improvements, please send a pull request. Due to legal reasons, contributors will be asked to accept a Developer Certificate of Origin (DCO) when they submit their first pull request to this project. This happens in an automated fashion during the submission process. SAP uses [the standard DCO text of the Linux Foundation](https://developercertificate.org/).

## How to obtain support

This project is provided "as-is": there is no guarantee that raised issues will be answered or addressed in future releases.

## Legal

- [Privacy Statement](PRIVACY_STATEMENT.md)
- [Terms of Use](https://www.sap.com/about/legal/terms-of-use.html)

## License

Copyright (c) 2026 SAP SE or an SAP affiliate company. All rights reserved.
This project is licensed under the Apache Software License, v. 2 except as noted otherwise in the [LICENSE](LICENSES/Apache-2.0.txt) file.
