-- 0007: 保存待审文档修订，不推进当前 HEAD。
CREATE TABLE document_submissions (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    title TEXT,
    base_commit_id TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    author_id TEXT NOT NULL REFERENCES users(id),
    created_at BIGINT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending','approved','rejected')) DEFAULT 'pending',
    reviewed_by TEXT REFERENCES users(id),
    reviewed_at BIGINT,
    reason TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_document_submissions_pending ON document_submissions(status, created_at);
