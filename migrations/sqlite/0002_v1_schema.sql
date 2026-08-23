-- 0002: v1 全部业务表（doc/01 §3~§8）。双方言通用 DDL，勿使用单一方言特性。

-- ===== 身份与会话（仅 SSO，无密码字段） =====
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    issuer        TEXT NOT NULL,
    subject       TEXT NOT NULL,
    email         TEXT NOT NULL DEFAULT '',
    display_name  TEXT NOT NULL,
    role          TEXT NOT NULL CHECK(role IN ('viewer','editor','admin')) DEFAULT 'viewer',
    status        TEXT NOT NULL CHECK(status IN ('active','disabled')) DEFAULT 'active',
    created_at    BIGINT NOT NULL,
    last_login_at BIGINT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_issuer_subject ON users(issuer, subject);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS api_tokens (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    prefix       TEXT NOT NULL,
    token_hash   TEXT NOT NULL,
    created_at   BIGINT NOT NULL,
    last_used_at BIGINT NOT NULL DEFAULT 0,
    revoked_at   BIGINT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_api_tokens_user ON api_tokens(user_id);

-- ===== 文档与版本 =====
CREATE TABLE IF NOT EXISTS documents (
    id             TEXT PRIMARY KEY,
    parent_id      TEXT REFERENCES documents(id),
    space_id       TEXT,
    slug           TEXT NOT NULL,
    title          TEXT NOT NULL,
    sort_key       INTEGER NOT NULL DEFAULT 0,
    visibility     TEXT NOT NULL CHECK(visibility IN ('standard','restricted')) DEFAULT 'standard',
    head_commit_id TEXT NOT NULL DEFAULT '',
    created_by     TEXT NOT NULL REFERENCES users(id),
    updated_by     TEXT NOT NULL REFERENCES users(id),
    created_at     BIGINT NOT NULL,
    updated_at     BIGINT NOT NULL,
    deleted_at     BIGINT,
    deleted_by     TEXT,
    purge_at       BIGINT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_slug
    ON documents(COALESCE(parent_id,''), slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_deleted ON documents(purge_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_documents_parent ON documents(parent_id);

CREATE TABLE IF NOT EXISTS document_blobs (
    hash       TEXT PRIMARY KEY,
    content    TEXT NOT NULL,
    size       INTEGER NOT NULL,
    created_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS document_commits (
    id               TEXT PRIMARY KEY,
    document_id      TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    commit_no        INTEGER NOT NULL,
    parent_commit_id TEXT,
    blob_hash        TEXT NOT NULL REFERENCES document_blobs(hash),
    author_id        TEXT NOT NULL REFERENCES users(id),
    message          TEXT NOT NULL DEFAULT '',
    created_at       BIGINT NOT NULL,
    CONSTRAINT uq_commits_doc_no UNIQUE(document_id, commit_no),
    CONSTRAINT uq_commits_doc_id UNIQUE(document_id, id)
);
CREATE INDEX IF NOT EXISTS idx_commits_doc_no ON document_commits(document_id, commit_no DESC);

CREATE TABLE IF NOT EXISTS document_drafts (
    document_id    TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    base_commit_id TEXT NOT NULL,
    content        TEXT NOT NULL,
    updated_at     BIGINT NOT NULL,
    PRIMARY KEY(document_id, user_id)
);

-- ===== 协作 =====
CREATE TABLE IF NOT EXISTS comments (
    id          TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    author_id   TEXT NOT NULL REFERENCES users(id),
    content     TEXT NOT NULL,
    created_at  BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_comments_doc ON comments(document_id, created_at);

CREATE TABLE IF NOT EXISTS comment_mentions (
    comment_id TEXT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id),
    PRIMARY KEY(comment_id, user_id)
);

-- ===== 附件 =====
CREATE TABLE IF NOT EXISTS attachments (
    id           TEXT PRIMARY KEY,
    document_id  TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    filename     TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    mime_type    TEXT NOT NULL,
    size         INTEGER NOT NULL,
    sha256       TEXT NOT NULL,
    uploaded_by  TEXT NOT NULL REFERENCES users(id),
    created_at   BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_attachments_doc ON attachments(document_id);

-- ===== 后台任务 =====
CREATE TABLE IF NOT EXISTS search_reindex_jobs (
    id          TEXT PRIMARY KEY,
    document_id TEXT,
    reason      TEXT NOT NULL CHECK(reason IN ('update','delete','restore','manual','corrupt')),
    status      TEXT NOT NULL CHECK(status IN ('pending','running','done','failed')) DEFAULT 'pending',
    attempts    INTEGER NOT NULL DEFAULT 0,
    last_error  TEXT,
    created_at  BIGINT NOT NULL,
    finished_at BIGINT
);

CREATE TABLE IF NOT EXISTS backup_jobs (
    id           TEXT PRIMARY KEY,
    kind         TEXT NOT NULL CHECK(kind IN ('export','import')),
    filename     TEXT,
    status       TEXT NOT NULL CHECK(status IN ('pending','running','done','failed')) DEFAULT 'pending',
    requested_by TEXT NOT NULL REFERENCES users(id),
    last_error   TEXT,
    created_at   BIGINT NOT NULL,
    started_at   BIGINT,
    finished_at  BIGINT
);

CREATE TABLE IF NOT EXISTS import_jobs (
    id             TEXT PRIMARY KEY,
    status         TEXT NOT NULL CHECK(status IN ('pending','running','done','failed')) DEFAULT 'pending',
    total_files    INTEGER NOT NULL DEFAULT 0,
    imported_files INTEGER NOT NULL DEFAULT 0,
    failed_files   INTEGER NOT NULL DEFAULT 0,
    requested_by   TEXT NOT NULL REFERENCES users(id),
    last_error     TEXT,
    created_at     BIGINT NOT NULL,
    started_at     BIGINT,
    finished_at    BIGINT
);

-- ===== 种子设置（doc/01 §7；迁移只应用一次，普通 INSERT 安全） =====
INSERT INTO settings (key, value, updated_at) VALUES
    ('wiki_title', 'Element Wiki', 0),
    ('anonymous_read', 'false', 0),
    ('comments_enabled', 'false', 0),
    ('max_versions', '100', 0),
    ('upload_max_mb', '20', 0),
    ('allowed_extensions', 'png,jpg,jpeg,gif,webp,svg,txt,log,csv,md,zip,pdf,docx,xlsx,pptx,mp4', 0),
    ('timezone', 'UTC', 0),
    ('default_lang', 'zh-CN', 0),
    ('trash_retention_days', '30', 0)
