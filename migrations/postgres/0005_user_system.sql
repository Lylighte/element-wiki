-- 0005: 用户系统扩展（令牌有效期、偏好与个人页）
ALTER TABLE api_tokens ADD COLUMN expires_at BIGINT;

INSERT INTO settings (key, value, updated_at) VALUES
    ('user_pages_enabled', 'false', 0),
    ('user_pages_review_required', 'false', 0),
    ('document_review_required', 'false', 0),
    ('comment_review_required', 'false', 0),
    ('deployment_preset', 'internal', 0);

CREATE TABLE user_preferences (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    language TEXT NOT NULL CHECK(language IN ('zh-CN','en')),
    theme TEXT NOT NULL CHECK(theme IN ('light','dark','system')),
    updated_at BIGINT NOT NULL
);

CREATE TABLE user_pages (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    published_revision_id TEXT NOT NULL DEFAULT '',
    pending_revision_id TEXT NOT NULL DEFAULT '',
    updated_at BIGINT NOT NULL
);

CREATE TABLE user_page_revisions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending','published','rejected')),
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at BIGINT NOT NULL,
    reviewed_by TEXT,
    reviewed_at BIGINT,
    reason TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_user_page_revisions_user ON user_page_revisions(user_id, created_at DESC);
CREATE INDEX idx_user_page_revisions_pending ON user_page_revisions(status, created_at);

CREATE TABLE content_reports (
    id TEXT PRIMARY KEY,
    reporter_id TEXT NOT NULL REFERENCES users(id),
    content_type TEXT NOT NULL CHECK(content_type IN ('document','comment','user_page')),
    content_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending','resolved','dismissed')) DEFAULT 'pending',
    created_at BIGINT NOT NULL,
    reviewed_by TEXT REFERENCES users(id),
    reviewed_at BIGINT,
    resolution TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_content_reports_status ON content_reports(status, created_at);

CREATE TABLE moderation_actions (
    id TEXT PRIMARY KEY,
    content_type TEXT NOT NULL CHECK(content_type IN ('document','comment','user_page','report')),
    content_id TEXT NOT NULL,
    revision_id TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL CHECK(action IN ('approve','reject','unpublish','restore','resolve_report','dismiss_report')),
    actor_id TEXT NOT NULL REFERENCES users(id),
    reason TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL
);
CREATE INDEX idx_moderation_actions_content ON moderation_actions(content_type, content_id, created_at DESC);
