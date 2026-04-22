import { defineConfig } from 'vitepress'
import { readFileSync, existsSync } from 'fs'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'
import type MarkdownIt from 'markdown-it'
import type { Plugin } from 'vite'

const __dirname = dirname(fileURLToPath(import.meta.url))

// Load auto-generated archive sidebar (created by copy-content.js)
const archivePath = resolve(__dirname, 'archive-sidebar.json')
const archive = existsSync(archivePath)
  ? JSON.parse(readFileSync(archivePath, 'utf-8'))
  : { specs: [], plans: [] }

// Read version from package.json for shellbar badge
const pkg = JSON.parse(readFileSync(resolve(__dirname, '../package.json'), 'utf-8'))

// Vite plugin: escape Vue-incompatible content in archive pages.
// VitePress compiles markdown → Vue SFC. This plugin intercepts the SFC
// *after* VitePress but *before* @vitejs/plugin-vue and escapes:
// - {{ }} template interpolation (Go template syntax in archive docs)
// - Unrecognised HTML-like tags (<uuid>, <id>, <query>, etc.) in prose/tables
const archiveEscapePlugin: Plugin = {
  name: 'vitepress-archive-escape',
  // 'post' so we run after VitePress transforms md → Vue SFC
  enforce: 'post',
  transform(code, id) {
    if (!id.includes('/archive/') || !id.endsWith('.md')) return
    // Escape {{ and }} that appear outside code blocks in the rendered SFC
    return code.replace(/\{\{/g, '&#123;&#123;').replace(/\}\}/g, '&#125;&#125;')
  },
}

export default defineConfig({
  title: 'sap-devs',
  description: 'SAP developer knowledge, injected into your AI coding tools',
  base: '/sap-devs-cli/',
  ignoreDeadLinks: true,

  vite: {
    define: {
      __APP_VERSION__: JSON.stringify(pkg.version || '0.0.0'),
    },
    plugins: [archiveEscapePlugin],
  },

  vue: {
    template: {
      compilerOptions: {
        // Treat unknown HTML tags as custom elements instead of erroring.
        // Archive docs may include XML/HTML fragments (e.g. <response>, <item>).
        isCustomElement: (tag) => /^[a-z]+-?[a-z]*$/.test(tag) && !['div', 'span', 'p', 'a', 'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'code', 'pre', 'blockquote', 'strong', 'em', 'img', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'hr', 'br', 'section', 'article', 'nav', 'header', 'footer', 'main', 'aside', 'details', 'summary'].includes(tag),
      },
    },
  },

  markdown: {
    theme: {
      dark: 'vitesse-dark',
      light: 'vitesse-light',
    },
    config: (md: MarkdownIt) => {
      // Escape {{ }} in inline code to prevent Vue template compilation errors.
      // Archive docs contain Go/goreleaser template syntax like {{ .Version }}.
      const originalCode = md.renderer.rules.code_inline!
      md.renderer.rules.code_inline = (tokens, idx, options, env, self) => {
        const token = tokens[idx]
        token.content = token.content.replace(/\{\{/g, '&#123;&#123;').replace(/\}\}/g, '&#125;&#125;')
        return originalCode
          ? originalCode(tokens, idx, options, env, self)
          : self.renderToken(tokens, idx, options)
      }
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
