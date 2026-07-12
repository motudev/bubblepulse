<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { CSSProperties } from 'vue'
import { api } from '@/services/api'
import type { DashboardEntry } from '@/types'

const GOLDEN_ANGLE = 2.39996 // ≈ 137.5° golden angle in radians
const BASE_RADIUS = 18       // % of container dimension per sqrt-unit

const entries = ref<DashboardEntry[]>([])
const hoveredId = ref<number | null>(null)

onMounted(async () => {
  entries.value = await api.getDashboard()
})

function clamp(v: number, lo: number, hi: number): number {
  return Math.min(Math.max(v, lo), hi)
}

const nodeStyles = computed((): CSSProperties[] =>
  entries.value.map((_, i) => {
    const angle = i * GOLDEN_ANGLE
    const radius = i === 0 ? 0 : Math.sqrt(i) * BASE_RADIUS
    const floatDelay = (i * 0.08) + 0.6 + (i % 5) * 0.55
    return {
      '--node-index': i,
      '--float-delay': `${floatDelay}s`,
      left: `${clamp(50 + radius * Math.cos(angle), 8, 92)}%`,
      top: `${clamp(50 + radius * Math.sin(angle), 8, 92)}%`,
    } as CSSProperties
  })
)

function initials(name: string): string {
  return name
    .split(' ')
    .slice(0, 2)
    .map(w => w[0]?.toUpperCase() ?? '')
    .join('')
}
</script>

<template>
  <div class="bubble-map" role="region" aria-label="Team pulse board">
    <div
      v-for="(entry, i) in entries"
      :key="entry.id"
      class="bubble-map__node"
      :style="nodeStyles[i]"
      tabindex="0"
      :aria-label="`${entry.name}${entry.update_text ? ': ' + entry.update_text : ': no update yet'}`"
      @mouseenter="hoveredId = entry.id"
      @mouseleave="hoveredId = null"
      @focus="hoveredId = entry.id"
      @blur="hoveredId = null"
    >
      <span
        class="bubble-map__avatar"
        :class="{ 'bubble-map__avatar--active': entry.update_text !== null }"
      >{{ initials(entry.name) }}</span>

      <span class="bubble-map__name">{{ entry.name }}</span>

      <div
        class="bubble-map__tooltip"
        :class="{ 'bubble-map__tooltip--visible': hoveredId === entry.id }"
        aria-hidden="true"
      >
        <span class="bubble-map__tooltip-name">{{ entry.name }}</span>
        <span class="bubble-map__tooltip-text">{{ entry.update_text ?? 'No update yet' }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bubble-map {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: var(--color-canvas-bg);
}

/* ── Node wrapper ──────────────────────────────────────────────────── */
.bubble-map__node {
  position: absolute;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  cursor: pointer;
  opacity: 0;
  outline: none; /* handled per child below */
  animation-name: nodeEntrance, bubbleFloat;
  animation-duration: 0.5s, 6.5s;
  animation-timing-function: ease, ease-in-out;
  animation-delay: calc(var(--node-index, 0) * 80ms), var(--float-delay, 0.6s);
  animation-fill-mode: forwards, none;
  animation-iteration-count: 1, infinite;
}

/* ── Avatar circle ─────────────────────────────────────────────────── */
.bubble-map__avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 80px;
  height: 80px;
  border-radius: var(--radius-full);
  background: linear-gradient(135deg, rgba(26, 26, 46, 0.38), rgba(13, 17, 23, 0.22));
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  box-shadow: var(--shadow-node), 0 0 0 1px rgba(255, 255, 255, 0.12), inset 0 1px 0 rgba(255, 255, 255, 0.1);
  font-family: var(--font-sans);
  font-size: var(--font-size-lg);
  font-weight: 700;
  color: var(--color-text-muted);
  transition:
    box-shadow var(--transition-base),
    transform var(--transition-base);
  pointer-events: none; /* events handled by wrapper */
}

.bubble-map__avatar--active {
  background: linear-gradient(135deg, rgba(0, 184, 148, 0.38), rgba(0, 206, 201, 0.28));
  color: var(--color-text-primary);
}

.bubble-map__node:hover .bubble-map__avatar,
.bubble-map__node:focus-visible .bubble-map__avatar {
  box-shadow: var(--shadow-node-hover);
  transform: translateY(-4px);
}

.bubble-map__node:active .bubble-map__avatar {
  box-shadow: var(--shadow-node);
  transform: translateY(0);
}

.bubble-map__node:focus-visible .bubble-map__avatar {
  outline: 2px solid var(--color-brand);
  outline-offset: 2px;
}

/* ── Name label ────────────────────────────────────────────────────── */
.bubble-map__name {
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
  white-space: nowrap;
  pointer-events: none;
}

/* ── Tooltip ───────────────────────────────────────────────────────── */
.bubble-map__tooltip {
  position: absolute;
  bottom: calc(100% + var(--space-2));
  left: 50%;
  transform: translateX(-50%) translateY(4px);
  opacity: 0;
  pointer-events: none;
  transition:
    opacity var(--transition-fast),
    transform var(--transition-fast);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  box-shadow: var(--shadow-md), 0 0 0 1px rgba(255, 255, 255, 0.07);
  border-radius: var(--radius-xl);
  padding: var(--space-3) var(--space-4);
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  min-width: 140px;
  max-width: 26ch;
  word-break: break-word;
  z-index: 10;
}

.bubble-map__tooltip--visible {
  opacity: 1;
  transform: translateX(-50%) translateY(0);
}

.bubble-map__tooltip-name {
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  font-weight: 700;
  color: var(--color-text-primary);
}

.bubble-map__tooltip-text {
  font-family: var(--font-sans);
  font-size: var(--font-size-xs);
  color: var(--color-text-secondary);
}
</style>
