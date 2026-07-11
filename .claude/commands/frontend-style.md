# Frontend Style — BubblePulse

You are working on the BubblePulse frontend (Vue 3 + Vite + TypeScript + native CSS). The reference implementation is `frontend/src/views/LoginView.vue`, `DashboardPreview.vue`, and `frontend/src/assets/main.css`. Every component you write or modify must match that quality bar. These rules are non-negotiable.

---

## Design Principles

- **Dark by default.** Page background is always `var(--color-canvas-bg)` (`#0d1117`). Never use white or light-grey backgrounds.
- **Shadows are the only elevation mechanism.** No `border: 1px solid …` on cards, panels, or interactive elements. Use `box-shadow` tokens only.
- **Glassmorphism for floating surfaces.** Cards and panels use `background: var(--glass-bg)` + `backdrop-filter: blur(var(--glass-blur))`. Always pair with a shadow token. Always declare both `-webkit-backdrop-filter` and `backdrop-filter`.
- **One subtle ring instead of a border.** When a panel needs a definition edge, append `0 0 0 1px rgba(255,255,255,0.07)` to an existing `box-shadow`. Never use `border`.
- **Motion is purposeful and light.** Entrance animations use `fadeSlideUp` (pages/cards) or `nodeEntrance` (staggered items). Hover changes `box-shadow` + `transform: translateY(-2px)`. Active snaps back. Duration tokens: `fast` for micro, `base` for most transitions, `slow` for full-page entries.

---

## Token System

**Never hardcode** hex colours, raw `px` sizes, or `ms` durations inside component `<style>` blocks. All values come from `src/assets/main.css`.

### Typography
- `var(--font-sans)` everywhere — no other font families.
- Weight scale: `400` body, `700` labels/buttons, `900` display headings.
- Size scale: `--font-size-{xs|sm|base|lg|xl|2xl|3xl}`.

### Spacing
- `var(--space-{1|2|3|4|6|8|12})` for all `padding`, `gap`, and `margin`. No raw `rem` in components.

### Radii
- `--radius-sm` (inputs, tags), `--radius-md` (buttons, small cards), `--radius-lg` (node bubbles), `--radius-xl` (main cards/panels), `--radius-full` (pills/badges).

### Shadows — choose the closest semantic token

| Token | Use |
|---|---|
| `--shadow-sm` | Subtle depth on small elements |
| `--shadow-md` | Raised panels, dropdowns |
| `--shadow-lg` | Full-screen cards (login card) |
| `--shadow-node` | Graph node default |
| `--shadow-node-hover` | Graph node on hover |
| `--shadow-btn` | Button default |
| `--shadow-btn-hover` | Button on hover |
| `--shadow-panel` | Glassmorphism sidebars |

### Transitions
- `var(--transition-fast)` — 150 ms, micro-interactions (colour, opacity).
- `var(--transition-base)` — 250 ms, most hover effects.
- `var(--transition-slow)` — 400 ms, layout shifts.

### Text colours
- `--color-text-primary` — headings and body copy.
- `--color-text-secondary` — supporting text, ghost button labels.
- `--color-text-muted` — timestamps, captions, legal copy.

### Brand
- `--color-brand` (`#6c63ff`) — focus rings, active accents, logo mark.
- `--color-brand-light` — hover tints.

---

## Component CSS Rules

- **All styles are `<style scoped>`** — no global class leakage.
- **BEM naming: `block__element--modifier`.** The block name matches the Vue component's root CSS class. Example: `.dashboard-preview`, `.dashboard-preview__node`, `.dashboard-preview__node--blocking`.
- **No raw magic values.** If a value appears more than once, extract it as a scoped CSS custom property or reference a token.
- **`inset: 0`** shorthand for `position: absolute` full-bleed children.
- **Focus states are mandatory.** Every interactive element must have:
  ```css
  :focus-visible { outline: 2px solid var(--color-brand); outline-offset: 2px; }
  ```
  Never `outline: none` without a visible replacement.
- **Pointer reset on decoration.** Decorative siblings of interactive content use `pointer-events: none` (see `.login__backdrop`).

---

## Interactive States Pattern

Every clickable element must implement all four states:

```css
.block__element           { /* base state */ }
.block__element:hover     { box-shadow: var(--shadow-*-hover); transform: translateY(-2px); }
.block__element:active    { transform: translateY(0); box-shadow: var(--shadow-*); }
.block__element:focus-visible { outline: 2px solid var(--color-brand); outline-offset: 2px; }
```

Ghost/text buttons (no background) use colour + background transitions:

```css
.block__btn:hover  { color: var(--color-text-primary); background: rgba(255,255,255,0.06); }
.block__btn:active { background: rgba(255,255,255,0.1); }
```

---

## Entrance Animations

Use the global `@keyframes` from `main.css` — they are accessible from scoped blocks.

- **Page or card entrance:** `animation: fadeSlideUp 0.65s ease forwards` with `animation-delay: 100ms` and initial `opacity: 0`.
- **Staggered list items or graph nodes:** `animation: nodeEntrance 0.5s ease forwards` with `animation-delay: calc(var(--node-index, 0) * 80ms)`. Pass `--node-index` via `:style` binding typed as `CSSProperties`.
- **Never skip animations on primary surfaces** — the entrance motion is part of the premium feel.

---

## Layout Patterns

### Full-screen layered view (e.g. login screen)
```
position: relative; min-height: 100vh; overflow: hidden
  .backdrop  → position: absolute; inset: 0; z-index: 0; pointer-events: none
  .veil      → position: absolute; inset: 0; z-index: 1; background + backdrop-filter
  .content   → position: relative; z-index: 2
```

### Canvas + sidebar layout (e.g. graph view)
```css
display: grid;
grid-template-columns: 1fr var(--sidebar-width);
height: 100vh;
```
Mobile (`max-width: 768px`): collapse to single column, canvas `height: 65vh`, sidebar scrolls below.

### Glass sidebar
```css
background: var(--glass-bg);
backdrop-filter: blur(var(--glass-blur));
-webkit-backdrop-filter: blur(var(--glass-blur));
box-shadow: var(--shadow-panel);
overflow-y: auto;
```

---

## Responsive Rules

- Use `min-height: 100vh` not `height: 100vh` on scroll-capable pages.
- Cards/panels: `max-width` + `width: 100%` + `margin: var(--space-8)` so they never touch screen edges on mobile.
- Max reading width: `max-width: 26ch` (short taglines), `max-width: 1200px` (dashboard content areas).
- All images and SVGs: `max-width: 100%`.

---

## Accessibility

- Decorative SVGs: `aria-hidden="true" focusable="false"`.
- Decorative layout elements (backdrops, veils): `aria-hidden="true"`.
- Icon-only buttons must have a visible `aria-label`.
- Colour is never the sole differentiator — status badges also carry text.

---

## Vue Component Conventions

These supplement the rules already in `CLAUDE.md`.

- **Presentational components** (no router/store/API): props-only input, emit-only output. `DashboardPreview.vue` is the canonical example.
- **`CSSProperties`** for dynamic style bindings that carry CSS custom properties:
  ```ts
  import type { CSSProperties } from 'vue'
  function nodeStyle(index: number): CSSProperties {
    return { '--node-index': index } as CSSProperties
  }
  ```
- **SVG percentage coordinates** for responsive overlay graphics: `x1="{node.xPct}%" y1="{node.yPct}%"` — never hardcoded `px`.
- **Blocking/critical-path visual indicators** use `stroke-dasharray` + `filter: drop-shadow` + `animation: dashFlow` on the SVG `<line>`.
- **Inline SVG logos** (e.g. Slack) — use the official multi-colour path, `aria-hidden="true"`, no `fill` attribute on the parent `<svg>` tag.
