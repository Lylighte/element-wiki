<script setup lang="ts">
import { Search, Close } from '@element-plus/icons-vue'
import UiButton from './UiButton.vue'

defineOptions({ inheritAttrs: false })
const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  submitLabel?: string
  loadingLabel?: string
  loading?: boolean
  disabled?: boolean
}>(), { placeholder: '', submitLabel: '', loadingLabel: '', loading: false, disabled: false })
const emit = defineEmits<{
  'update:modelValue': [value: string]
  submit: []
  clear: []
}>()
</script>

<template>
  <div class="ui-search-field">
    <Search class="ui-search-field-icon" aria-hidden="true" />
    <input
      v-bind="$attrs"
      :value="modelValue"
      type="search"
      :placeholder="placeholder"
      :disabled="disabled || loading"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      @keydown.enter.prevent="emit('submit')"
    />
    <button v-if="modelValue" type="button" class="ui-search-field-clear" :aria-label="'Clear search'" @click="emit('update:modelValue', ''); emit('clear')">
      <Close aria-hidden="true" />
    </button>
    <UiButton type="button" variant="primary" size="md" class="ui-search-field-submit" :disabled="disabled || loading || !modelValue.trim()" @click="emit('submit')">
      <span v-if="loading" class="ui-search-field-spinner" aria-hidden="true" />
      {{ loading ? loadingLabel : submitLabel }}
    </UiButton>
  </div>
</template>

<style scoped>
.ui-search-field { display: flex; align-items: stretch; width: 100%; min-width: 0; min-height: 2.75rem; overflow: hidden; border: 1px solid var(--color-border); border-radius: .7rem; background: var(--color-card-background); transition: border-color .15s, box-shadow .15s; }
.ui-search-field:focus-within { border-color: var(--color-primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-primary) 15%, transparent); }
.ui-search-field-icon { align-self: center; width: 1.1rem; height: 1.1rem; margin-left: .8rem; flex: none; color: var(--color-text-light); }
.ui-search-field input { min-width: 0; flex: 1; border: 0; outline: 0; padding: .55rem .65rem; background: transparent; color: var(--color-text); font: inherit; }
.ui-search-field input::placeholder { color: var(--color-text-light); }
.ui-search-field-clear { display: inline-flex; align-items: center; justify-content: center; width: 2rem; border: 0; background: transparent; color: var(--color-text-light); cursor: pointer; }
.ui-search-field-clear:hover { color: var(--color-text); background: var(--color-background-soft); }
.ui-search-field-clear svg { width: .95rem; height: .95rem; }
.ui-search-field-submit { min-width: 5rem; border-radius: 0; }
.ui-search-field-spinner { width: .9rem; height: .9rem; border: 2px solid color-mix(in srgb, currentColor 35%, transparent); border-top-color: currentColor; border-radius: 999px; animation: ui-search-spin .7s linear infinite; }
@keyframes ui-search-spin { to { transform: rotate(360deg); } }
</style>
