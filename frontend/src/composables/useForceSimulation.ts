import {
  forceCenter,
  forceCollide,
  forceLink,
  forceManyBody,
  forceSimulation,
} from 'd3-force'
import type { Simulation, SimulationNodeDatum, SimulationLinkDatum } from 'd3-force'
import { ref, watch, onUnmounted } from 'vue'
import type { Ref } from 'vue'
import type { DashboardResponse } from '@/types'

const SIMILARITY_THRESHOLD = 0.85

/** A simulation node representing either a user bubble or a topic zone label. */
export interface SimNode extends SimulationNodeDatum {
  id: string
  type: 'user' | 'topic'
  label: string
  initials: string
  hasUpdate: boolean
}

/** A simulation link connecting two simulation nodes. */
export interface SimLink extends SimulationLinkDatum<SimNode> {
  type: 'user-topic' | 'topic-similarity'
  strength: number
}

function buildNodes(data: DashboardResponse): SimNode[] {
  const userNodes: SimNode[] = data.users.map((u) => ({
    id: `user:${u.id}`,
    type: 'user',
    label: u.name,
    initials: u.name
      .split(' ')
      .slice(0, 2)
      .map((w) => (w[0] ?? '').toUpperCase())
      .join(''),
    hasUpdate: u.update_text !== null,
  }))

  const topicNodes: SimNode[] = data.topics.map((t) => ({
    id: `topic:${t}`,
    type: 'topic',
    label: t,
    initials: '',
    hasUpdate: false,
  }))

  return [...userNodes, ...topicNodes]
}

function buildLinks(data: DashboardResponse, nodes: SimNode[]): SimLink[] {
  const nodeSet = new Set(nodes.map((n) => n.id))

  const userTopicLinks: SimLink[] = data.users.flatMap((u) =>
    u.topics
      .filter((t) => nodeSet.has(`topic:${t}`))
      .map((t) => ({
        source: `user:${u.id}`,
        target: `topic:${t}`,
        type: 'user-topic' as const,
        strength: 0.4,
      })),
  )

  const simLinks: SimLink[] = []
  const { topics, similarity_matrix } = data
  for (let i = 0; i < topics.length; i++) {
    for (let j = i + 1; j < topics.length; j++) {
      const sim = similarity_matrix[i]?.[j] ?? 0
      if (sim > SIMILARITY_THRESHOLD) {
        simLinks.push({
          source: `topic:${topics[i]}`,
          target: `topic:${topics[j]}`,
          type: 'topic-similarity',
          strength: sim,
        })
      }
    }
  }

  return [...userTopicLinks, ...simLinks]
}

function topologyKey(d: DashboardResponse, w: number, h: number, r: number): string {
  const users = d.users
    .map((u) => `${u.id}:${[...u.topics].sort().join(',')}`)
    .join('|')
  return `${users}##${d.topics.join('|')}##${w}|${h}|${r}`
}

export function useForceSimulation(
  data: Ref<DashboardResponse | null>,
  width: Ref<number>,
  height: Ref<number>,
  userRadius: Ref<number> = ref(44),
) {
  const nodes = ref<SimNode[]>([])
  const links = ref<SimLink[]>([])
  // Incremented on every simulation tick; read in computed properties to re-derive node x/y.
  const tickCount = ref(0)

  let sim: Simulation<SimNode, SimLink> | null = null

  function restart() {
    sim?.stop()
    if (!data.value || width.value === 0) return

    const allNodes = buildNodes(data.value)
    const allLinks = buildLinks(data.value, allNodes)

    // Pre-seed positions at SVG centre so the CSS entrance animation plays from there.
    // Without this, d3 places nodes via a phyllotaxis spiral around (0,0) — the top-left corner.
    const cx = width.value / 2
    const cy = height.value / 2
    allNodes.forEach((n) => {
      n.x = cx + (Math.random() - 0.5) * 10
      n.y = cy + (Math.random() - 0.5) * 10
    })

    nodes.value = allNodes
    links.value = allLinks

    const linkForce = forceLink<SimNode, SimLink>(allLinks)
      .id((d) => d.id)
      .distance((d) => (d.type === 'user-topic' ? 90 : 60))
      .strength((d) => d.strength)

    const chargeForce = forceManyBody<SimNode>().strength((d) =>
      d.type === 'topic' ? -280 : -120,
    )

    const collideForce = forceCollide<SimNode>()
      .radius((d) => (d.type === 'topic' ? 54 : userRadius.value + 10))
      .strength(0.7)

    sim = forceSimulation<SimNode>(allNodes)
      .force('link', linkForce)
      .force('charge', chargeForce)
      .force('center', forceCenter(width.value / 2, height.value / 2).strength(0.05))
      .force('collide', collideForce)
      .alphaDecay(0.02)
      .on('tick', () => {
        tickCount.value++
      })

    sim.restart()
  }

  let lastKey = ''

  watch(
    [data, width, height, userRadius],
    ([newData, newW, newH, newR]) => {
      if (!newData) return
      const key = topologyKey(newData, newW as number, newH as number, newR as number)

      if (key === lastKey && nodes.value.length > 0) {
        // Topology unchanged — only hasUpdate may differ; patch nodes in-place.
        const updateMap = new Map(
          newData.users.map((u) => [`user:${u.id}`, u.update_text !== null])
        )
        nodes.value.forEach((n) => {
          if (n.type === 'user') {
            const next = updateMap.get(n.id)
            if (next !== undefined) n.hasUpdate = next
          }
        })
        return
      }

      lastKey = key
      restart()
    },
    { immediate: true },
  )

  onUnmounted(() => sim?.stop())

  return { nodes, links, tickCount }
}
