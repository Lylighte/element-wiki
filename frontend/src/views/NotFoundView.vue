<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <section class="max-w-md mx-auto mt-16 text-center space-y-3" data-test="not-found-page">
    <h1 class="text-2xl font-semibold">{{ t('common.notFound') }}</h1>
    <RouterLink to="/" class="text-blue-600 hover:underline">{{ t('nav.home') }}</RouterLink>
  </section>
</template>
