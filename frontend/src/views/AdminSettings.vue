<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/services/api'
import type { UpdateUserPayload } from '@/services/api'
import { useUserStore } from '@/stores/user'
import TeamList from '@/components/admin/TeamList.vue'
import UserTable from '@/components/admin/UserTable.vue'
import type { Team, OrgUser, Role } from '@/types'

const userStore = useUserStore()

const teams = ref<Team[]>([])
const users = ref<OrgUser[]>([])
const errorMessage = ref('')
const orgName = ref(userStore.user?.org?.name ?? '')
const orgSaved = ref(false)

const actorRole = computed<Role>(() => userStore.user?.role ?? 'UPDATER')
const actorTeamId = computed(() => userStore.user?.team_id ?? null)
const isAdmin = computed(() => actorRole.value === 'ADMIN')
const orgNeedsName = computed(() => isAdmin.value && (userStore.user?.org?.name ?? '') === '')

async function loadAll(): Promise<void> {
  try {
    const [teamList, userList] = await Promise.all([api.getTeams(), api.getOrgUsers()])
    teams.value = teamList
    users.value = userList
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : 'Failed to load settings'
  }
}

async function run(action: () => Promise<unknown>): Promise<void> {
  errorMessage.value = ''
  try {
    await action()
    await loadAll()
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : 'Operation failed'
  }
}

function handleCreateTeam(name: string): void {
  void run(() => api.createTeam(name))
}

function handleRenameTeam(id: string, name: string): void {
  void run(() => api.updateTeam(id, name))
}

function handleDeleteTeam(id: string): void {
  void run(() => api.deleteTeam(id))
}

function handleUpdateUser(userId: number, payload: UpdateUserPayload): void {
  void run(async () => {
    await api.updateUser(userId, payload)
    // Changing our own team or role affects the nav and route guards.
    if (userId === userStore.user?.id) {
      userStore.user = await api.getMe()
    }
  })
}

async function handleRenameOrg(): Promise<void> {
  const name = orgName.value.trim()
  if (!name) return
  errorMessage.value = ''
  orgSaved.value = false
  try {
    await api.renameOrg(name)
    userStore.user = await api.getMe()
    orgSaved.value = true
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : 'Failed to rename organization'
  }
}

onMounted(loadAll)
</script>

<template>
  <main class="admin">
    <h1 class="admin__title">Admin Settings</h1>

    <p v-if="errorMessage" class="admin__error" role="alert">{{ errorMessage }}</p>

    <section v-if="isAdmin" class="admin__panel">
      <h2 class="admin__heading">Organization</h2>
      <p v-if="orgNeedsName" class="admin__hint">
        Your organization doesn't have a name yet — give it one so your team recognizes it.
      </p>
      <form class="admin__org-form" @submit.prevent="handleRenameOrg">
        <input
          v-model="orgName"
          class="admin__input"
          type="text"
          placeholder="Organization name"
          aria-label="Organization name"
        />
        <button class="admin__btn" type="submit">Save</button>
        <span v-if="orgSaved" class="admin__saved">Saved</span>
      </form>
    </section>

    <section v-if="isAdmin && userStore.user?.slack_install_enabled" class="admin__panel">
      <h2 class="admin__heading">Slack Integration</h2>
      <p class="admin__hint">
        Install the BubblePulse bot in your Slack workspace so your team can send daily updates via DM.
      </p>
      <a href="/api/slack/install" class="admin__slack-cta">
        <img
          alt="Add to Slack"
          height="40"
          width="139"
          src="https://platform.slack-edge.com/img/add_to_slack.png"
          srcset="
            https://platform.slack-edge.com/img/add_to_slack.png    1x,
            https://platform.slack-edge.com/img/add_to_slack@2x.png 2x
          "
        />
      </a>
      <p class="admin__slack-note">You can run this again at any time to refresh the connection.</p>
    </section>

    <section class="admin__panel">
      <TeamList
        :teams="teams"
        :role="actorRole"
        :own-team-id="actorTeamId"
        @create="handleCreateTeam"
        @rename="handleRenameTeam"
        @delete="handleDeleteTeam"
      />
    </section>

    <section class="admin__panel">
      <UserTable
        :users="users"
        :teams="teams"
        :actor-role="actorRole"
        :actor-team-id="actorTeamId"
        @update="handleUpdateUser"
      />
    </section>
  </main>
</template>

<style scoped>
.admin {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
  padding: var(--space-8);
  max-width: 900px;
  margin: 0 auto;
  width: 100%;
}

.admin__title {
  font-family: var(--font-sans);
  font-size: var(--font-size-2xl);
  font-weight: 700;
  color: var(--color-text-primary);
}

.admin__error {
  color: #ff7675;
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
}

.admin__panel {
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: var(--radius-lg);
  padding: var(--space-6);
}

.admin__heading {
  font-family: var(--font-sans);
  font-size: var(--font-size-lg);
  font-weight: 700;
  color: var(--color-text-primary);
  margin-bottom: var(--space-4);
}

.admin__hint {
  color: var(--color-brand-light);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  margin-bottom: var(--space-3);
}

.admin__org-form {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.admin__input {
  flex: 1;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: var(--radius-md);
  color: var(--color-text-primary);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  padding: var(--space-2) var(--space-3);
}

.admin__input:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 1px;
}

.admin__btn {
  background: var(--color-brand);
  border: none;
  border-radius: var(--radius-md);
  color: var(--color-text-primary);
  font-family: var(--font-sans);
  font-size: var(--font-size-sm);
  font-weight: 700;
  cursor: pointer;
  padding: var(--space-2) var(--space-4);
  transition: background var(--transition-fast);
}

.admin__btn:hover {
  background: var(--color-brand-light);
}

.admin__saved {
  color: var(--color-text-muted);
  font-family: var(--font-sans);
  font-size: var(--font-size-xs);
}

.admin__slack-cta {
  display: inline-block;
  border-radius: var(--radius-sm);
  transition: var(--transition-fast);
}
.admin__slack-cta img { display: block; max-width: 100%; }
.admin__slack-cta:hover { opacity: 0.85; transform: translateY(-1px); }
.admin__slack-cta:active { opacity: 1; transform: translateY(0); }
.admin__slack-cta:focus-visible { outline: 2px solid var(--color-brand); outline-offset: 4px; }

.admin__slack-note {
  font-family: var(--font-sans);
  font-size: var(--font-size-xs);
  color: var(--color-text-muted);
  margin-top: var(--space-3);
}
</style>
