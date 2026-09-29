-- 同一用户对同一内容仅允许保留一条待审举报。
CREATE UNIQUE INDEX idx_content_reports_unique_pending
ON content_reports(reporter_id,content_type,content_id) WHERE status='pending';
