<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { HiddenContent, PendingComment, PendingContentReport, PendingDocumentSubmission, PendingUserPage } from '@/api'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import AdminPanelHeading from './AdminPanelHeading.vue'

defineProps<{
  loading: boolean
  userPages: PendingUserPage[]
  comments: PendingComment[]
  documents: PendingDocumentSubmission[]
  reports: PendingContentReport[]
  hiddenContent: HiddenContent[]
}>()
const emit = defineEmits<{
  reload: []
  'decide-report': [item: PendingContentReport, action: 'resolve' | 'dismiss']
  'unpublish-report': [item: PendingContentReport]
  restore: [item: HiddenContent]
  'approve-document': [item: PendingDocumentSubmission]
  'reject-document': [item: PendingDocumentSubmission]
  'approve-user-page': [item: PendingUserPage]
  'reject-user-page': [item: PendingUserPage]
  'approve-comment': [item: PendingComment]
  'reject-comment': [item: PendingComment]
}>()
const { t } = useI18n()
</script>

<template>
  <section class="space-y-5" data-test="admin-reviews">
    <AdminPanelHeading :title="t('admin.reviews')" :description="t('admin.reviewsIntro')">
      <UiButton @click="emit('reload')">{{ t('common.retry') }}</UiButton>
    </AdminPanelHeading>
    <p v-if="loading" class="text-[var(--color-text-light)]">{{ t('common.loading') }}</p>
    <p v-else-if="!userPages.length && !comments.length && !documents.length && !reports.length && !hiddenContent.length" class="rounded-lg border border-dashed p-8 text-center text-[var(--color-text-light)]">{{ t('admin.reviewQueueEmpty') }}</p>
    <h2 v-if="reports.length" class="text-base font-semibold">{{ t('admin.contentReports') }}</h2>
    <UiCard v-for="item in reports" :key="item.id" as="article" padding="md">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div><strong>{{ t(`admin.reportType_${item.content_type}`) }}</strong><span class="ml-2 text-sm text-[var(--color-text-light)]">{{ item.content_id }} · {{ item.reporter_id }} · {{ new Date(item.created_at).toLocaleString() }}</span></div>
        <div class="flex gap-2"><UiButton variant="warning" size="sm" data-test="admin-report-unpublish" @click="emit('unpublish-report', item)">{{ t('admin.unpublish') }}</UiButton><UiButton variant="primary" size="sm" @click="emit('decide-report', item, 'resolve')">{{ t('admin.reportResolve') }}</UiButton><UiButton size="sm" @click="emit('decide-report', item, 'dismiss')">{{ t('admin.reportDismiss') }}</UiButton></div>
      </div>
      <h3 v-if="item.title" class="mb-2 font-semibold">{{ item.title }}</h3>
      <!-- eslint-disable-next-line vue/no-v-html：预览使用与文档相同的服务端消毒渲染 -->
      <div v-if="item.preview_html" class="prose prose-sm max-h-72 overflow-auto rounded bg-[var(--color-background)] p-3" v-html="item.preview_html" />
      <pre v-else-if="item.preview_text" class="max-h-72 overflow-auto whitespace-pre-wrap rounded bg-[var(--color-background)] p-3 text-sm">{{ item.preview_text }}</pre>
      <p class="whitespace-pre-wrap text-sm">{{ item.reason }}</p>
    </UiCard>
    <h2 v-if="hiddenContent.length" class="pt-4 text-base font-semibold">{{ t('admin.hiddenContent') }}</h2>
    <UiCard v-for="item in hiddenContent" :key="`${item.content_type}:${item.content_id}`" as="article" padding="md" class="flex flex-wrap items-center justify-between gap-3">
      <span>{{ t(`admin.reportType_${item.content_type}`) }} · {{ item.content_id }} · {{ new Date(item.hidden_at).toLocaleString() }}</span>
      <UiButton size="sm" data-test="admin-content-restore" @click="emit('restore', item)">{{ t('admin.restoreContent') }}</UiButton>
    </UiCard>
    <h2 v-if="documents.length" class="text-base font-semibold">{{ t('admin.documentSubmissions') }}</h2>
    <UiCard v-for="item in documents" :key="item.id" as="article" padding="md">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><div><strong>{{ item.title || item.document_id }}</strong><span class="ml-2 text-sm text-[var(--color-text-light)]">{{ item.author_id }} · {{ new Date(item.created_at).toLocaleString() }}</span></div><div class="flex gap-2"><UiButton variant="primary" size="sm" data-test="admin-review-document-approve" @click="emit('approve-document', item)">{{ t('admin.reviewApprove') }}</UiButton><UiButton size="sm" @click="emit('reject-document', item)">{{ t('admin.reviewReject') }}</UiButton></div></div>
      <pre class="max-h-72 overflow-auto whitespace-pre-wrap rounded bg-[var(--color-background-mute)] p-3 text-sm">{{ item.content }}</pre>
    </UiCard>
    <h2 v-if="userPages.length" class="text-base font-semibold">{{ t('userPage.label') }}</h2>
    <UiCard v-for="item in userPages" :key="item.id" as="article" padding="md">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><div><strong>{{ t('userPage.label') }}</strong><span class="ml-2 text-sm text-[var(--color-text-light)]">{{ item.user_id }} · {{ new Date(item.created_at).toLocaleString() }}</span></div><div class="flex gap-2"><UiButton variant="primary" size="sm" data-test="admin-review-approve" @click="emit('approve-user-page', item)">{{ t('admin.reviewApprove') }}</UiButton><UiButton size="sm" @click="emit('reject-user-page', item)">{{ t('admin.reviewReject') }}</UiButton></div></div>
      <pre class="max-h-72 overflow-auto whitespace-pre-wrap rounded bg-[var(--color-background-mute)] p-3 text-sm">{{ item.content }}</pre>
    </UiCard>
    <h2 v-if="comments.length" class="pt-4 text-base font-semibold">{{ t('comments.title') }}</h2>
    <UiCard v-for="item in comments" :key="item.id" as="article" padding="md">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><div><strong>{{ t('comments.title') }}</strong><span class="ml-2 text-sm text-[var(--color-text-light)]">{{ item.author_id }} · {{ new Date(item.created_at).toLocaleString() }}</span></div><div class="flex gap-2"><UiButton variant="primary" size="sm" @click="emit('approve-comment', item)">{{ t('admin.reviewApprove') }}</UiButton><UiButton size="sm" @click="emit('reject-comment', item)">{{ t('admin.reviewReject') }}</UiButton></div></div>
      <pre class="whitespace-pre-wrap rounded bg-[var(--color-background-mute)] p-3 text-sm">{{ item.content }}</pre>
    </UiCard>
  </section>
</template>
