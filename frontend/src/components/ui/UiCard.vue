<script setup lang="ts">
withDefaults(defineProps<{
  as?: 'div' | 'section' | 'article' | 'li'
  padding?: 'none' | 'sm' | 'md' | 'lg'
}>(), {
  as: 'section',
  padding: 'none',
})
</script>

<template>
  <component
    :is="as"
    class="rounded-lg border border-[var(--color-border)] bg-[var(--color-card-background)]"
    :class="{
      'p-2': padding === 'sm',
      'p-4': padding === 'md',
      'p-6': padding === 'lg',
    }"
  >
    <slot />
  </component>
</template>
