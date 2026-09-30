<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>

<template>
  <header class="ui-page-header">
    <div class="ui-page-header-copy">
      <h1>{{ title }}</h1>
      <p v-if="description">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="ui-page-header-actions"><slot name="actions" /></div>
  </header>
</template>

<style scoped>
.ui-page-header { display: flex; align-items: flex-end; justify-content: space-between; gap: 1rem; margin-bottom: 1.25rem; }
.ui-page-header-copy { min-width: 0; }
.ui-page-header h1 { color: var(--color-heading); font-size: 1.5rem; font-weight: 700; line-height: 1.3; }
.ui-page-header p { margin-top: .35rem; color: var(--color-text-light); font-size: .875rem; line-height: 1.5; }
.ui-page-header-actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: .5rem; flex: none; }
@media (max-width: 640px) { .ui-page-header { align-items: flex-start; flex-direction: column; } .ui-page-header-actions { justify-content: flex-start; } }
</style>
