<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { DashboardStats } from '@/api'
import AdminPanelHeading from './AdminPanelHeading.vue'

defineProps<{
  stats: DashboardStats | null
  documentPath: (id: string, slug: string) => string
  formatDate: (timestamp: number) => string
}>()
const { t } = useI18n()
</script>

<template>
  <div data-test="admin-dashboard" class="admin-dashboard">
    <AdminPanelHeading :title="t('admin.dashboard')" :description="t('admin.dashboardIntro')" />
    <section class="admin-dashboard-stats" :aria-label="t('admin.siteOverview')">
      <article class="admin-dashboard-stat"><span>{{ t('admin.statDocs') }}</span><strong>{{ stats?.documents_total ?? '—' }}</strong></article>
      <article class="admin-dashboard-stat"><span>{{ t('admin.statComments') }}</span><strong>{{ stats?.comments_total ?? '—' }}</strong></article>
      <article class="admin-dashboard-stat"><span>{{ t('admin.statFiles') }}</span><strong>{{ stats?.attachments_total ?? '—' }}</strong></article>
    </section>
    <div class="admin-dashboard-lists">
      <section class="admin-dashboard-panel" data-test="dash-recent">
        <header class="admin-dashboard-panel-heading"><h2>{{ t('admin.recentDocs') }}</h2><span>{{ stats?.recent_docs?.length ?? 0 }}</span></header>
        <ul v-if="stats?.recent_docs?.length" class="admin-dashboard-list">
          <li v-for="doc in stats.recent_docs" :key="doc.id"><RouterLink :to="`/docs/${documentPath(doc.id, doc.slug)}`" class="admin-dashboard-doc"><span class="admin-dashboard-doc-title">{{ doc.title }}</span><time class="admin-dashboard-meta">{{ formatDate(doc.updated_at) }}</time></RouterLink></li>
        </ul>
        <p v-else class="admin-dashboard-empty">{{ t('admin.noRecentDocs') }}</p>
      </section>
      <section class="admin-dashboard-panel" data-test="dash-contributors">
        <header class="admin-dashboard-panel-heading"><h2>{{ t('admin.contributors') }}</h2><span>{{ stats?.contributors?.length ?? 0 }}</span></header>
        <ul v-if="stats?.contributors?.length" class="admin-dashboard-list">
          <li v-for="contributor in stats.contributors" :key="contributor.user_id" class="admin-dashboard-contributor"><span class="admin-dashboard-contributor-name">{{ contributor.name || contributor.user_id }}</span><span class="admin-dashboard-count">{{ contributor.count }}</span></li>
        </ul>
        <p v-else class="admin-dashboard-empty">{{ t('admin.noContributors') }}</p>
      </section>
    </div>
  </div>
</template>

<style scoped>
.admin-dashboard { display: flex; flex-direction: column; gap: 1.25rem; }
.admin-dashboard-stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 14rem), 1fr)); gap: 1rem; }
.admin-dashboard-stat { display: flex; flex-direction: column; gap: .45rem; min-height: 7.25rem; padding: 1.1rem 1.2rem; border: 1px solid var(--color-border); border-radius: .9rem; background: var(--color-card-background); box-shadow: 0 4px 14px rgb(15 23 42 / 4%); }
.admin-dashboard-stat span { color: var(--color-text-light); font-size: .8rem; font-weight: 550; }
.admin-dashboard-stat strong { color: var(--color-text); font-size: 1.8rem; font-weight: 700; line-height: 1.1; font-variant-numeric: tabular-nums; }
.admin-dashboard-lists { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
.admin-dashboard-panel { min-width: 0; padding: 1rem 1.15rem; border: 1px solid var(--color-border); border-radius: .9rem; background: var(--color-card-background); }
.admin-dashboard-panel-heading { display: flex; align-items: center; justify-content: space-between; gap: .8rem; padding-bottom: .8rem; border-bottom: 1px solid var(--color-border); }
.admin-dashboard-panel-heading h2 { font-size: .95rem; font-weight: 650; }
.admin-dashboard-panel-heading > span { flex: none; padding: .3rem .65rem; border-radius: 999px; background: var(--color-background-soft); color: var(--color-text-light); font-size: .75rem; font-weight: 600; }
.admin-dashboard-list { margin: 0; padding: 0; list-style: none; }
.admin-dashboard-list > li + li { border-top: 1px solid var(--color-border); }
.admin-dashboard-doc, .admin-dashboard-contributor { display: flex; align-items: center; justify-content: space-between; gap: 1rem; min-width: 0; padding: .75rem 0; }
.admin-dashboard-doc { text-decoration: none; }
.admin-dashboard-doc:hover .admin-dashboard-doc-title { color: var(--color-primary); text-decoration: underline; }
.admin-dashboard-doc-title, .admin-dashboard-contributor-name { min-width: 0; overflow: hidden; color: var(--color-text); font-size: .875rem; font-weight: 550; text-overflow: ellipsis; white-space: nowrap; }
.admin-dashboard-meta { flex: none; color: var(--color-text-light); font-size: .75rem; }
.admin-dashboard-count { flex: none; min-width: 2rem; padding: .25rem .5rem; border-radius: .5rem; background: var(--color-background-soft); color: var(--color-text-light); font-size: .75rem; font-weight: 650; text-align: center; font-variant-numeric: tabular-nums; }
.admin-dashboard-empty { padding: 1.75rem .25rem; color: var(--color-text-light); font-size: .85rem; text-align: center; }
@media (max-width: 720px) { .admin-dashboard-lists { grid-template-columns: minmax(0, 1fr); } }
</style>
