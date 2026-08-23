-- 0001: 站点设置键值表（doc/01 §7）
CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at BIGINT NOT NULL,
    updated_by TEXT
)
