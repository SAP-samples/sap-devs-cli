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
