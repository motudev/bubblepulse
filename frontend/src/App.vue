<script setup lang="ts">
import { RouterView } from 'vue-router'
import AppNav from '@/components/AppNav.vue'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()
</script>

<template>
  <div class="app-layout">
    <AppNav v-if="userStore.user" />
    <RouterView />
  </div>
</template>

<style scoped>
.app-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
}
</style>
