-- 0003: 操作型任务表不再硬引用 users（支持全量恢复语义）
CREATE TABLE backup_jobs_new (
    id           TEXT PRIMARY KEY,
    kind         TEXT NOT NULL CHECK(kind IN ('export','import')),
    filename     TEXT,
    status       TEXT NOT NULL CHECK(status IN ('pending','running','done','failed')) DEFAULT 'pending',
    requested_by TEXT,
    last_error   TEXT,
    created_at   BIGINT NOT NULL,
    started_at   BIGINT,
    finished_at  BIGINT
);
INSERT INTO backup_jobs_new SELECT id,kind,filename,status,requested_by,last_error,created_at,started_at,finished_at FROM backup_jobs;
DROP TABLE backup_jobs;
ALTER TABLE backup_jobs_new RENAME TO backup_jobs;

CREATE TABLE import_jobs_new (
    id             TEXT PRIMARY KEY,
    status         TEXT NOT NULL CHECK(status IN ('pending','running','done','failed')) DEFAULT 'pending',
    total_files    INTEGER NOT NULL DEFAULT 0,
    imported_files INTEGER NOT NULL DEFAULT 0,
    failed_files   INTEGER NOT NULL DEFAULT 0,
    requested_by   TEXT,
    last_error     TEXT,
    created_at     BIGINT NOT NULL,
    started_at     BIGINT,
    finished_at    BIGINT
);
INSERT INTO import_jobs_new SELECT id,status,total_files,imported_files,failed_files,requested_by,last_error,created_at,started_at,finished_at FROM import_jobs;
DROP TABLE import_jobs;
ALTER TABLE import_jobs_new RENAME TO import_jobs;
