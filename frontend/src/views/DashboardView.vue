<script setup lang="ts">
import BubbleMap from '@/components/BubbleMap.vue'
import SlackInstallModal from '@/components/SlackInstallModal.vue'
</script>

<template>
  <main class="dashboard">
    <div class="dashboard__canvas" aria-label="Team pulse canvas">
      <BubbleMap />
    </div>
    <SlackInstallModal />
  </main>
</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
}

.dashboard__canvas {
  flex: 1;
  position: relative;
  animation: fadeSlideUp 0.65s ease forwards;
  animation-delay: 100ms;
  opacity: 0;
}
</style>
