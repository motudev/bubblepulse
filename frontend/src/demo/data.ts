import type { User, DashboardResponse, UserEntry } from '@/types'

// ── Demo user ────────────────────────────────────────────────────────────────

export const DEMO_USER: User = {
  id: 0,
  email: 'demo@bubblepulse.dev',
  name: 'Demo Admin',
  role: 'ADMIN',
  team_id: 'team-eng',
  org: { id: 'org-demo', name: 'Acme Corp' },
}

// ── Topics ───────────────────────────────────────────────────────────────────

export const DEMO_TOPICS: string[] = [
  'API design',        // 0
  'Infrastructure',    // 1
  'Authentication',    // 2
  'Performance',       // 3
  'CI/CD',             // 4
  'Security',          // 5
  'Roadmap',           // 6
  'Q4 planning',       // 7
  'Analytics',         // 8
  'User research',     // 9
  'A/B testing',       // 10
  'Metrics',           // 11
  'Mobile app',        // 12
  'User flows',        // 13
  'Prototyping',       // 14
  'Design system',     // 15
  'Accessibility',     // 16
  'Dashboard redesign',// 17
  'Brand',             // 18
]

// ── Similarity matrix (19×19, symmetric) ─────────────────────────────────────
// Pairs above the 0.85 threshold that form dashed similarity edges in the bubble map.

function buildMatrix(): number[][] {
  const n = DEMO_TOPICS.length
  const m: number[][] = Array.from({ length: n }, () => Array(n).fill(0))

  const pairs: [number, number, number][] = [
    [0,  1,  0.86], // API design      ↔ Infrastructure
    [2,  5,  0.89], // Authentication  ↔ Security
    [3,  4,  0.86], // Performance     ↔ CI/CD
    [6,  7,  0.90], // Roadmap         ↔ Q4 planning
    [8,  11, 0.91], // Analytics       ↔ Metrics
    [9,  10, 0.86], // User research   ↔ A/B testing
    [9,  13, 0.87], // User research   ↔ User flows
    [12, 14, 0.86], // Mobile app      ↔ Prototyping
    [13, 17, 0.85], // User flows      ↔ Dashboard redesign
    [15, 16, 0.88], // Design system   ↔ Accessibility
  ]

  for (const [i, j, score] of pairs) {
    m[i][j] = score
    m[j][i] = score
  }

  return m
}

export const DEMO_SIMILARITY_MATRIX: number[][] = buildMatrix()

// ── Users ────────────────────────────────────────────────────────────────────

interface DemoUserEntry extends UserEntry {
  _teamId: string
}

const DEMO_USERS: DemoUserEntry[] = [
  // Engineering × 6
  {
    id: 1,
    name: 'Alex Chen',
    email: 'alex.chen@acme.dev',
    update_text: 'Finished API v2 migration for 3 endpoints, one more left. Blocked on security team sign-off.',
    update_at: new Date(Date.now() - 2 * 3600_000).toISOString(),
    topics: ['API design', 'Infrastructure', 'Authentication'],
    _teamId: 'team-eng',
  },
  {
    id: 2,
    name: 'Maria Santos',
    email: 'maria.santos@acme.dev',
    update_text: 'Rolled out the new caching layer to staging — 40% latency improvement. Deploying to prod tomorrow.',
    update_at: new Date(Date.now() - 1 * 3600_000).toISOString(),
    topics: ['Infrastructure', 'Performance', 'CI/CD'],
    _teamId: 'team-eng',
  },
  {
    id: 3,
    name: 'James Kim',
    email: 'james.kim@acme.dev',
    update_text: null,
    update_at: null,
    topics: ['Security', 'Authentication', 'API design'],
    _teamId: 'team-eng',
  },
  {
    id: 4,
    name: 'Sophie Taylor',
    email: 'sophie.taylor@acme.dev',
    update_text: 'Automated the release pipeline. Build times down from 12 min to 4 min.',
    update_at: new Date(Date.now() - 3 * 3600_000).toISOString(),
    topics: ['CI/CD', 'Performance'],
    _teamId: 'team-eng',
  },
  {
    id: 5,
    name: 'Noah Williams',
    email: 'noah.williams@acme.dev',
    update_text: 'Profiling shows memory leak in the WebSocket handler. Working on a fix.',
    update_at: new Date(Date.now() - 30 * 60_000).toISOString(),
    topics: ['Performance', 'Mobile app'],
    _teamId: 'team-eng',
  },
  {
    id: 6,
    name: 'Priya Patel',
    email: 'priya.patel@acme.dev',
    update_text: null,
    update_at: null,
    topics: ['Infrastructure', 'Security'],
    _teamId: 'team-eng',
  },

  // Product × 7
  {
    id: 7,
    name: 'Lucas Johnson',
    email: 'lucas.johnson@acme.dev',
    update_text: 'Finalised Q4 roadmap with stakeholders. Dashboard and mobile are the top two priorities.',
    update_at: new Date(Date.now() - 4 * 3600_000).toISOString(),
    topics: ['Roadmap', 'Q4 planning', 'Analytics'],
    _teamId: 'team-product',
  },
  {
    id: 8,
    name: 'Emma Brown',
    email: 'emma.brown@acme.dev',
    update_text: 'Ran 8 user interviews this week. Navigation is consistently confusing for new users.',
    update_at: new Date(Date.now() - 2.5 * 3600_000).toISOString(),
    topics: ['User research', 'A/B testing', 'User flows'],
    _teamId: 'team-product',
  },
  {
    id: 9,
    name: 'Daniel Lee',
    email: 'daniel.lee@acme.dev',
    update_text: null,
    update_at: null,
    topics: ['Analytics', 'Metrics', 'Dashboard redesign'],
    _teamId: 'team-product',
  },
  {
    id: 10,
    name: 'Olivia Wilson',
    email: 'olivia.wilson@acme.dev',
    update_text: 'New onboarding A/B test is live. Too early for significance but early trends look positive.',
    update_at: new Date(Date.now() - 45 * 60_000).toISOString(),
    topics: ['A/B testing', 'Metrics', 'Mobile app'],
    _teamId: 'team-product',
  },
  {
    id: 11,
    name: 'Ethan Davis',
    email: 'ethan.davis@acme.dev',
    update_text: null,
    update_at: null,
    topics: ['Q4 planning', 'Roadmap'],
    _teamId: 'team-product',
  },
  {
    id: 12,
    name: 'Ava Martinez',
    email: 'ava.martinez@acme.dev',
    update_text: 'Blocked on design tokens from the design team — mobile prototypes are on hold.',
    update_at: new Date(Date.now() - 1.5 * 3600_000).toISOString(),
    topics: ['Mobile app', 'User flows', 'User research'],
    _teamId: 'team-product',
  },
  {
    id: 13,
    name: 'Mason Thompson',
    email: 'mason.thompson@acme.dev',
    update_text: 'Wrote specs for dashboard v2. Waiting on design mockups to unblock engineering.',
    update_at: new Date(Date.now() - 5 * 3600_000).toISOString(),
    topics: ['Dashboard redesign', 'Analytics'],
    _teamId: 'team-product',
  },

  // Design × 5
  {
    id: 14,
    name: 'Isabella Garcia',
    email: 'isabella.garcia@acme.dev',
    update_text: 'Released design token v2 — spacing, colours, typography all unified. Engineering can unblock.',
    update_at: new Date(Date.now() - 20 * 60_000).toISOString(),
    topics: ['Design system', 'Accessibility', 'Dashboard redesign'],
    _teamId: 'team-design',
  },
  {
    id: 15,
    name: 'Liam Anderson',
    email: 'liam.anderson@acme.dev',
    update_text: 'Completed WCAG 2.1 AA audit. Found 12 issues, 3 critical. Filing tickets today.',
    update_at: new Date(Date.now() - 3.5 * 3600_000).toISOString(),
    topics: ['Accessibility', 'User flows'],
    _teamId: 'team-design',
  },
  {
    id: 16,
    name: 'Mia Jackson',
    email: 'mia.jackson@acme.dev',
    update_text: null,
    update_at: null,
    topics: ['Prototyping', 'Mobile app', 'User flows'],
    _teamId: 'team-design',
  },
  {
    id: 17,
    name: 'Aiden White',
    email: 'aiden.white@acme.dev',
    update_text: 'Finished 5 user flow diagrams for the checkout redesign. Available for review.',
    update_at: new Date(Date.now() - 2 * 3600_000).toISOString(),
    topics: ['User flows', 'Dashboard redesign', 'Prototyping'],
    _teamId: 'team-design',
  },
  {
    id: 18,
    name: 'Charlotte Harris',
    email: 'charlotte.harris@acme.dev',
    update_text: null,
    update_at: null,
    topics: ['Brand', 'Design system'],
    _teamId: 'team-design',
  },
]

// ── Update text pool for the live ticker ─────────────────────────────────────

export const UPDATE_TEXTS: string[] = [
  'Just pushed the fix for the race condition. Tests passing, deploying to staging now.',
  'Had a great sync with design — unblocked on the new component specs. Moving fast.',
  'Spent the morning reviewing PRs. Left detailed feedback on the auth flow changes.',
  'Blocked on the API response from the payments team. Escalating with Lucas.',
  'Finished wireframes for the settings page. Ready for team review.',
  'Performance benchmarks looking great — 35% reduction in load time after the refactor.',
  'Wrapped up user interviews for this sprint. Synthesising insights now.',
  'Fixed the flaky test in the CI pipeline. Build should be green across the board.',
  'Reviewed the Q4 roadmap. Flagged 2 items that need more scoping before we commit.',
  'Working on the database migration script. Should be ready for review by EOD.',
  'New onboarding flow is live in production. Watching metrics closely.',
  'Drafted the RFC for the new auth system. Sharing with the team for async review.',
  'Caught a critical accessibility bug in the modal component. PR is up.',
  'Stakeholder demo went great! Got sign-off on the mobile redesign direction.',
  'Deep in debugging the memory leak — found the culprit in the event listener cleanup.',
  'Running load tests on the new infrastructure. Results are promising.',
]

// ── Dataset builder ───────────────────────────────────────────────────────────

/** Returns a DashboardResponse for the given team, or all teams when teamId is null. */
export function buildDataForTeam(teamId: string | null): DashboardResponse {
  const users: UserEntry[] = (teamId === null
    ? DEMO_USERS
    : DEMO_USERS.filter((u) => u._teamId === teamId)
  ).map(({ _teamId: _, ...rest }) => rest)

  // Derive the topic set from this user slice, preserving the canonical order.
  const usedTopics = new Set(users.flatMap((u) => u.topics))
  const topics = DEMO_TOPICS.filter((t) => usedTopics.has(t))

  // Extract the sub-matrix for the active topics.
  const similarity_matrix: number[][] = topics.map((row) =>
    topics.map((col) => {
      const ri = DEMO_TOPICS.indexOf(row)
      const ci = DEMO_TOPICS.indexOf(col)
      return DEMO_SIMILARITY_MATRIX[ri]?.[ci] ?? 0
    })
  )

  return { users, topics, similarity_matrix }
}

export const DEMO_ORG_DASHBOARD: DashboardResponse = buildDataForTeam(null)
