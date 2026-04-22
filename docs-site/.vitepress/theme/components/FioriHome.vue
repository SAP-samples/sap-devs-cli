<script setup lang="ts">
import { withBase } from 'vitepress'

const features = [
  { icon: 'sap-icon--action-settings', title: 'Inject', desc: 'Push SAP context into Claude, Cursor, Copilot', link: '/sap-devs-cli/guide/user-guide' },
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
        <div class="hero__text">
          <h1 class="hero__title">sap-devs</h1>
          <p class="hero__subtitle">SAP developer knowledge, injected into your AI coding tools</p>
          <div class="hero__actions">
            <a href="/sap-devs-cli/guide/user-guide" class="fd-button fd-button--emphasized">Get Started</a>
            <a href="https://github.com/SAP-samples/sap-devs-cli" class="fd-button fd-button--transparent" target="_blank" rel="noopener">View on GitHub</a>
          </div>
        </div>
        <div class="hero__logo">
          <img :src="withBase('/mascot.png')" alt="sap-devs mascot" class="hero__mascot" />
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
  padding: 60px 24px;
}
html.dark .hero {
  background: linear-gradient(135deg, #12171c 0%, #1a2530 100%);
}
.hero__inner {
  max-width: 960px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  gap: 48px;
}
.hero__text {
  flex: 1;
}
.hero__logo {
  flex-shrink: 0;
}
.hero__mascot {
  max-height: 280px;
  width: auto;
  filter: drop-shadow(0 4px 24px rgba(0, 0, 0, 0.4));
}
@media (max-width: 768px) {
  .hero__inner {
    flex-direction: column-reverse;
    text-align: center;
  }
  .hero__mascot {
    max-height: 180px;
  }
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
  flex-wrap: wrap;
}
@media (max-width: 768px) {
  .hero__actions {
    justify-content: center;
  }
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
  background: #0070f2;
  color: #fff;
  border: 2px solid #0070f2;
}
.hero__actions .fd-button--emphasized:hover {
  background: #0058b8;
  border-color: #0058b8;
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
