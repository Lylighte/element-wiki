export interface SettingsForm {
  wiki_title: string
  timezone: string
  default_lang: 'zh-CN' | 'en'
  anonymous_read: boolean
  comments_enabled: boolean
  user_pages_enabled: boolean
  user_pages_review_required: boolean
  document_review_required: boolean
  comment_review_required: boolean
  deployment_preset: 'internal' | 'public_readonly' | 'public_contributions'
  max_versions: number
  upload_max_mb: number
  trash_retention_days: number
  allowed_extensions: string
  site_icon_url: string
  theme_preset: 'blue'
  theme_light_primary: string
  theme_light_accent: string
  theme_light_focus: string
  theme_dark_primary: string
  theme_dark_accent: string
  theme_dark_focus: string
  article_footer_markdown: string
  sidebar_footer_markdown: string
}

export interface AdminUserRow {
  id: string
  email: string
  display_name: string
  role: 'viewer' | 'editor' | 'admin'
  status: 'active' | 'disabled'
}
