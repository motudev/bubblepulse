<script setup lang="ts">
import type { OrgUser, Team, Role } from '@/types'
import type { UpdateUserPayload } from '@/services/api'

const props = defineProps<{
  users: OrgUser[]
  teams: Team[]
  actorRole: Role
  actorTeamId: string | null
}>()

const emit = defineEmits<{
  update: [userId: number, payload: UpdateUserPayload]
}>()

const ROLES: Role[] = ['ADMIN', 'TEAM_EDITOR', 'UPDATER']
const NO_TEAM = ''

/**
 * TEAM_EDITORs may only move unassigned users into their own team or remove
 * members from it; ADMINs may assign anyone anywhere.
 */
function teamOptions(): Team[] {
  if (props.actorRole === 'ADMIN') return props.teams
  return props.teams.filter((t) => t.id === props.actorTeamId)
}

function canEditTeam(user: OrgUser): boolean {
  if (props.actorRole === 'ADMIN') return true
  if (props.actorRole !== 'TEAM_EDITOR' || !props.actorTeamId) return false
  return user.team_id === null || user.team_id === props.actorTeamId
}

function teamName(teamId: string | null): string {
  return props.teams.find((t) => t.id === teamId)?.name ?? '—'
}

function onTeamChange(user: OrgUser, event: Event): void {
  const value = (event.target as HTMLSelectElement).value
  emit('update', user.id, { team_id: value === NO_TEAM ? null : value })
}

function onRoleChange(user: OrgUser, event: Event): void {
  const value = (event.target as HTMLSelectElement).value as Role
  emit('update', user.id, { role: value })
}
</script>

<template>
  <section class="user-table">
    <h2 class="user-table__heading">Members</h2>
    <table class="user-table__table">
      <thead>
        <tr>
          <th class="user-table__th">Name</th>
          <th class="user-table__th">Email</th>
          <th class="user-table__th">Team</th>
          <th class="user-table__th">Role</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="user in users" :key="user.id" class="user-table__row">
          <td class="user-table__td">{{ user.name }}</td>
          <td class="user-table__td user-table__td--muted">{{ user.email }}</td>
          <td class="user-table__td">
            <select
              v-if="canEditTeam(user)"
              class="user-table__select"
              :value="user.team_id ?? NO_TEAM"
              :aria-label="`Team for ${user.name}`"
              @change="onTeamChange(user, $event)"
            >
              <option :value="NO_TEAM">No team</option>
              <option v-for="team in teamOptions()" :key="team.id" :value="team.id">
                {{ team.name }}
              </option>
            </select>
            <span v-else>{{ teamName(user.team_id) }}</span>
          </td>
          <td class="user-table__td">
            <select
              v-if="actorRole === 'ADMIN'"
              class="user-table__select"
              :value="user.role"
              :aria-label="`Role for ${user.name}`"
              @change="onRoleChange(user, $event)"
            >
              <option v-for="role in ROLES" :key="role" :value="role">{{ role }}</option>
            </select>
            <span v-else>{{ user.role }}</span>
          </td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<style scoped>
.user-table__heading {
  font-family: var(--font-sans);
  font-size: var(--font-size-lg);
  font-weight: 700;
  color: var(--color-text-primary);
  margin-bottom: var(--space-4);
}

.user-table__table {
  width: 100%;
  border-collapse: collapse;
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
}

.user-table__th {
  text-align: left;
  color: var(--color-text-muted);
  font-size: var(--font-size-xs);
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.user-table__row:hover {
  background: rgba(255, 255, 255, 0.04);
}

.user-table__td {
  color: var(--color-text-primary);
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.user-table__td--muted {
  color: var(--color-text-secondary);
}

.user-table__select {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: var(--radius-md);
  color: var(--color-text-primary);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  padding: var(--space-1) var(--space-2);
}

.user-table__select:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 1px;
}
</style>
