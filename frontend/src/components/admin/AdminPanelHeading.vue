<script setup lang="ts">
defineProps<{ title: string; description: string }>()
</script>

<template>
  <header class="admin-panel-heading">
    <div><h1>{{ title }}</h1><p>{{ description }}</p></div>
    <slot />
  </header>
</template>

<style scoped>
.admin-panel-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.admin-panel-heading h1 { color: var(--color-text); font-size: 1.25rem; font-weight: 650; line-height: 1.35; }
.admin-panel-heading p { margin-top: .3rem; color: var(--color-text-light); font-size: .875rem; line-height: 1.5; }
@media (max-width: 720px) { .admin-panel-heading { align-items: center; } }
</style>
