<script setup lang="ts">
import { useData, withBase } from 'vitepress'

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
      <a :href="withBase('/')" class="fd-shellbar__branding" aria-label="Home">
        <span class="fd-shellbar__logo">
          <img :src="withBase('/favicon.png')" alt="sap-devs" width="24" height="24" />
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
