-- 0006: 评论待审核状态；旧评论保持已发布。
ALTER TABLE comments ADD COLUMN status TEXT NOT NULL DEFAULT 'published' CHECK(status IN ('pending','published','rejected'));
ALTER TABLE comments ADD COLUMN reviewed_by TEXT;
ALTER TABLE comments ADD COLUMN reviewed_at BIGINT;
ALTER TABLE comments ADD COLUMN review_reason TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_comments_review ON comments(status, created_at);
