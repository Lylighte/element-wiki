CREATE TABLE content_moderation_state (
    content_type TEXT NOT NULL CHECK(content_type IN ('document','comment','user_page')),
    content_id TEXT NOT NULL,
    previous_state TEXT NOT NULL,
    hidden_by TEXT NOT NULL REFERENCES users(id),
    hidden_at BIGINT NOT NULL,
    PRIMARY KEY(content_type,content_id)
);
