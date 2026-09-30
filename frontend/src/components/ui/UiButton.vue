<script setup lang="ts">
import { computed, useAttrs } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  variant?: 'primary' | 'secondary' | 'danger' | 'warning' | 'subtle'
  size?: 'sm' | 'md' | 'lg'
  block?: boolean
  type?: 'button' | 'submit' | 'reset'
  disabled?: boolean
}>(), {
  variant: 'secondary',
  size: 'md',
  block: false,
  type: 'button',
  disabled: false,
})

const attrs = useAttrs()
const forwardedAttrs = computed(() => {
  const { class: _class, ...rest } = attrs
  return rest
})
const classes = computed(() => [
  'inline-flex items-center justify-center gap-2 rounded font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50',
  {
    primary: 'bg-[var(--color-primary)] text-white hover:brightness-95',
    secondary: 'border border-[var(--color-border)] bg-[var(--color-card-background)] text-[var(--color-text)] hover:bg-[var(--color-background-soft)]',
    danger: 'text-[var(--color-danger)] hover:underline',
    warning: 'bg-[var(--color-warning)] text-[var(--color-background)] hover:brightness-95',
    subtle: 'text-[var(--color-text)] hover:bg-[var(--color-background-mute)]',
  }[props.variant],
  {
    sm: 'px-2 py-1 text-sm',
    md: 'px-3 py-1.5',
    lg: 'px-4 py-2',
  }[props.size],
  props.block && 'w-full',
  attrs.class,
])
</script>

<template>
  <button v-bind="forwardedAttrs" :type="type" :disabled="disabled" :class="classes">
    <slot />
  </button>
</template>
