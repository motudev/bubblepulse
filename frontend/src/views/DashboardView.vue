<script setup lang="ts">
import BubbleMap from '@/components/BubbleMap.vue'
</script>

<template>
  <main class="dashboard">
    <header class="dashboard__header">
      <h1>Today's Check-ins</h1>
    </header>
    <BubbleMap />
  </main>
</template>

<style scoped>
.dashboard {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem;
}

.dashboard__header {
  margin-bottom: 2rem;
}

.dashboard__header h1 {
  font-size: 1.75rem;
  font-weight: 600;
}
</style>
