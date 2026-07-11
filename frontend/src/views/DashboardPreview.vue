<script setup lang="ts">
import type { CSSProperties } from 'vue'

type NodeStatus = 'active' | 'blocked' | 'blocking'

interface GraphNode {
  readonly id: string
  label: string
  initials: string
  xPct: number
  yPct: number
  status: NodeStatus
}

interface GraphEdge {
  readonly id: string
  fromId: string
  toId: string
  blocking: boolean
}

interface RenderedEdge {
  readonly id: string
  blocking: boolean
  from: GraphNode
  to: GraphNode
}

interface FeedItem {
  readonly id: string
  icon: string
  text: string
  timestamp: string
}

const nodes: GraphNode[] = [
  { id: 'tech-lead',      label: 'Tech Lead',     initials: 'TL', xPct: 50, yPct: 12, status: 'active' },
  { id: 'product-owner',  label: 'Product Owner',  initials: 'PO', xPct: 15, yPct: 30, status: 'active' },
  { id: 'backend-dev',    label: 'Backend Dev',    initials: 'BD', xPct: 82, yPct: 30, status: 'active' },
  { id: 'ui-designer',    label: 'UI Designer',    initials: 'UD', xPct: 9,  yPct: 68, status: 'active' },
  { id: 'frontend-dev',   label: 'Frontend Dev',   initials: 'FD', xPct: 33, yPct: 55, status: 'blocking' },
  { id: 'api-engineer',   label: 'API Engineer',   initials: 'AE', xPct: 62, yPct: 55, status: 'blocked' },
  { id: 'qa-engineer',    label: 'QA Engineer',    initials: 'QA', xPct: 82, yPct: 73, status: 'active' },
  { id: 'devops',         label: 'DevOps',         initials: 'DO', xPct: 50, yPct: 86, status: 'active' },
]

const edges: GraphEdge[] = [
  { id: 'e1',  fromId: 'tech-lead',     toId: 'product-owner', blocking: false },
  { id: 'e2',  fromId: 'tech-lead',     toId: 'backend-dev',   blocking: false },
  { id: 'e3',  fromId: 'tech-lead',     toId: 'frontend-dev',  blocking: false },
  { id: 'e4',  fromId: 'product-owner', toId: 'ui-designer',   blocking: false },
  { id: 'e5',  fromId: 'product-owner', toId: 'frontend-dev',  blocking: false },
  { id: 'e6',  fromId: 'backend-dev',   toId: 'api-engineer',  blocking: false },
  { id: 'e7',  fromId: 'frontend-dev',  toId: 'api-engineer',  blocking: true  },
  { id: 'e8',  fromId: 'api-engineer',  toId: 'qa-engineer',   blocking: false },
  { id: 'e9',  fromId: 'api-engineer',  toId: 'devops',        blocking: false },
  { id: 'e10', fromId: 'qa-engineer',   toId: 'devops',        blocking: false },
]

function requireNode(id: string): GraphNode {
  const n = nodes.find((node) => node.id === id)
  if (n === undefined) throw new Error(`Node not found: ${id}`)
  return n
}

const renderedEdges: RenderedEdge[] = edges.map((e) => ({
  id: e.id,
  blocking: e.blocking,
  from: requireNode(e.fromId),
  to: requireNode(e.toId),
}))

const feedItems: FeedItem[] = [
  { id: 'f1', icon: '🚀', text: 'Tech Lead pushed sprint goal update',       timestamp: '2m ago' },
  { id: 'f2', icon: '🔴', text: 'API Engineer blocked by Frontend Dev',      timestamp: '5m ago' },
  { id: 'f3', icon: '✅', text: 'Backend Dev completed auth endpoint',       timestamp: '12m ago' },
  { id: 'f4', icon: '💬', text: 'Product Owner added comment on checkout',   timestamp: '18m ago' },
  { id: 'f5', icon: '🧪', text: 'QA Engineer flagged regression in checkout', timestamp: '31m ago' },
  { id: 'f6', icon: '📦', text: 'DevOps deployed staging build v0.4.2',      timestamp: '1h ago' },
]

function nodeStyle(index: number, node: GraphNode): CSSProperties {
  return {
    left: `${node.xPct}%`,
    top: `${node.yPct}%`,
    '--node-index': index,
  }
}
</script>

<template>
  <div class="dashboard-preview">
    <div class="dashboard-preview__canvas">
      <svg class="dashboard-preview__edges" aria-hidden="true">
        <line
          v-for="edge in renderedEdges"
          :key="edge.id"
          :x1="`${edge.from.xPct}%`"
          :y1="`${edge.from.yPct}%`"
          :x2="`${edge.to.xPct}%`"
          :y2="`${edge.to.yPct}%`"
          :class="[
            'dashboard-preview__edge',
            edge.blocking && 'dashboard-preview__edge--blocking',
          ]"
        />
      </svg>

      <div
        v-for="(node, i) in nodes"
        :key="node.id"
        class="dashboard-preview__node"
        :class="`dashboard-preview__node--${node.status}`"
        :style="nodeStyle(i, node)"
      >
        <span class="dashboard-preview__node__initials">{{ node.initials }}</span>
        <span class="dashboard-preview__node__label">{{ node.label }}</span>
        <span class="dashboard-preview__node__badge" :class="`dashboard-preview__node__badge--${node.status}`">
          {{ node.status }}
        </span>
      </div>
    </div>

    <aside class="dashboard-preview__sidebar">
      <h2 class="dashboard-preview__sidebar__title">Activity Feed</h2>
      <ul class="dashboard-preview__feed">
        <li
          v-for="item in feedItems"
          :key="item.id"
          class="dashboard-preview__feed__item"
        >
          <span class="dashboard-preview__feed__icon" aria-hidden="true">{{ item.icon }}</span>
          <div class="dashboard-preview__feed__body">
            <p class="dashboard-preview__feed__text">{{ item.text }}</p>
            <time class="dashboard-preview__feed__timestamp">{{ item.timestamp }}</time>
          </div>
        </li>
      </ul>
    </aside>
  </div>
</template>

<style scoped>
.dashboard-preview {
  display: grid;
  grid-template-columns: 1fr var(--sidebar-width);
  height: 100vh;
  background: var(--color-canvas-bg);
}

/* ── Canvas ── */
.dashboard-preview__canvas {
  position: relative;
  overflow: hidden;
}

.dashboard-preview__edges {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.dashboard-preview__edge {
  stroke: var(--color-edge-normal);
  stroke-width: 1.5;
}

.dashboard-preview__edge--blocking {
  stroke: var(--color-edge-blocking);
  stroke-width: 2;
  stroke-dasharray: 8 4;
  stroke-dashoffset: 0;
  filter: drop-shadow(0 0 5px var(--color-edge-blocking));
  animation: dashFlow 1.5s linear infinite;
}

/* ── Nodes ── */
.dashboard-preview__node {
  position: absolute;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-1);
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-node);
  cursor: default;
  min-width: 90px;
  text-align: center;
  opacity: 0;
  animation: nodeEntrance 0.5s ease forwards;
  animation-delay: calc(var(--node-index, 0) * 80ms);
  transition: box-shadow var(--transition-base), transform var(--transition-base);
}

.dashboard-preview__node:hover {
  box-shadow: var(--shadow-node-hover);
  transform: translate(-50%, calc(-50% - 4px));
}

.dashboard-preview__node--active {
  background: linear-gradient(135deg, var(--color-node-active-from), var(--color-node-active-to));
}

.dashboard-preview__node--blocked {
  background: linear-gradient(135deg, var(--color-node-blocked-from), var(--color-node-blocked-to));
}

.dashboard-preview__node--blocking {
  background: linear-gradient(135deg, var(--color-node-blocking-from), var(--color-node-blocking-to));
  box-shadow: var(--shadow-node), 0 0 20px rgba(232, 67, 147, 0.4);
}

.dashboard-preview__node__initials {
  font-size: var(--font-size-lg);
  font-weight: 900;
  color: rgba(255, 255, 255, 0.95);
  line-height: 1;
}

.dashboard-preview__node__label {
  font-size: var(--font-size-xs);
  font-weight: 700;
  color: rgba(255, 255, 255, 0.85);
  white-space: nowrap;
}

.dashboard-preview__node__badge {
  font-size: 0.65rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 2px 6px;
  border-radius: var(--radius-full);
  background: rgba(0, 0, 0, 0.25);
  color: rgba(255, 255, 255, 0.8);
}

.dashboard-preview__node__badge--blocking {
  background: rgba(0, 0, 0, 0.35);
  color: #fff;
}

.dashboard-preview__node__badge--blocked {
  background: rgba(0, 0, 0, 0.35);
  color: #fff;
}

/* ── Sidebar ── */
.dashboard-preview__sidebar {
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  overflow-y: auto;
  padding: var(--space-6);
  box-shadow: var(--shadow-panel);
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

.dashboard-preview__sidebar__title {
  font-size: var(--font-size-lg);
  font-weight: 700;
  color: var(--color-text-primary);
}

.dashboard-preview__feed {
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.dashboard-preview__feed__item {
  display: flex;
  gap: var(--space-3);
  align-items: flex-start;
}

.dashboard-preview__feed__icon {
  font-size: var(--font-size-base);
  flex-shrink: 0;
  margin-top: 2px;
}

.dashboard-preview__feed__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.dashboard-preview__feed__text {
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
  line-height: 1.4;
}

.dashboard-preview__feed__timestamp {
  font-size: var(--font-size-xs);
  color: var(--color-text-muted);
}

/* ── Mobile ── */
@media (max-width: 768px) {
  .dashboard-preview {
    grid-template-columns: 1fr;
    grid-template-rows: 65vh auto;
    height: auto;
    min-height: 100vh;
  }

  .dashboard-preview__canvas {
    height: 65vh;
  }

  .dashboard-preview__sidebar {
    max-height: 35vh;
    overflow-y: auto;
  }
}
</style>
