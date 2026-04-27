# Creating Theme Variants from SAP Fiori Horizon Styles

How to build event/product-branded theme overlays on top of the SAP Fiori Horizon base theme using CSS data-attribute selectors. No changes to the base Fiori styles are needed — each theme is a pure additive CSS layer.

## Architecture

```
Base layer:   SAP Fiori Horizon CSS custom properties (--sap* vars)
                ↓ inherited by all components
Theme layer:  CSS overrides scoped via [data-theme="<name>"] selectors
                ↓ naturally higher specificity than base
Dark layer:   [data-theme="<name>"][data-dark] compound selectors
                ↓ wins over light-mode theme selectors
```

### Why this works

- `[data-theme="joule"] .hero` has higher specificity than `.hero` alone
- `[data-theme="joule"][data-dark] .hero` beats `[data-theme="joule"] .hero`
- No `!important` needed (except for elements that already use it in the base)
- The base Fiori theme is completely unaffected when no `data-theme` attribute is set
- Each theme's CSS is self-contained and can be removed without breaking anything

### Key base Fiori Horizon values being overridden

| SAP Custom Property | Light Fallback | Dark Fallback | Used For |
|---|---|---|---|
| `--sapShellColor` | `#354a5f` | `#223548` | Nav bar, hero background |
| `--sapBrandColor` | `#0070f2` | `#0070f2` | Links, active states, rings |
| `--sapPositiveColor` | `#107e3e` | `#57b520` | Completed states (keep this!) |
| `--sapAccentColor1` | `#e38b16` | `#e38b16` | Prize/earned states |
| `--sapTile_Background` | `#fff` | `#1d2d3e` | Card backgrounds |
| `--sapTextColor` | `#32363a` | `#d1e8ff` | Body text |
| `--sapContent_LabelColor` | `#556b82` | `#89a0b5` | Secondary text |
| `--sapNeutralBorderColor` | `#d9d9d9` | `#556b82` | Borders, inactive states |
| `--sapBackgroundColor` | `#f5f6f7` | `#12171c` | Page background |

## JavaScript Pattern (Vue 3 / VitePress)

### Theme detection via URL parameter

```typescript
import { ref, computed } from 'vue'
import { useData } from 'vitepress'

const { isDark } = useData()
const activeTheme = ref<'joule' | 'sapphire' | null>(null)

// Reactive content that changes per theme
const eventName = computed(() => {
  if (activeTheme.value === 'sapphire') return 'SAP Sapphire 2026'
  return 'SAP TechEd'
})

onMounted(() => {
  const params = new URLSearchParams(window.location.search)
  const theme = params.get('theme')
  if (theme === 'joule' || theme === 'sapphire') {
    activeTheme.value = theme
  }
})
```

### Template root element

```html
<div class="my-component"
  :data-theme="activeTheme ?? undefined"
  :data-dark="isDark ? '' : undefined"
>
```

The `isDark` ref from VitePress is reactive — it watches the `html.dark` class that VitePress toggles. Setting `data-dark` as a boolean attribute (empty string = present) means the CSS selectors activate automatically on toggle.

### Adding a new theme

1. Add the theme name to the union type: `ref<'joule' | 'sapphire' | 'newtheme' | null>(null)`
2. Add detection: `if (theme === 'joule' || theme === 'sapphire' || theme === 'newtheme')`
3. Add content overrides to the computed (event name, etc.)
4. Add CSS sections (see template below)

## CSS Selector Pattern

Every theme override follows this structure:

```css
/* Light mode override */
.my-component[data-theme="<name>"] .<target-class> {
  /* override properties */
}

/* Dark mode override — compound selector wins by specificity */
.my-component[data-theme="<name>"][data-dark] .<target-class> {
  /* dark-specific overrides */
}
```

### What to override per theme

Each theme needs overrides for these UI surface categories:

| Category | CSS Targets | What Changes |
|---|---|---|
| **Hero banner** | `.hero` | Background gradient |
| **Hero glass elements** | `.stat-card`, `.instruction-icon`, `.demo-toggle` | `rgba()` tints matching the gradient |
| **Card hover** | `.track-card:hover`, `.track-card:focus-visible` | Border color, box-shadow color |
| **Progress ring** | `.progress-ring-fill` | `stroke` color |
| **Progress bar** | `.track-card__progress-fill` | `background` (gradient or solid) |
| **Available state** | `.step--available .timeline-bubble`, `.step--available .timeline-content` | `border-color`, `color` |
| **In-progress state** | `.step--in-progress .timeline-bubble`, `.step--in-progress .timeline-content` | `border-color`, `background`, `color` |
| **Tutorial badges** | `.item-type--tutorial` | `background`, `color` |
| **Links** | `.timeline-content__title.clickable` | `color` |
| **Hover glow** | `.timeline-bubble.clickable:hover` | `box-shadow` ring color |
| **Back button** | `.back-button:hover` | `color` |

### What NOT to override

- **Completed state** (`.step--completed`) — keep the green `--sapPositiveColor` across all themes for consistency
- **Earned/prize state** (`.step--earned`) — keep gold `--sapAccentColor1` unless the theme has a distinct "reward" color
- **Locked state** (`.step--locked`) — keep the neutral gray/disabled styling
- **Typography** — keep `--sapFontFamily` and font sizes consistent
- **Spacing/layout** — themes change color, not structure

---

## Joule Theme

**Brand identity:** AI assistant purple gradient
**Source:** SAP Joule brand guidelines
**Activation:** `?theme=joule`

### Color Palette

| Token | Light Mode | Dark Mode | Usage |
|---|---|---|---|
| **Primary** | `#5D36FF` | `#8B6FFF` | Interactive elements, links, rings |
| **Secondary** | `#A100C2` | `#C040E0` | Gradient end, accent |
| **Gradient** | `#5D36FF → #7B42F0 → #A100C2` | `#2A1066 → #4B1A8A → #6B0080` | Hero banner |
| **Tint 8%** | `rgba(93, 54, 255, 0.08)` | `rgba(139, 111, 255, 0.12)` | Badge backgrounds, in-progress bg |
| **Tint 4%** | `rgba(93, 54, 255, 0.04)` | `rgba(139, 111, 255, 0.06)` | Content card backgrounds |
| **Glass** | `rgba(255, 255, 255, 0.14–0.22)` | `rgba(93, 54, 255, 0.12–0.25)` | Hero stat cards, instruction icons |

### Design Principles

- The gradient runs at **135 degrees** (top-left to bottom-right)
- Light mode uses the **full-saturation** brand colors directly
- Dark mode **desaturates and lightens** the primary (`#5D36FF` → `#8B6FFF`) for readability on dark backgrounds
- Dark mode hero uses **deep, muted** versions of the gradient colors (not just darkened)
- Progress bars use the full gradient (`primary → secondary`) as a linear-gradient
- Glass overlay elements on the hero use white `rgba()` in light mode, but switch to primary-tinted `rgba()` in dark mode

### Complete CSS — Light Mode

```css
.app-space[data-theme="joule"] .hero {
  background: linear-gradient(135deg, #5D36FF 0%, #7B42F0 40%, #A100C2 100%);
}

.app-space[data-theme="joule"] .stat-card {
  background: rgba(255, 255, 255, 0.14);
  border-color: rgba(255, 255, 255, 0.22);
}

.app-space[data-theme="joule"] .instruction-icon {
  background: rgba(255, 255, 255, 0.22);
}

.app-space[data-theme="joule"] .demo-toggle {
  border-color: rgba(255, 255, 255, 0.3) !important;
  background: rgba(255, 255, 255, 0.1) !important;
}

.app-space[data-theme="joule"] .demo-toggle:hover {
  background: rgba(255, 255, 255, 0.2) !important;
}

.app-space[data-theme="joule"] .track-card:hover {
  border-color: #5D36FF;
  box-shadow: 0 0.25rem 1rem rgba(93, 54, 255, 0.15);
}

.app-space[data-theme="joule"] .track-card:focus-visible {
  outline-color: #5D36FF;
}

.app-space[data-theme="joule"] .progress-ring-fill {
  stroke: #5D36FF;
}

.app-space[data-theme="joule"] .track-card--complete .progress-ring-fill {
  stroke: var(--sapPositiveColor, #107e3e);
}

.app-space[data-theme="joule"] .track-card__progress-fill {
  background: linear-gradient(90deg, #5D36FF, #A100C2);
}

.app-space[data-theme="joule"] .track-card--complete .track-card__progress-fill {
  background: var(--sapPositiveColor, #107e3e);
}

.app-space[data-theme="joule"] .step--available .timeline-bubble {
  border-color: #5D36FF;
  color: #5D36FF;
}

.app-space[data-theme="joule"] .step--available .timeline-content {
  border-color: #5D36FF;
}

.app-space[data-theme="joule"] .step--in-progress .timeline-bubble {
  border-color: #5D36FF;
  background: rgba(93, 54, 255, 0.08);
  color: #5D36FF;
}

.app-space[data-theme="joule"] .step--in-progress .timeline-content {
  border-color: #5D36FF;
  background: rgba(93, 54, 255, 0.04);
}

.app-space[data-theme="joule"] .item-type--tutorial {
  background: rgba(93, 54, 255, 0.08);
  color: #5D36FF;
}

.app-space[data-theme="joule"] .timeline-content__title.clickable {
  color: #5D36FF;
}

.app-space[data-theme="joule"] .timeline-bubble.clickable:hover {
  box-shadow: 0 0 0 3px rgba(93, 54, 255, 0.1);
}

.app-space[data-theme="joule"] .back-button:hover {
  color: #5D36FF !important;
}
```

### Complete CSS — Dark Mode

```css
.app-space[data-theme="joule"][data-dark] .hero {
  background: linear-gradient(135deg, #2A1066 0%, #4B1A8A 40%, #6B0080 100%);
}

.app-space[data-theme="joule"][data-dark] .stat-card {
  background: rgba(93, 54, 255, 0.12);
  border-color: rgba(93, 54, 255, 0.25);
}

.app-space[data-theme="joule"][data-dark] .instruction-icon {
  background: rgba(93, 54, 255, 0.25);
}

.app-space[data-theme="joule"][data-dark] .track-card:hover {
  border-color: #8B6FFF;
  box-shadow: 0 0.25rem 1rem rgba(93, 54, 255, 0.25);
}

.app-space[data-theme="joule"][data-dark] .track-card:focus-visible {
  outline-color: #8B6FFF;
}

.app-space[data-theme="joule"][data-dark] .progress-ring-fill {
  stroke: #8B6FFF;
}

.app-space[data-theme="joule"][data-dark] .track-card__progress-fill {
  background: linear-gradient(90deg, #8B6FFF, #C040E0);
}

.app-space[data-theme="joule"][data-dark] .track-card--complete .track-card__progress-fill {
  background: var(--sapPositiveColor, #57b520);
}

.app-space[data-theme="joule"][data-dark] .step--available .timeline-bubble {
  border-color: #8B6FFF;
  color: #8B6FFF;
}

.app-space[data-theme="joule"][data-dark] .step--available .timeline-content {
  border-color: #8B6FFF;
}

.app-space[data-theme="joule"][data-dark] .step--in-progress .timeline-bubble {
  border-color: #8B6FFF;
  background: rgba(139, 111, 255, 0.15);
  color: #8B6FFF;
}

.app-space[data-theme="joule"][data-dark] .step--in-progress .timeline-content {
  border-color: #8B6FFF;
  background: rgba(139, 111, 255, 0.06);
}

.app-space[data-theme="joule"][data-dark] .item-type--tutorial {
  background: rgba(139, 111, 255, 0.12);
  color: #8B6FFF;
}

.app-space[data-theme="joule"][data-dark] .timeline-content__title.clickable {
  color: #8B6FFF;
}

.app-space[data-theme="joule"][data-dark] .timeline-bubble.clickable:hover {
  box-shadow: 0 0 0 3px rgba(139, 111, 255, 0.15);
}
```

---

## SAP Sapphire 2026 Theme

**Brand identity:** SAP Sapphire event — "blue to pink" core colorway
**Source:** SAP Sapphire 2026 Creative Guide
**Activation:** `?theme=sapphire`
**Content override:** Event name changes to "SAP Sapphire 2026"

### Color Palette (from Creative Guide)

| Token | Hex | Usage |
|---|---|---|
| **Blue 2** | `#D1EFFF` | Very light accent (hover backgrounds) |
| **Blue 4** | `#89D1FF` | Dark-mode primary interactive color |
| **Blue 6** | `#1B90FF` | Light-mode accent, hero gradient start |
| **Blue 7** | `#0070F2` | Primary brand blue (same as `--sapBrandColor`) |
| **Blue 10** | `#002A86` | Deep navy, hero gradient end, dark-mode hero start |
| **Blue 11** | `#00144A` | Darkest navy, dark-mode hero mid |
| **Pink accent** | `#C850C0` | Light-mode earned/reward, progress gradient end |
| **Pink accent (dark)** | `#E070D0` | Dark-mode earned/reward, lightened for readability |

### Design Principles

- The **"blue to pink" core colorway** is Sapphire's visual signature — apply it to progress bars and achievement states
- The hero gradient uses the **Blue family** only (Blue 6 → Blue 7 → Blue 10) — the pink appears in detail elements, not the hero
- **45 degrees** is a signature angle in the Sapphire brand; the hero uses 135 degrees (which creates the same diagonal)
- Light mode primary (`#0070F2`) is identical to Fiori's `--sapBrandColor` — the differentiation comes from the hero gradient depth and the pink accent
- Dark mode switches to **Blue 4** (`#89D1FF`) as the primary interactive color — light enough to be readable on dark navy
- Prize/earned states use the **pink accent** instead of the default gold `--sapAccentColor1` to reinforce the blue-to-pink colorway

### Deriving dark-mode colors

For dark mode, the rules are:
1. **Hero**: shift the gradient deeper — use Blue 10, Blue 11, and a hint of near-black purple (`#0A0030`)
2. **Interactive elements**: use Blue 4 (`#89D1FF`) — the lightest usable blue from the palette
3. **Tinted backgrounds**: use the dark-mode primary at 10-15% opacity
4. **Pink accent**: lighten from `#C850C0` to `#E070D0` for contrast on dark surfaces
5. **Glass overlays**: tint with Blue 6 at low opacity instead of white

### Complete CSS — Light Mode

```css
.app-space[data-theme="sapphire"] .hero {
  background: linear-gradient(135deg, #1B90FF 0%, #0070F2 45%, #002A86 100%);
}

.app-space[data-theme="sapphire"] .stat-card {
  background: rgba(255, 255, 255, 0.14);
  border-color: rgba(255, 255, 255, 0.22);
}

.app-space[data-theme="sapphire"] .instruction-icon {
  background: rgba(255, 255, 255, 0.22);
}

.app-space[data-theme="sapphire"] .demo-toggle {
  border-color: rgba(255, 255, 255, 0.3) !important;
  background: rgba(255, 255, 255, 0.1) !important;
}

.app-space[data-theme="sapphire"] .demo-toggle:hover {
  background: rgba(255, 255, 255, 0.2) !important;
}

.app-space[data-theme="sapphire"] .track-card:hover {
  border-color: #0070F2;
  box-shadow: 0 0.25rem 1rem rgba(0, 112, 242, 0.12), 0 0 0 1px rgba(200, 80, 192, 0.08);
}

.app-space[data-theme="sapphire"] .track-card:focus-visible {
  outline-color: #0070F2;
}

.app-space[data-theme="sapphire"] .progress-ring-fill {
  stroke: #0070F2;
}

.app-space[data-theme="sapphire"] .track-card--complete .progress-ring-fill {
  stroke: var(--sapPositiveColor, #107e3e);
}

/* Core colorway: blue → pink gradient on progress bars */
.app-space[data-theme="sapphire"] .track-card__progress-fill {
  background: linear-gradient(90deg, #0070F2, #C850C0);
}

.app-space[data-theme="sapphire"] .track-card--complete .track-card__progress-fill {
  background: var(--sapPositiveColor, #107e3e);
}

.app-space[data-theme="sapphire"] .step--available .timeline-bubble {
  border-color: #0070F2;
  color: #0070F2;
}

.app-space[data-theme="sapphire"] .step--available .timeline-content {
  border-color: #0070F2;
}

.app-space[data-theme="sapphire"] .step--in-progress .timeline-bubble {
  border-color: #1B90FF;
  background: rgba(27, 144, 255, 0.08);
  color: #0070F2;
}

.app-space[data-theme="sapphire"] .step--in-progress .timeline-content {
  border-color: #1B90FF;
  background: rgba(27, 144, 255, 0.04);
}

.app-space[data-theme="sapphire"] .item-type--tutorial {
  background: rgba(0, 112, 242, 0.08);
  color: #0070F2;
}

.app-space[data-theme="sapphire"] .timeline-content__title.clickable {
  color: #0070F2;
}

.app-space[data-theme="sapphire"] .timeline-bubble.clickable:hover {
  box-shadow: 0 0 0 3px rgba(0, 112, 242, 0.1);
}

.app-space[data-theme="sapphire"] .back-button:hover {
  color: #0070F2 !important;
}

/* Prize earned uses the pink accent from the core colorway */
.app-space[data-theme="sapphire"] .step--earned .timeline-bubble {
  border-color: #C850C0;
  background: rgba(200, 80, 192, 0.08);
  color: #C850C0;
}

.app-space[data-theme="sapphire"] .step--earned .timeline-content {
  border-left: 3px solid #C850C0;
}
```

### Complete CSS — Dark Mode

```css
.app-space[data-theme="sapphire"][data-dark] .hero {
  background: linear-gradient(135deg, #002A86 0%, #00144A 60%, #0A0030 100%);
}

.app-space[data-theme="sapphire"][data-dark] .stat-card {
  background: rgba(27, 144, 255, 0.1);
  border-color: rgba(27, 144, 255, 0.2);
}

.app-space[data-theme="sapphire"][data-dark] .instruction-icon {
  background: rgba(27, 144, 255, 0.2);
}

.app-space[data-theme="sapphire"][data-dark] .track-card:hover {
  border-color: #89D1FF;
  box-shadow: 0 0.25rem 1rem rgba(0, 42, 134, 0.35), 0 0 0 1px rgba(200, 80, 192, 0.1);
}

.app-space[data-theme="sapphire"][data-dark] .track-card:focus-visible {
  outline-color: #89D1FF;
}

.app-space[data-theme="sapphire"][data-dark] .progress-ring-fill {
  stroke: #89D1FF;
}

/* Blue-to-pink gradient — dark variant uses lighter endpoints */
.app-space[data-theme="sapphire"][data-dark] .track-card__progress-fill {
  background: linear-gradient(90deg, #89D1FF, #E070D0);
}

.app-space[data-theme="sapphire"][data-dark] .track-card--complete .track-card__progress-fill {
  background: var(--sapPositiveColor, #57b520);
}

.app-space[data-theme="sapphire"][data-dark] .step--available .timeline-bubble {
  border-color: #89D1FF;
  color: #89D1FF;
}

.app-space[data-theme="sapphire"][data-dark] .step--available .timeline-content {
  border-color: #89D1FF;
}

.app-space[data-theme="sapphire"][data-dark] .step--in-progress .timeline-bubble {
  border-color: #89D1FF;
  background: rgba(137, 209, 255, 0.12);
  color: #89D1FF;
}

.app-space[data-theme="sapphire"][data-dark] .step--in-progress .timeline-content {
  border-color: #89D1FF;
  background: rgba(137, 209, 255, 0.05);
}

.app-space[data-theme="sapphire"][data-dark] .item-type--tutorial {
  background: rgba(137, 209, 255, 0.1);
  color: #89D1FF;
}

.app-space[data-theme="sapphire"][data-dark] .timeline-content__title.clickable {
  color: #89D1FF;
}

.app-space[data-theme="sapphire"][data-dark] .timeline-bubble.clickable:hover {
  box-shadow: 0 0 0 3px rgba(137, 209, 255, 0.12);
}

/* Prize earned — lightened pink for dark backgrounds */
.app-space[data-theme="sapphire"][data-dark] .step--earned .timeline-bubble {
  border-color: #E070D0;
  background: rgba(224, 112, 208, 0.12);
  color: #E070D0;
}

.app-space[data-theme="sapphire"][data-dark] .step--earned .timeline-content {
  border-left: 3px solid #E070D0;
}
```

---

## Creating a New Theme — Checklist

### 1. Define your palette (minimum 6 tokens)

| Token | Purpose | Light Example | Dark Example |
|---|---|---|---|
| Primary | Interactive elements, links, rings | Brand main color | Lightened 30-40% for dark bg |
| Secondary | Gradient end, accent | Brand secondary | Lightened for dark bg |
| Hero gradient | 3-stop linear-gradient | Vibrant brand colors | Deep/muted versions |
| Tint 8% | Badge backgrounds | `rgba(primary, 0.08)` | `rgba(dark-primary, 0.12)` |
| Tint 4% | Content backgrounds | `rgba(primary, 0.04)` | `rgba(dark-primary, 0.06)` |
| Glass | Hero overlays | `rgba(255,255,255, 0.14)` | `rgba(primary, 0.12)` |

### 2. Derive dark-mode colors

- **Hero**: Darken and desaturate; never use the same gradient as light mode
- **Primary interactive**: Lighten by 30-40% — must pass WCAG AA on `--sapTile_Background` dark
- **Tinted backgrounds**: Increase opacity slightly (0.08 → 0.12) to compensate for lower contrast
- **Glass overlays**: Switch from white-tinted to primary-tinted
- **Gradients**: Use the lightened endpoints

### 3. Add the CSS sections

Copy one of the existing themes and replace colors:

```css
/* ═══════════════════════════════════════════════
   <Theme Name> — Light Mode
   ═══════════════════════════════════════════════ */

/* hero, stat-card, instruction-icon, demo-toggle       */
/* track-card hover/focus, progress-ring, progress-bar   */
/* timeline available, in-progress                       */
/* badges, links, hover glow, back button                */
/* (optionally) earned state if theme has a reward color */

/* ═══════════════════════════════════════════════
   <Theme Name> — Dark Mode
   ═══════════════════════════════════════════════ */

/* same targets, dark-adjusted colors                    */
```

### 4. Wire up JavaScript

```typescript
// Add to union type
const activeTheme = ref<'joule' | 'sapphire' | 'newtheme' | null>(null)

// Add detection
const theme = params.get('theme')
if (theme === 'joule' || theme === 'sapphire' || theme === 'newtheme') {
  activeTheme.value = theme
}

// Add content overrides (optional)
const eventName = computed(() => {
  if (activeTheme.value === 'sapphire') return 'SAP Sapphire 2026'
  if (activeTheme.value === 'newtheme') return 'My Custom Event'
  return 'SAP TechEd'
})
```

### 5. Verify

Test each combination:
- `?theme=<name>` in light mode
- `?theme=<name>` in dark mode
- No theme parameter (base Fiori must be unaffected)
- All UI states: default, in-progress (demo mode click 1), track complete (demo mode click 2)
- Track detail view: locked, available, in-progress, completed, earned
- Responsive: desktop (1280px), tablet (768px), mobile (375px)
