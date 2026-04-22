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
// Standard HTML5 elements that should NOT be escaped
const HTML5_ELEMENTS = new Set([
  'a', 'abbr', 'address', 'area', 'article', 'aside', 'audio', 'b', 'base',
  'bdi', 'bdo', 'blockquote', 'body', 'br', 'button', 'canvas', 'caption',
  'cite', 'code', 'col', 'colgroup', 'data', 'datalist', 'dd', 'del',
  'details', 'dfn', 'dialog', 'div', 'dl', 'dt', 'em', 'embed', 'fieldset',
  'figcaption', 'figure', 'footer', 'form', 'h1', 'h2', 'h3', 'h4', 'h5',
  'h6', 'head', 'header', 'hgroup', 'hr', 'html', 'i', 'iframe', 'img',
  'input', 'ins', 'kbd', 'label', 'legend', 'li', 'link', 'main', 'map',
  'mark', 'menu', 'meta', 'meter', 'nav', 'noscript', 'object', 'ol',
  'optgroup', 'option', 'output', 'p', 'picture', 'pre', 'progress', 'q',
  'rp', 'rt', 'ruby', 's', 'samp', 'script', 'search', 'section', 'select',
  'slot', 'small', 'source', 'span', 'strong', 'style', 'sub', 'summary',
  'sup', 'table', 'tbody', 'td', 'template', 'textarea', 'tfoot', 'th',
  'thead', 'time', 'title', 'tr', 'track', 'u', 'ul', 'var', 'video', 'wbr',
])

/**
 * Escape pseudo-HTML tags in archive markdown content to prevent Vue template
 * compilation errors. Archive docs use <placeholder> syntax for documentation
 * (e.g., <uuid>, <id>, <query>, <date>) that are not real HTML/Vue elements.
 *
 * Only escapes tags that are:
 * - Not real HTML5 elements
 * - Not inside fenced code blocks (``` or ~~~)
 * - Including closing tags (</foo>) for the same element names
 */
function escapeArchiveContent(content) {
  const lines = content.split('\n')
  let inFence = false
  const result = []

  for (const line of lines) {
    // Track fenced code block boundaries
    if (/^\s{0,3}(`{3,}|~{3,})/.test(line)) {
      inFence = !inFence
      result.push(line)
      continue
    }

    if (inFence) {
      result.push(line)
      continue
    }

    // Outside fences: escape <tag> and </tag> for non-HTML5 elements
    const escaped = line.replace(/<\/?([a-zA-Z][a-zA-Z0-9_-]*)(\s[^>]*)?\/?>/g, (match, tag) => {
      const lowerTag = tag.toLowerCase()
      if (HTML5_ELEMENTS.has(lowerTag)) return match
      return match.replace(/</g, '&lt;').replace(/>/g, '&gt;')
    })
    result.push(escaped)
  }

  return result.join('\n')
}

function discoverArchive(srcDir, destDir) {
  const srcPath = resolve(root, srcDir)
  const files = readdirSync(srcPath).filter(f => f.endsWith('.md')).sort().reverse()
  const items = []

  for (const file of files) {
    const raw = readFileSync(resolve(srcPath, file), 'utf8')
    const content = escapeArchiveContent(raw)
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

// --- Generate archive index pages ---
function writeArchiveIndex(destDir, title, items) {
  const lines = [`# ${title}\n`]
  for (const item of items) {
    lines.push(`- [${item.text}](${item.link})`)
  }
  const indexPath = resolve(site, destDir, 'index.md')
  writeFileSync(indexPath, lines.join('\n') + '\n', 'utf8')
}

writeArchiveIndex('archive/specs', 'Design Specs', specs)
writeArchiveIndex('archive/plans', 'Implementation Plans', plans)

// --- Write sidebar JSON ---
const sidebarPath = resolve(site, '.vitepress/archive-sidebar.json')
mkdirSync(dirname(sidebarPath), { recursive: true })
writeFileSync(sidebarPath, JSON.stringify({ specs, plans }, null, 2), 'utf8')
console.log(`\nSidebar: .vitepress/archive-sidebar.json (${specs.length} specs, ${plans.length} plans)`)

console.log(`\nDone. ${mappings.length + specs.length + plans.length} files processed.`)
