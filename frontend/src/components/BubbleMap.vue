<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import type { CSSProperties } from 'vue'
import { api } from '@/services/api'
import { useScopeStore } from '@/stores/scope'
import { useForceSimulation } from '@/composables/useForceSimulation'
import type { SimNode } from '@/composables/useForceSimulation'
import type { DashboardResponse, UserEntry } from '@/types'
import StarField from '@/components/StarField.vue'

const svgRef = ref<SVGSVGElement | null>(null)
const svgWidth = ref(800)
const svgHeight = ref(600)
const dashboardData = ref<DashboardResponse | null>(null)
const hoveredId = ref<string | null>(null)

const { nodes, links, tickCount } = useForceSimulation(dashboardData, svgWidth, svgHeight)

const nodePositions = computed(() => {
  void tickCount.value
  return nodes.value.map((n) => ({
    ...n,
    cx: n.x ?? svgWidth.value / 2,
    cy: n.y ?? svgHeight.value / 2,
  }))
})

const linkCoords = computed(() => {
  void tickCount.value
  return links.value.map((l, i) => {
    const s = l.source as SimNode
    const t = l.target as SimNode
    return {
      key: i,
      type: l.type,
      x1: typeof s === 'object' ? (s.x ?? 0) : 0,
      y1: typeof s === 'object' ? (s.y ?? 0) : 0,
      x2: typeof t === 'object' ? (t.x ?? 0) : 0,
      y2: typeof t === 'object' ? (t.y ?? 0) : 0,
    }
  })
})

const topicNodes = computed(() => nodePositions.value.filter((n) => n.type === 'topic'))
const userNodes = computed(() => nodePositions.value.filter((n) => n.type === 'user'))
const hoveredNode = computed(() => nodePositions.value.find((n) => n.id === hoveredId.value))

function tooltipStyle(node: { cx: number; cy: number }): CSSProperties {
  return { left: `${node.cx}px`, top: `${node.cy - 68}px` }
}

function topicPillWidth(label: string): number {
  return label.length * 7 + 24
}

function updateEntry(id: string): UserEntry | undefined {
  const numId = parseInt(id.replace('user:', ''), 10)
  return dashboardData.value?.users.find((u) => u.id === numId)
}

// Vary float timing per bubble so they never drift in unison.
const FLOAT_DURATIONS = [7.4, 9.1, 8.3, 10.6, 7.8, 9.8]
const FLOAT_DELAYS    = [0, -2.3, -4.7, -1.6, -3.2, -5.4]
const TOPIC_DURATIONS = [11.2, 9.6, 12.4, 10.0]
const TOPIC_DELAYS    = [-1.0, -4.2, -6.1, -2.8]

function bubbleStyle(index: number): CSSProperties {
  return {
    '--float-dur':  `${FLOAT_DURATIONS[index % FLOAT_DURATIONS.length]}s`,
    '--float-delay': `${FLOAT_DELAYS[index % FLOAT_DELAYS.length]}s`,
    '--node-index': index,
  } as CSSProperties
}

function topicStyle(index: number): CSSProperties {
  return {
    '--float-dur':  `${TOPIC_DURATIONS[index % TOPIC_DURATIONS.length]}s`,
    '--float-delay': `${TOPIC_DELAYS[index % TOPIC_DELAYS.length]}s`,
    '--node-index': index,
  } as CSSProperties
}

const scopeStore = useScopeStore()

// Refetch whenever the org/team scope toggle changes.
watch(
  () => scopeStore.activeTeamId,
  async (teamId) => {
    dashboardData.value = await api.getDashboard(teamId)
  }
)

let ro: ResizeObserver | null = null

onMounted(async () => {
  dashboardData.value = await api.getDashboard(scopeStore.activeTeamId)

  ro = new ResizeObserver((entries) => {
    const r = entries[0]?.contentRect
    if (r) {
      svgWidth.value = r.width
      svgHeight.value = r.height
    }
  })
  if (svgRef.value) ro.observe(svgRef.value)
})

onUnmounted(() => ro?.disconnect())
</script>

<template>
  <div class="bubble-map" role="region" aria-label="Team pulse board">
    <StarField class="bubble-map__stars" />
    <svg ref="svgRef" class="bubble-map__canvas" aria-hidden="true">
      <defs>
        <!-- Glass sheen: top-left highlight that simulates a translucent sphere -->
        <radialGradient id="bubble-sheen" cx="38%" cy="32%" r="62%" gradientUnits="objectBoundingBox">
          <stop offset="0%"   stop-color="#ffffff" stop-opacity="0.18"/>
          <stop offset="100%" stop-color="#ffffff" stop-opacity="0"/>
        </radialGradient>
        <!-- Soft green centre overlay for users who submitted today -->
        <radialGradient id="bubble-active" cx="50%" cy="50%" r="55%" gradientUnits="objectBoundingBox">
          <stop offset="0%"   stop-color="#00b894" stop-opacity="0.35"/>
          <stop offset="100%" stop-color="#00b894" stop-opacity="0"/>
        </radialGradient>
      </defs>

      <!-- Edge layer -->
      <g class="bubble-map__edges">
        <line
          v-for="link in linkCoords"
          :key="link.key"
          :x1="link.x1" :y1="link.y1"
          :x2="link.x2" :y2="link.y2"
          class="bubble-map__edge"
          :class="`bubble-map__edge--${link.type}`"
        />
      </g>

      <!-- Topic nodes: floating pill labels -->
      <g class="bubble-map__topics">
        <g
          v-for="(node, i) in topicNodes"
          :key="node.id"
          class="bubble-map__topic-node"
          :transform="`translate(${node.cx},${node.cy})`"
        >
          <g class="bubble-map__topic-bubble" :style="topicStyle(i)">
            <rect
              :width="topicPillWidth(node.label)"
              height="28"
              :x="-topicPillWidth(node.label) / 2"
              y="-14"
              rx="14"
              class="bubble-map__topic-pill"
            />
            <text class="bubble-map__topic-label" text-anchor="middle" dominant-baseline="central">
              {{ node.label }}
            </text>
          </g>
        </g>
      </g>

      <!-- User nodes: glass bubbles with initials -->
      <g class="bubble-map__users">
        <g
          v-for="(node, i) in userNodes"
          :key="node.id"
          class="bubble-map__user-node"
          :class="{ 'bubble-map__user-node--active': node.hasUpdate }"
          :transform="`translate(${node.cx},${node.cy})`"
          tabindex="0"
          :aria-label="`${node.label}${updateEntry(node.id)?.update_text ? ': ' + updateEntry(node.id)?.update_text : ': no update yet'}`"
          @mouseenter="hoveredId = node.id"
          @mouseleave="hoveredId = null"
          @focus="hoveredId = node.id"
          @blur="hoveredId = null"
        >
          <g class="bubble-map__user-bubble" :style="bubbleStyle(i)">
            <!-- Base glass circle -->
            <circle r="44" class="bubble-map__avatar-circle" />
            <!-- Active tint overlay (only for users with today's update) -->
            <circle
              v-if="node.hasUpdate"
              r="44"
              fill="url(#bubble-active)"
              class="bubble-map__avatar-active-fill"
            />
            <!-- Glass sheen highlight -->
            <circle r="44" fill="url(#bubble-sheen)" class="bubble-map__avatar-sheen" />
            <text
              class="bubble-map__avatar-initials"
              text-anchor="middle"
              dominant-baseline="central"
            >{{ node.initials }}</text>
            <text class="bubble-map__node-name" text-anchor="middle" y="58">
              {{ node.label }}
            </text>
          </g>
        </g>
      </g>
    </svg>

    <!-- Tooltip overlaid on canvas via absolute positioning -->
    <Transition name="tooltip">
      <div
        v-if="hoveredNode"
        class="bubble-map__tooltip"
        :style="tooltipStyle(hoveredNode)"
        aria-hidden="true"
      >
        <span class="bubble-map__tooltip-name">{{ hoveredNode.label }}</span>
        <span class="bubble-map__tooltip-text">
          {{ updateEntry(hoveredNode.id)?.update_text ?? 'No update yet' }}
        </span>
      </div>
    </Transition>
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

/* ── SVG canvas ─────────────────────────────────────────────────────── */
.bubble-map__canvas {
  display: block;
  width: 100%;
  height: 100%;
}

/* ── Star field canvas ──────────────────────────────────────────────── */
.bubble-map__stars {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

/* ── Edges ────────────────────────────────────────────────────────── */
.bubble-map__edge {
  stroke: var(--color-edge-normal);
  stroke-width: 1;
  opacity: 0.35;
  fill: none;
}

.bubble-map__edge--topic-similarity {
  stroke: var(--color-brand-light);
  stroke-width: 1;
  stroke-dasharray: 5 4;
  opacity: 0.45;
  fill: none;
  animation: dashFlow 1.8s linear infinite;
}

/* ── Topic nodes ───────────────────────────────────────���────────────── */
.bubble-map__topic-node {
  cursor: default;
}

.bubble-map__topic-bubble {
  animation:
    nodeEntranceSvg 0.5s ease calc(var(--node-index, 0) * 60ms) both,
    bubbleFloatSvg  var(--float-dur, 10s) var(--float-delay, 0s) ease-in-out infinite;
  transform-origin: center;
}

.bubble-map__topic-pill {
  fill: rgba(108, 99, 255, 0.12);
  stroke: rgba(108, 99, 255, 0.45);
  stroke-width: 1;
  filter: drop-shadow(0 2px 8px rgba(108, 99, 255, 0.2));
}

.bubble-map__topic-label {
  font-family: var(--font-sans);
  font-size: var(--font-size-xs);
  fill: var(--color-brand-light);
  pointer-events: none;
}

/* ── User bubble wrapper — carries float + entrance ─────────────────── */
.bubble-map__user-bubble {
  animation:
    nodeEntranceSvg 0.6s ease calc(var(--node-index, 0) * 80ms) both,
    bubbleFloatSvg  var(--float-dur, 8s) var(--float-delay, 0s) ease-in-out infinite;
  transform-origin: center;
}

/* Pause float on hover so the lift on the circle feels deliberate */
.bubble-map__user-node:hover .bubble-map__user-bubble,
.bubble-map__user-node:focus-visible .bubble-map__user-bubble {
  animation-play-state: running, paused;
}

/* ── User nodes ─────���───────────────────────────────────────────────── */
.bubble-map__user-node {
  cursor: pointer;
  outline: none;
}

.bubble-map__user-node:focus-visible .bubble-map__avatar-circle {
  stroke: var(--color-brand);
  stroke-width: 2.5;
}

/* ── Avatar circle: main glass body ─────────────────────────────────── */
.bubble-map__avatar-circle {
  fill: rgba(16, 18, 38, 0.52);
  /* stroke: rgba(255, 255, 255, 0.20); */
  /* stroke-width: 1.5; */
  filter:
    drop-shadow(0 6px 20px rgba(0, 0, 0, 0.55))
    drop-shadow(0 0 0 rgba(108, 99, 255, 0));
  transition: transform var(--transition-base), filter var(--transition-base);
}

.bubble-map__user-node--active .bubble-map__avatar-circle {
  stroke: rgba(0, 184, 148, 0.45);
  filter:
    drop-shadow(0 6px 20px rgba(0, 0, 0, 0.55))
    drop-shadow(0 0 18px rgba(0, 184, 148, 0.3));
}

.bubble-map__user-node:hover .bubble-map__avatar-circle,
.bubble-map__user-node:focus-visible .bubble-map__avatar-circle {
  transform: translateY(-5px);
  filter:
    drop-shadow(0 12px 32px rgba(0, 0, 0, 0.7))
    drop-shadow(0 0 16px rgba(108, 99, 255, 0.35));
}

.bubble-map__user-node:active .bubble-map__avatar-circle {
  transform: translateY(0);
  filter: drop-shadow(0 4px 16px rgba(0, 0, 0, 0.5));
}

/* ��─ Sheen and active-fill overlays ───────────────────────────────���─── */
.bubble-map__avatar-sheen,
.bubble-map__avatar-active-fill {
  pointer-events: none;
}

/* ── Avatar text ────────────────────────────────────────────────────── */
.bubble-map__avatar-initials {
  font-family: var(--font-sans);
  font-size: var(--font-size-lg);
  font-weight: 700;
  fill: var(--color-text-primary);
  pointer-events: none;
}

.bubble-map__node-name {
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  fill: var(--color-text-secondary);
  pointer-events: none;
}

/* ── Tooltip ──────────────��─────────────────────────────────────────── */
.bubble-map__tooltip {
  position: absolute;
  transform: translateX(-50%) translateY(-100%);
  pointer-events: none;
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

/* ── Tooltip Vue transition ───────────��─────────────────────────────── */
.tooltip-enter-active,
.tooltip-leave-active {
  transition: opacity var(--transition-fast), transform var(--transition-fast);
}

.tooltip-enter-from,
.tooltip-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(calc(-100% + 6px));
}
</style>
