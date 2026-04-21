---
name: release-notes
description: Generate release notes from commits since last tag for GoReleaser GitHub Releases
disable-model-invocation: true
---

## Recent Changes
- Commits since last tag: !`git log $(git describe --tags --abbrev=0)..HEAD --oneline`
- Last tag: !`git describe --tags --abbrev=0`

Generate release notes for this Go CLI (`sap-devs`):

1. Group commits by type using conventional commit prefixes:
   - **New Features** — `feat:` commits
   - **Bug Fixes** — `fix:` commits
   - **Documentation** — `docs:` commits
   - **Other Changes** — everything else
2. Write user-friendly descriptions (not raw commit messages)
3. Highlight breaking changes prominently at the top
4. Format as markdown suitable for a GitHub Release body
5. Include a one-line summary at the top describing the release theme
