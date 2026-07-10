n<script setup lang="ts">
// BubbleMap will render the team dependency graph.
// Phase 1: placeholder. Phase 2: D3/Cytoscape implementation.
</script>

<template>
  <div class="bubble-map" role="img" aria-label="Team dependency graph">
    <p class="bubble-map__placeholder">BubbleMap coming soon.</p>
  </div>
</template>

<style scoped>
.bubble-map {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 400px;
  border: 1px dashed #ccc;
  border-radius: 8px;
}

.bubble-map__placeholder {
  color: #888;
  font-size: 1rem;
}
</style>
