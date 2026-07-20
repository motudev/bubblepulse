<script setup lang="ts">
import { ref } from 'vue'
import type { Team, Role } from '@/types'

const props = defineProps<{
  teams: Team[]
  role: Role
  ownTeamId: string | null
}>()

const emit = defineEmits<{
  create: [name: string]
  rename: [id: string, name: string]
  delete: [id: string]
}>()

const newTeamName = ref('')
const editingId = ref<string | null>(null)
const editingName = ref('')

function canRename(team: Team): boolean {
  if (props.role === 'ADMIN') return true
  return props.role === 'TEAM_EDITOR' && props.ownTeamId === team.id
}

function submitCreate(): void {
  const name = newTeamName.value.trim()
  if (!name) return
  emit('create', name)
  newTeamName.value = ''
}

function startEdit(team: Team): void {
  editingId.value = team.id
  editingName.value = team.name
}

function submitRename(): void {
  const name = editingName.value.trim()
  if (editingId.value && name) {
    emit('rename', editingId.value, name)
  }
  editingId.value = null
}
</script>

<template>
  <section class="team-list">
    <h2 class="team-list__heading">Teams</h2>

    <form v-if="role === 'ADMIN'" class="team-list__create" @submit.prevent="submitCreate">
      <input
        v-model="newTeamName"
        class="team-list__input"
        type="text"
        placeholder="New team name"
        aria-label="New team name"
      />
      <button class="team-list__btn team-list__btn--primary" type="submit">Create team</button>
    </form>

    <p v-if="teams.length === 0" class="team-list__empty">No teams yet.</p>

    <ul class="team-list__items">
      <li v-for="team in teams" :key="team.id" class="team-list__item">
        <template v-if="editingId === team.id">
          <input
            v-model="editingName"
            class="team-list__input"
            type="text"
            aria-label="Team name"
            @keyup.enter="submitRename"
            @keyup.escape="editingId = null"
          />
          <button class="team-list__btn team-list__btn--primary" type="button" @click="submitRename">
            Save
          </button>
          <button class="team-list__btn" type="button" @click="editingId = null">Cancel</button>
        </template>
        <template v-else>
          <span class="team-list__name">{{ team.name }}</span>
          <button v-if="canRename(team)" class="team-list__btn" type="button" @click="startEdit(team)">
            Rename
          </button>
          <button
            v-if="role === 'ADMIN'"
            class="team-list__btn team-list__btn--danger"
            type="button"
            @click="emit('delete', team.id)"
          >
            Delete
          </button>
        </template>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.team-list__heading {
  font-family: var(--font-sans);
  font-size: var(--font-size-lg);
  font-weight: 700;
  color: var(--color-text-primary);
  margin-bottom: var(--space-4);
}

.team-list__create {
  display: flex;
  gap: var(--space-2);
  margin-bottom: var(--space-4);
}

.team-list__input {
  flex: 1;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: var(--radius-md);
  color: var(--color-text-primary);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  padding: var(--space-2) var(--space-3);
}

.team-list__input:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 1px;
}

.team-list__empty {
  color: var(--color-text-muted);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  margin-bottom: var(--space-4);
}

.team-list__items {
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.team-list__item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  background: rgba(255, 255, 255, 0.04);
}

.team-list__name {
  flex: 1;
  color: var(--color-text-primary);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
}

.team-list__btn {
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: var(--radius-md);
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: var(--font-size-xs);
  font-weight: 700;
  cursor: pointer;
  padding: var(--space-1) var(--space-3);
  transition:
    color var(--transition-fast),
    background var(--transition-fast);
}

.team-list__btn:hover {
  color: var(--color-text-primary);
  background: rgba(255, 255, 255, 0.08);
}

.team-list__btn--primary {
  background: var(--color-brand);
  border-color: var(--color-brand);
  color: var(--color-text-primary);
}

.team-list__btn--danger:hover {
  color: #ff7675;
  border-color: #ff7675;
}
</style>
