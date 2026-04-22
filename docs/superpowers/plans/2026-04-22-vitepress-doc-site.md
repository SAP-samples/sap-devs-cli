# VitePress Documentation Site with Fiori Styling — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a VitePress documentation site at `/docs-site/` with full SAP Fundamental Styles theming, deployed to GitHub Pages.

**Architecture:** Content is copied at build time from `/docs/` to keep original markdown as single source of truth. The theme extends VitePress's default with SAP Horizon CSS variables, cherry-picked Fundamental Styles components, and a custom shellbar. Archive sidebar items are auto-generated from 50+ spec/plan files.

**Tech Stack:** VitePress 1.6+, Vue 3.5+, fundamental-styles, @sap-theming/theming-base-content, Node.js (copy script), GitHub Actions

**Spec:** `docs/superpowers/specs/2026-04-21-vitepress-doc-site-design.md`

**Reference implementation:** The SWAPI project at `d:/projects/cloud-cap-hana-swapi/site/` uses the same VitePress + content-copy pattern. Use it as a reference for `copy-content.js`, `config.mts`, and GitHub Actions workflow structure.

---

## File Map

| File | Responsibility |
| --- | --- |
| `docs-site/package.json` | Dependencies and npm scripts |
| `docs-site/.gitignore` | Ignore copied content dirs, build artifacts, node_modules |
| `docs-site/scripts/copy-content.js` | Copy content from `/docs/` + generate `archive-sidebar.json` |
| `docs-site/.vitepress/config.mts` | VitePress config: nav, sidebar (imports archive-sidebar.json), base URL, search, markdown theme |
| `docs-site/.vitepress/theme/index.ts` | Theme entry: extends default, registers FioriHome + FioriShellbar, imports style.css |
| `docs-site/.vitepress/theme/style.css` | Three CSS layers: SAP Horizon variables, Fundamental Styles imports, VitePress bridge |
| `docs-site/.vitepress/theme/components/FioriShellbar.vue` | Custom shellbar replacing VitePress navbar |
| `docs-site/.vitepress/theme/components/FioriHome.vue` | Hero banner + feature cards homepage |
| `docs-site/index.md` | Homepage frontmatter (layout: FioriHome) |
| `docs-site/public/favicon.ico` | SAP-style favicon |
| `docs-site/public/og-image.svg` | Open Graph social preview image |
| `.github/workflows/docs.yml` | GitHub Actions: copy → build → deploy to Pages |

---

### Task 1: Project Scaffold

**Files:**
- Create: `docs-site/package.json`
- Create: `docs-site/.gitignore`

- [ ] **Step 1: Create `docs-site/package.json`**

```json
{
  "name": "sap-devs-cli-docs",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "node scripts/copy-content.js && vitepress dev",
    "build": "vitepress build",
    "preview": "vitepress preview"
  },
  "devDependencies": {
    "vitepress": "^1.6.0",
    "vue": "^3.5.0",
    "fundamental-styles": "^0.39.0",
    "@sap-theming/theming-base-content": "^11.18.0"
  }
}
```

- [ ] **Step 2: Create `docs-site/.gitignore`**

```
guide/
developer/
archive/
.vitepress/dist/
.vitepress/cache/
.vitepress/archive-sidebar.json
node_modules/
```

- [ ] **Step 3: Install dependencies**

Run: `cd docs-site && npm install`
Expected: `node_modules/` created, `package-lock.json` generated

- [ ] **Step 4: Commit**

```bash
git add docs-site/package.json docs-site/package-lock.json docs-site/.gitignore
git commit -m "feat(docs-site): scaffold VitePress project with Fundamental Styles deps"
```

---

### Task 2: Content Copy Script

**Files:**
- Create: `docs-site/scripts/copy-content.js`

This script copies markdown from `/docs/` into VitePress source dirs and auto-generates the archive sidebar JSON from spec/plan filenames.

**Reference:** `d:/projects/cloud-cap-hana-swapi/site/scripts/copy-content.js` — same pattern but simpler (no link rewrites needed, plus archive auto-discovery).

- [ ] **Step 1: Create `docs-site/scripts/copy-content.js`**

```js
#!/usr/bin/env node
import { mkdirSync, readFileSync, writeFileSync, readdirSync } from 'fs'
import { resolve, dirname, basename } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const root = resolve(__dirname, '../..') // repo root
const site = resolve(__dirname, '..') // docs-site/

// --- Static content mappings ---
const mappings = [
  // Getting Started
  { src: 'docs/user/user-guide.md', dest: 'guide/user-guide.md' },
  // Developer
  { src: 'docs/developer/developer-guide.md', dest: 'developer/developer-guide.md' },
  { src: 'docs/developer/dependencies.md', dest: 'developer/dependencies.md' },
  { src: 'docs/developer/external-apis.md', dest: 'developer/external-apis.md' },
  { src: 'docs/developer/security-review.md', dest: 'developer/security-review.md' },
  { src: 'docs/content-authoring.md', dest: 'developer/content-authoring.md' },
  { src: 'docs/content/content-guide.md', dest: 'developer/content-guide.md' },
  { src: 'docs/mcp-server.md', dest: 'developer/mcp-server.md' },
]

// --- Copy static files ---
for (const { src, dest } of mappings) {
  const srcPath = resolve(root, src)
  const destPath = resolve(site, dest)
  mkdirSync(dirname(destPath), { recursive: true })
  const content = readFileSync(srcPath, 'utf8')
  writeFileSync(destPath, content, 'utf8')
  console.log(`Copied: ${src} → ${dest}`)
}

// --- Auto-discover and copy archive files ---
function discoverArchive(srcDir, destDir) {
  const srcPath = resolve(root, srcDir)
  const files = readdirSync(srcPath).filter(f => f.endsWith('.md')).sort().reverse()
  const items = []

  for (const file of files) {
    const content = readFileSync(resolve(srcPath, file), 'utf8')
    const destFile = resolve(site, destDir, file)
    mkdirSync(dirname(destFile), { recursive: true })
    writeFileSync(destFile, content, 'utf8')

    // Extract title from first # heading, or fall back to cleaned filename
    const match = content.match(/^#\s+(.+)$/m)
    const slug = file.replace(/\.md$/, '')
    const text = match
      ? match[1].replace(/\*\*/g, '').replace(/[—–]/g, '—').trim()
      : slug.replace(/^\d{4}-\d{2}-\d{2}-/, '').replace(/-/g, ' ').replace(/\b\w/g, c => c.toUpperCase())

    items.push({ text, link: `/${destDir}/${slug}` })
  }

  console.log(`Archive: ${files.length} files copied to ${destDir}/`)
  return items
}

const specs = discoverArchive('docs/superpowers/specs', 'archive/specs')
const plans = discoverArchive('docs/superpowers/plans', 'archive/plans')

// --- Write sidebar JSON ---
const sidebarPath = resolve(site, '.vitepress/archive-sidebar.json')
mkdirSync(dirname(sidebarPath), { recursive: true })
writeFileSync(sidebarPath, JSON.stringify({ specs, plans }, null, 2), 'utf8')
console.log(`\nSidebar: .vitepress/archive-sidebar.json (${specs.length} specs, ${plans.length} plans)`)

console.log(`\nDone. ${mappings.length + specs.length + plans.length} files processed.`)
```

- [ ] **Step 2: Test the copy script locally**

Run: `cd docs-site && node scripts/copy-content.js`
Expected:
- `guide/`, `developer/`, `archive/specs/`, `archive/plans/` directories created
- `archive-sidebar.json` written with `specs` and `plans` arrays
- Console output listing all copied files and counts

- [ ] **Step 3: Verify archive-sidebar.json structure**

Run: `cat docs-site/.vitepress/archive-sidebar.json | head -20`
Expected: JSON with `{ "specs": [ { "text": "...", "link": "/archive/specs/..." }, ... ], "plans": [...] }`
Items should be sorted by date descending (newest first — filenames sort reverse alphabetically).

- [ ] **Step 4: Commit**

```bash
git add docs-site/scripts/copy-content.js
git commit -m "feat(docs-site): content copy script with archive auto-discovery"
```

---

### Task 3: VitePress Configuration

**Files:**
- Create: `docs-site/.vitepress/config.mts`
- Create: `docs-site/index.md`

- [ ] **Step 1: Create `docs-site/.vitepress/config.mts`**

```ts
import { defineConfig } from 'vitepress'
import { readFileSync, existsSync } from 'fs'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))

// Load auto-generated archive sidebar (created by copy-content.js)
const archivePath = resolve(__dirname, 'archive-sidebar.json')
const archive = existsSync(archivePath)
  ? JSON.parse(readFileSync(archivePath, 'utf-8'))
  : { specs: [], plans: [] }

// Read version from package.json for shellbar badge
const pkg = JSON.parse(readFileSync(resolve(__dirname, '../package.json'), 'utf-8'))

export default defineConfig({
  title: 'sap-devs',
  description: 'SAP developer knowledge, injected into your AI coding tools',
  base: '/sap-devs-cli/',
  ignoreDeadLinks: true,

  vite: {
    define: {
      __APP_VERSION__: JSON.stringify(pkg.version || '0.0.0'),
    },
  },

  markdown: {
    theme: {
      dark: 'vitesse-dark',
      light: 'vitesse-light',
    },
  },

  themeConfig: {
    siteTitle: 'sap-devs',

    nav: [
      { text: 'Guide', link: '/guide/user-guide' },
      { text: 'Developer', link: '/developer/developer-guide' },
      { text: 'Archive', link: '/archive/specs/' },
    ],

    sidebar: {
      '/': [
        {
          text: 'Getting Started',
          items: [
            { text: 'Overview', link: '/' },
            { text: 'User Guide', link: '/guide/user-guide' },
          ],
        },
        {
          text: 'Developer',
          items: [
            { text: 'Developer Guide', link: '/developer/developer-guide' },
            { text: 'Content Authoring', link: '/developer/content-authoring' },
            { text: 'Content Guide', link: '/developer/content-guide' },
            { text: 'MCP Server', link: '/developer/mcp-server' },
            { text: 'Dependencies', link: '/developer/dependencies' },
            { text: 'External APIs', link: '/developer/external-apis' },
            { text: 'Security Review', link: '/developer/security-review' },
          ],
        },
        {
          text: 'Design Archive',
          items: [
            { text: 'Specs', collapsed: true, items: archive.specs },
            { text: 'Plans', collapsed: true, items: archive.plans },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/SAP-samples/sap-devs-cli' },
    ],

    search: { provider: 'local' },
  },
})
```

- [ ] **Step 2: Create `docs-site/index.md`**

```markdown
---
layout: FioriHome
title: sap-devs Documentation
---
```

This is a placeholder — the `FioriHome` layout component (Task 6) renders the full homepage. For now, VitePress will fall back to the default layout until the component is registered.

- [ ] **Step 3: Test VitePress builds and serves**

Run: `cd docs-site && node scripts/copy-content.js && npx vitepress dev`
Expected: Dev server starts on localhost:5173 (or next available port). The site shows content pages with VitePress default styling. The sidebar has Getting Started, Developer, and Design Archive sections. Archive groups show collapsed spec/plan lists.

The homepage will show raw frontmatter or an error until `FioriHome` is registered in Task 6 — that's expected.

- [ ] **Step 4: Commit**

```bash
git add docs-site/.vitepress/config.mts docs-site/index.md
git commit -m "feat(docs-site): VitePress config with sidebar, nav, search, and archive import"
```

---

### Task 4: Fiori Theme — CSS Layers

**Files:**
- Create: `docs-site/.vitepress/theme/style.css`
- Create: `docs-site/.vitepress/theme/index.ts`

This is the core theming task. The CSS file implements three layers:
1. SAP Horizon light variables at `:root`, dark overrides at `html.dark`
2. Cherry-picked Fundamental Styles component CSS imports
3. VitePress variable bridge mapping `--sap*` → `--vp-c-*`

- [ ] **Step 1: Create `docs-site/.vitepress/theme/style.css`**

```css
/* ============================================================
   Layer 1: SAP Theming Base — sap_horizon (light)
   Import the light theme variables at :root scope.
   ============================================================ */
@import '@sap-theming/theming-base-content/content/Base/baseLib/sap_horizon/css_variables.css';

/* ============================================================
   Layer 1b: SAP Horizon Dark overrides
   @sap-theming ships flat :root files — we scope dark values
   under html.dark so VitePress's toggle activates them.
   ============================================================ */
html.dark {
  --sapBackgroundColor: #12171c;
  --sapBaseColor: #1d232a;
  --sapShellColor: #1d232a;
  --sapBrandColor: #0070f2;
  --sapHighlightColor: #4db1ff;
  --sapTextColor: #f5f6f7;
  --sapTitleColor: #f5f6f7;
  --sapLinkColor: #008fff;
  --sapSelectedColor: #4db1ff;
  --sapShell_Background: #1d232a;
  --sapShell_TextColor: #f5f6f7;
  --sapShell_BorderColor: #2e3742;
  --sapShell_Navigation_Background: #1d232a;
  --sapShell_Navigation_TextColor: #f5f6f7;
  --sapShell_Navigation_SelectedColor: #4db1ff;
  --sapGroup_ContentBorderColor: #323c48;
  --sapHoverColor: #283340;
  --sapContent_ForegroundColor: #1d232a;
  --sapField_BorderColor: #a9b4be;
  --sapErrorColor: #fa6161;
  --sapWarningColor: #ffdf72;
  --sapSuccessColor: #97dd40;
}

/* ============================================================
   Layer 2: Fundamental Styles — cherry-picked component CSS
   ============================================================ */
@import 'fundamental-styles/dist/icon.css';
@import 'fundamental-styles/dist/button.css';
@import 'fundamental-styles/dist/shellbar.css';
@import 'fundamental-styles/dist/card.css';
@import 'fundamental-styles/dist/badge.css';
@import 'fundamental-styles/dist/message-strip.css';
@import 'fundamental-styles/dist/breadcrumb.css';
@import 'fundamental-styles/dist/avatar.css';

/* ============================================================
   Layer 3: VitePress Bridge — map --sap* to --vp-c-*
   ============================================================ */
:root {
  --vp-c-brand-1: var(--sapBrandColor);
  --vp-c-brand-2: var(--sapHighlightColor);
  --vp-c-brand-3: var(--sapSelectedColor);
  --vp-c-brand-soft: rgba(0, 112, 242, 0.14);

  --vp-c-bg: var(--sapBackgroundColor);
  --vp-c-bg-soft: var(--sapContent_ForegroundColor, #eff1f2);
  --vp-c-bg-mute: var(--sapBaseColor);

  --vp-c-text-1: var(--sapTextColor);
  --vp-c-text-2: var(--sapShell_Navigation_TextColor, #556b82);
  --vp-c-text-3: var(--sapField_BorderColor, #8c9baa);

  --vp-c-divider: var(--sapGroup_ContentBorderColor);
  --vp-c-border: var(--sapGroup_ContentBorderColor);

  --vp-sidebar-bg-color: var(--sapShell_Navigation_Background, #fff);
  --vp-nav-bg-color: var(--sapShell_Background);

  --vp-button-brand-bg: var(--sapBrandColor);
  --vp-button-brand-hover-bg: var(--sapHighlightColor);
  --vp-button-brand-text: #fff;

  --vp-font-family-base: var(--sapFontFamily, '72', '72full', Arial, Helvetica, sans-serif);
}

html.dark {
  --vp-c-brand-soft: rgba(77, 177, 255, 0.16);
}

/* ============================================================
   Shellbar: hide VitePress default nav, style shellbar
   ============================================================ */
.VPNav {
  display: none !important;
}

/* ============================================================
   Sidebar: Fiori overrides on VitePress built-in sidebar
   ============================================================ */
.VPSidebar .group .title {
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.VPSidebarItem.is-active > .item > .link {
  border-left: 3px solid var(--sapShell_Navigation_SelectedColor, #0064d9);
  color: var(--sapShell_Navigation_SelectedColor, #0064d9);
}

.VPSidebarItem .link:hover {
  background: var(--sapHoverColor);
}

/* ============================================================
   Content area: Fiori typography and element refinements
   ============================================================ */
.vp-doc h1 {
  font-weight: 700;
  letter-spacing: -0.01em;
}

.vp-doc a {
  color: var(--sapLinkColor);
}

.vp-doc code {
  color: var(--sapBrandColor);
}
```

- [ ] **Step 2: Create `docs-site/.vitepress/theme/index.ts`**

```ts
import DefaultTheme from 'vitepress/theme'
import './style.css'

export default {
  extends: DefaultTheme,
}
```

This is a minimal starter — we'll register `FioriHome` and `FioriShellbar` components in Tasks 5 and 6. Starting with just the CSS layers lets us verify the theme works before adding custom components.

- [ ] **Step 3: Test the Fiori theme**

Run: `cd docs-site && node scripts/copy-content.js && npx vitepress dev`
Expected:
- Site loads with SAP Horizon colors (blue brand, light grey background)
- Toggle dark mode — colors switch to Horizon Dark (dark background, blue highlights)
- VitePress default navbar is hidden (we'll add the shellbar in Task 5)
- Sidebar has Fiori-style text (uppercase group labels, blue active highlight)
- Content area uses SAP `72` font family

- [ ] **Step 4: Commit**

```bash
git add docs-site/.vitepress/theme/style.css docs-site/.vitepress/theme/index.ts
git commit -m "feat(docs-site): Fiori theme with SAP Horizon CSS layers and VitePress bridge"
```

---

### Task 5: FioriShellbar Component

**Files:**
- Create: `docs-site/.vitepress/theme/components/FioriShellbar.vue`
- Modify: `docs-site/.vitepress/theme/index.ts`

The shellbar replaces the hidden VitePress navbar. It renders in the `layout-top` slot and includes a search trigger that programmatically opens VitePress's built-in search.

- [ ] **Step 1: Create `docs-site/.vitepress/theme/components/FioriShellbar.vue`**

```vue
<script setup lang="ts">
import { useData } from 'vitepress'

declare const __APP_VERSION__: string

const { isDark } = useData()
const version = __APP_VERSION__ ?? '0.0.0'

function toggleDark() {
  isDark.value = !isDark.value
}

function openSearch() {
  // VitePress renders a hidden search button inside .VPNavBarSearch
  const btn = document.querySelector('.VPNavBarSearchButton') as HTMLButtonElement
    ?? document.querySelector('[aria-label="Search"]') as HTMLButtonElement
  btn?.click()
}
</script>

<template>
  <div class="fd-shellbar" role="banner">
    <div class="fd-shellbar__group fd-shellbar__group--product">
      <a href="/sap-devs-cli/" class="fd-shellbar__branding" aria-label="Home">
        <span class="fd-shellbar__logo">
          <svg viewBox="0 0 36 36" width="24" height="24" fill="currentColor">
            <rect x="2" y="2" width="14" height="14" rx="2"/>
            <rect x="20" y="2" width="14" height="14" rx="2"/>
            <rect x="2" y="20" width="14" height="14" rx="2"/>
            <rect x="20" y="20" width="14" height="14" rx="2"/>
          </svg>
        </span>
        <span class="fd-shellbar__title">sap-devs</span>
      </a>
      <span class="fd-shellbar__subtitle">Documentation</span>
    </div>
    <div class="fd-shellbar__group fd-shellbar__group--actions">
      <button class="fd-shellbar__button" aria-label="Search" @click="openSearch">
        <i class="sap-icon--search"></i>
      </button>
      <button class="fd-shellbar__button" :aria-label="isDark ? 'Switch to light mode' : 'Switch to dark mode'" @click="toggleDark">
        <i :class="isDark ? 'sap-icon--light-mode' : 'sap-icon--dark-mode'"></i>
      </button>
      <span class="fd-badge fd-badge--success shellbar-version">v{{ version }}</span>
      <a href="https://github.com/SAP-samples/sap-devs-cli" class="fd-shellbar__button" target="_blank" rel="noopener" aria-label="GitHub">
        <i class="sap-icon--source-code"></i>
      </a>
    </div>
  </div>
</template>

<style scoped>
.fd-shellbar {
  display: flex;
  align-items: center;
  height: 44px;
  padding: 0 16px;
  background: var(--sapShell_Background);
  color: var(--sapShell_TextColor);
  position: sticky;
  top: 0;
  z-index: 100;
}
.fd-shellbar__group--product {
  display: flex;
  align-items: center;
  gap: 8px;
}
.fd-shellbar__branding {
  display: flex;
  align-items: center;
  gap: 10px;
  color: inherit;
  text-decoration: none;
}
.fd-shellbar__title {
  font-size: 1rem;
  font-weight: 700;
  letter-spacing: 0.02em;
}
.fd-shellbar__subtitle {
  font-size: 0.75rem;
  opacity: 0.7;
}
.fd-shellbar__group--actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 4px;
}
.fd-shellbar__button {
  background: none;
  border: none;
  color: var(--sapShell_TextColor);
  cursor: pointer;
  padding: 8px;
  border-radius: 4px;
  font-size: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s;
  text-decoration: none;
}
.fd-shellbar__button:hover {
  background: rgba(255, 255, 255, 0.1);
}
.shellbar-version {
  font-size: 0.6875rem;
  margin-right: 4px;
}
</style>
```

- [ ] **Step 2: Update `docs-site/.vitepress/theme/index.ts` to register the shellbar**

```ts
import DefaultTheme from 'vitepress/theme'
import './style.css'
import FioriShellbar from './components/FioriShellbar.vue'
import { h } from 'vue'

export default {
  extends: DefaultTheme,
  Layout() {
    return h(DefaultTheme.Layout, null, {
      'layout-top': () => h(FioriShellbar),
    })
  },
}
```

- [ ] **Step 3: Test the shellbar**

Run: `cd docs-site && npx vitepress dev`
Expected:
- SAP-style shellbar appears at the top with logo, title, subtitle
- Version badge shows the version from package.json
- Dark mode toggle works (sun/moon icon switches, page theme changes)
- Search button opens VitePress's built-in search overlay
- GitHub link opens the repo in a new tab
- VitePress's default navbar is gone (hidden by CSS)
- Shellbar stays sticky on scroll

- [ ] **Step 4: Commit**

```bash
git add docs-site/.vitepress/theme/components/FioriShellbar.vue docs-site/.vitepress/theme/index.ts
git commit -m "feat(docs-site): Fiori shellbar component with search, dark mode, and GitHub link"
```

---

### Task 6: FioriHome Component

**Files:**
- Create: `docs-site/.vitepress/theme/components/FioriHome.vue`
- Modify: `docs-site/.vitepress/theme/index.ts`

The homepage hero banner + feature cards.

- [ ] **Step 1: Create `docs-site/.vitepress/theme/components/FioriHome.vue`**

```vue
<script setup lang="ts">
const features = [
  { icon: 'sap-icon--developer-setting', title: 'Inject', desc: 'Push SAP context into Claude, Cursor, Copilot', link: '/sap-devs-cli/guide/user-guide' },
  { icon: 'sap-icon--connected', title: 'MCP Server', desc: '31 live tools for AI agents', link: '/sap-devs-cli/developer/mcp-server' },
  { icon: 'sap-icon--learning-assistant', title: 'Learn', desc: 'Tutorials, journeys, and missions', link: '/sap-devs-cli/developer/content-guide' },
  { icon: 'sap-icon--stethoscope', title: 'Doctor', desc: 'Check tools and project health', link: '/sap-devs-cli/developer/developer-guide' },
  { icon: 'sap-icon--sys-enter-2', title: 'Content Packs', desc: 'Author and customize SAP knowledge', link: '/sap-devs-cli/developer/content-authoring' },
  { icon: 'sap-icon--documents', title: 'Design Archive', desc: 'Specs and plans behind every feature', link: '/sap-devs-cli/archive/specs/' },
]
</script>

<template>
  <div class="fiori-home">
    <!-- Hero Banner -->
    <section class="hero">
      <div class="hero__inner">
        <div class="hero__logo">
          <svg viewBox="0 0 36 36" width="64" height="64" fill="currentColor">
            <rect x="2" y="2" width="14" height="14" rx="2"/>
            <rect x="20" y="2" width="14" height="14" rx="2"/>
            <rect x="2" y="20" width="14" height="14" rx="2"/>
            <rect x="20" y="20" width="14" height="14" rx="2"/>
          </svg>
        </div>
        <h1 class="hero__title">sap-devs</h1>
        <p class="hero__subtitle">SAP developer knowledge, injected into your AI coding tools</p>
        <div class="hero__actions">
          <a href="/sap-devs-cli/guide/user-guide" class="fd-button fd-button--emphasized">Get Started</a>
          <a href="https://github.com/SAP-samples/sap-devs-cli" class="fd-button fd-button--transparent" target="_blank" rel="noopener">View on GitHub</a>
        </div>
      </div>
    </section>

    <!-- Feature Cards -->
    <section class="features">
      <div class="features__grid">
        <a v-for="f in features" :key="f.title" :href="f.link" class="feature-card fd-card" role="listitem">
          <div class="feature-card__icon"><i :class="f.icon"></i></div>
          <h3 class="feature-card__title">{{ f.title }}</h3>
          <p class="feature-card__desc">{{ f.desc }}</p>
        </a>
      </div>
    </section>
  </div>
</template>

<style scoped>
.fiori-home {
  min-height: calc(100vh - 44px);
}

/* Hero */
.hero {
  background: linear-gradient(135deg, var(--sapShell_Background) 0%, #1a3a5c 100%);
  color: #fff;
  padding: 80px 24px;
  text-align: center;
}
html.dark .hero {
  background: linear-gradient(135deg, #12171c 0%, #1a2530 100%);
}
.hero__inner {
  max-width: 640px;
  margin: 0 auto;
}
.hero__logo {
  margin-bottom: 24px;
  opacity: 0.9;
}
.hero__title {
  font-size: 3rem;
  font-weight: 700;
  margin-bottom: 12px;
  letter-spacing: -0.02em;
}
.hero__subtitle {
  font-size: 1.25rem;
  opacity: 0.85;
  margin-bottom: 32px;
  line-height: 1.6;
}
.hero__actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  flex-wrap: wrap;
}
.hero__actions .fd-button {
  padding: 10px 24px;
  border-radius: 4px;
  font-size: 0.9375rem;
  font-weight: 600;
  text-decoration: none;
  transition: all 0.15s;
  font-family: inherit;
  cursor: pointer;
}
.hero__actions .fd-button--emphasized {
  background: #fff;
  color: var(--sapShell_Background);
  border: 2px solid #fff;
}
.hero__actions .fd-button--emphasized:hover {
  background: #e8ecf0;
}
.hero__actions .fd-button--transparent {
  background: transparent;
  color: #fff;
  border: 2px solid rgba(255, 255, 255, 0.5);
}
.hero__actions .fd-button--transparent:hover {
  border-color: #fff;
}

/* Feature Cards */
.features {
  max-width: 960px;
  margin: 0 auto;
  padding: 48px 24px;
}
.features__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}
@media (max-width: 768px) {
  .features__grid { grid-template-columns: repeat(2, 1fr); }
}
@media (max-width: 480px) {
  .features__grid { grid-template-columns: 1fr; }
}
.feature-card {
  display: block;
  padding: 24px;
  text-decoration: none;
  color: var(--sapTextColor);
  background: var(--sapBaseColor);
  border: 1px solid var(--sapGroup_ContentBorderColor);
  border-radius: 8px;
  transition: box-shadow 0.15s, border-color 0.15s;
}
.feature-card:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  border-color: var(--sapBrandColor);
}
html.dark .feature-card:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
}
.feature-card__icon {
  font-size: 28px;
  color: var(--sapBrandColor);
  margin-bottom: 12px;
}
.feature-card__title {
  font-size: 1rem;
  font-weight: 600;
  margin-bottom: 4px;
}
.feature-card__desc {
  font-size: 0.8125rem;
  color: var(--sapShell_Navigation_TextColor, #556b82);
  line-height: 1.5;
}
</style>
```

- [ ] **Step 2: Update `docs-site/.vitepress/theme/index.ts` to register FioriHome**

```ts
import DefaultTheme from 'vitepress/theme'
import './style.css'
import FioriShellbar from './components/FioriShellbar.vue'
import FioriHome from './components/FioriHome.vue'
import { h } from 'vue'

export default {
  extends: DefaultTheme,
  Layout() {
    return h(DefaultTheme.Layout, null, {
      'layout-top': () => h(FioriShellbar),
    })
  },
  enhanceApp({ app }: { app: any }) {
    app.component('FioriHome', FioriHome)
  },
}
```

- [ ] **Step 3: Test the homepage**

Run: `cd docs-site && npx vitepress dev`
Navigate to: `http://localhost:5173/sap-devs-cli/`
Expected:
- Hero banner with gradient background, logo, title, subtitle
- "Get Started" and "View on GitHub" buttons
- 6 feature cards in a 3×2 grid
- Cards link to correct pages
- Dark mode toggles hero gradient and card backgrounds
- Responsive: cards reflow to 2-col and 1-col on smaller screens

- [ ] **Step 4: Commit**

```bash
git add docs-site/.vitepress/theme/components/FioriHome.vue docs-site/.vitepress/theme/index.ts
git commit -m "feat(docs-site): Fiori homepage with hero banner and feature cards"
```

---

### Task 7: Static Assets

**Files:**
- Create: `docs-site/public/favicon.svg`
- Create: `docs-site/public/og-image.svg`

- [ ] **Step 1: Create a simple favicon**

Generate a minimal SAP-blue favicon. A 32×32 SVG favicon is the simplest approach:

Create `docs-site/public/favicon.svg`:
```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect width="32" height="32" rx="6" fill="#0070f2"/>
  <text x="16" y="22" text-anchor="middle" font-family="Arial" font-weight="700" font-size="16" fill="#fff">S</text>
</svg>
```

- [ ] **Step 2: Create an Open Graph image**

Create `docs-site/public/og-image.svg`:
```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 630">
  <rect width="1200" height="630" fill="#354a5f"/>
  <text x="600" y="280" text-anchor="middle" font-family="Arial" font-weight="700" font-size="72" fill="#fff">sap-devs</text>
  <text x="600" y="360" text-anchor="middle" font-family="Arial" font-size="28" fill="rgba(255,255,255,0.8)">SAP developer knowledge, injected into your AI coding tools</text>
</svg>
```

- [ ] **Step 3: Add head meta to config.mts**

Then add it in `docs-site/.vitepress/config.mts` by adding to the `defineConfig`:
```ts
head: [
  ['link', { rel: 'icon', type: 'image/svg+xml', href: '/sap-devs-cli/favicon.svg' }],
  ['meta', { property: 'og:image', content: 'https://sap-samples.github.io/sap-devs-cli/og-image.svg' }],
],
```

- [ ] **Step 4: Commit**

```bash
git add docs-site/public/favicon.svg docs-site/public/og-image.svg docs-site/.vitepress/config.mts
git commit -m "feat(docs-site): favicon, og-image, and head meta"
```

---

### Task 8: GitHub Actions Workflow

**Files:**
- Create: `.github/workflows/docs.yml`

- [ ] **Step 1: Create `.github/workflows/docs.yml`**

```yaml
name: Deploy docs to GitHub Pages

on:
  push:
    branches: [main]
    paths:
      - 'docs-site/**'
      - 'docs/**'
      - '.github/workflows/docs.yml'

concurrency:
  group: pages
  cancel-in-progress: false

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      pages: write
      id-token: write
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: docs-site/package-lock.json
      - name: Install dependencies
        run: npm ci
        working-directory: docs-site
      - name: Copy content
        run: node scripts/copy-content.js
        working-directory: docs-site
      - name: Build VitePress site
        run: npm run build
        working-directory: docs-site
      - uses: actions/upload-pages-artifact@v3
        with:
          path: docs-site/.vitepress/dist
      - uses: actions/deploy-pages@v4
        id: deployment
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/docs.yml
git commit -m "ci: GitHub Actions workflow for VitePress docs deployment"
```

---

### Task 9: Local Verification and Polish

**Files:**
- Possibly modify: any of the above files

This is the integration testing task. Run the full build locally and verify everything works end-to-end.

- [ ] **Step 1: Full local build**

Run:
```bash
cd docs-site
node scripts/copy-content.js
npx vitepress build
npx vitepress preview
```
Expected: Preview server starts. Site loads at `http://localhost:4173/sap-devs-cli/`.

- [ ] **Step 2: Verify all pages**

Check:
- Homepage hero banner and feature cards render correctly
- All 6 feature card links work
- Shellbar: logo links to homepage, search opens overlay, dark mode toggles, GitHub link works
- Sidebar: Getting Started, Developer, Design Archive sections all have correct links
- Archive: Specs and Plans groups are collapsed, expanding shows all items sorted by date descending
- Click through several spec and plan pages — content renders with Fiori styling
- Developer Guide, Content Authoring, MCP Server pages all render
- Dark mode: toggle and verify all pages look correct in both modes
- Mobile: resize browser to verify responsive sidebar and card grid

- [ ] **Step 3: Fix any issues found**

Address any CSS glitches, broken links, or layout problems discovered during verification.

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "fix(docs-site): polish and fixes from local verification"
```

Only create this commit if there were actual fixes. Skip if verification passed cleanly.

---

### Task 10: Documentation Updates

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/developer/developer-guide.md`

- [ ] **Step 1: Add docs-site section to CLAUDE.md**

Add a `### Documentation Site` section under the Architecture Overview:

```markdown
### Documentation Site

`/docs-site/` is a VitePress site with SAP Fundamental Styles (Fiori) theming, deployed to GitHub Pages. Content is copied at build time from `/docs/` — never edit markdown directly in `docs-site/`. The archive sidebar is auto-generated from `docs/superpowers/specs/` and `docs/superpowers/plans/`.

**Local dev:** `cd docs-site && npm run dev`
**Build:** `cd docs-site && node scripts/copy-content.js && npm run build`
**Deployed at:** `https://sap-samples.github.io/sap-devs-cli/`
```

- [ ] **Step 2: Add docs-site section to developer guide**

Add a `## Documentation Site` section to `docs/developer/developer-guide.md`:

```markdown
## Documentation Site

The project includes a VitePress documentation site at `/docs-site/` that is deployed to GitHub Pages on every push to `main`.

### Local Development

\`\`\`bash
cd docs-site
npm install
npm run dev
\`\`\`

This copies content from `/docs/` and starts a local dev server. Content files in `docs-site/guide/`, `docs-site/developer/`, and `docs-site/archive/` are git-ignored build artifacts — always edit the source files in `/docs/`.

### Theme

The site uses SAP Fundamental Styles with `sap_horizon` (light) and `sap_horizon_dark` (dark) themes. CSS lives in `docs-site/.vitepress/theme/style.css`. Custom components: `FioriShellbar.vue` (top header) and `FioriHome.vue` (homepage).

### Archive Sidebar

Specs and plans in `docs/superpowers/` are auto-discovered by `docs-site/scripts/copy-content.js`, which generates `.vitepress/archive-sidebar.json`. No manual sidebar maintenance needed when adding new specs or plans.
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md docs/developer/developer-guide.md
git commit -m "docs: add documentation site section to CLAUDE.md and developer guide"
```
