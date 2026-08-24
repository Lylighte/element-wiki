<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>

<template>
  <h1 class="text-2xl font-semibold mb-4">{{ t('common.appName') }}</h1>
  <p class="text-gray-600">{{ t('nav.home') }}</p>
</template>
