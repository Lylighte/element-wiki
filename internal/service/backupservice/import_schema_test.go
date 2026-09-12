// 08 计划阶段 1 验收：上传库 schema 不受信任——列集合与固定清单不一致即整体失败，
// 主库零污染；selfID 参数化保留自身 job 行。
package backupservice

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"element-wiki/migrations"
)

// newMigrator 建库迁移器（evil 库用真实 schema 起底，再篡改）。
func newMigrator(t *testing.T, db *sql.DB) *migrations.Migrator {
	return &migrations.Migrator{DB: db, Dialect: "sqlite"}
}

// exportToZip 通过导出流程生成一份合法备份 zip 路径。
func exportToZip(t *testing.T, e *env) string {
	t.Helper()
	id, err := e.svc.StartExport(e.t.Context(), "ad")
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, func() bool {
		j, _ := e.svc.GetJob(e.t.Context(), id)
		return j.Status == "done" || j.Status == "failed"
	})
	j, _ := e.svc.GetJob(e.t.Context(), id)
	if j.Status != "done" {
		t.Fatalf("导出失败: %s", j.LastErr)
	}
	files, _ := e.svc.ListBackupFiles(e.t.Context())
	p, err := e.svc.BackupFilePath(files[0])
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// buildZipWithDB 生成含 manifest + 自定义 db 的 zip。
func buildZipWithDB(t *testing.T, path string, dbBytes []byte) {
	t.Helper()
	buildZip(t, path, map[string][]byte{
		"manifest.json": []byte(`{"schema_version":` + itoa(migrations_LatestVerInt()) + `,"created_at":0,"generator":"test","documents_total":0,"attachments_total":0}`),
		"db.sqlite3":    dbBytes,
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// evilDBBytes 以真实迁移建库后，对 documents 表做指定形态的 schema 篡改。
func evilDBBytes(t *testing.T, mutate func(db *sql.DB)) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), "evil.db")
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	m := newMigrator(t, db)
	if err := m.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	mutate(db)
	db.Close()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func importEvilAndAssertClean(t *testing.T, e *env, dbBytes []byte, wantErrFragment string) {
	t.Helper()
	var docsBefore, usersBefore int
	e.db.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&docsBefore)
	e.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&usersBefore)

	bad := filepath.Join(e.root, "evil.zip")
	buildZipWithDB(t, bad, dbBytes)

	id, err := e.svc.StartImportOfZip(e.t.Context(), "ad", bad, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, func() bool {
		j, _ := e.svc.GetJob(e.t.Context(), id)
		return j.Status == "done" || j.Status == "failed"
	})
	j, _ := e.svc.GetJob(e.t.Context(), id)
	if j.Status != "failed" {
		t.Fatalf("恶意 schema 导入不应成功, status=%s", j.Status)
	}
	if !contains(j.LastErr, wantErrFragment) {
		t.Errorf("错误应含 %q, got %q", wantErrFragment, j.LastErr)
	}
	// 主库零污染
	var docsAfter, usersAfter int
	e.db.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&docsAfter)
	e.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&usersAfter)
	if docsAfter != docsBefore || usersAfter != usersBefore {
		t.Errorf("主库被污染: documents %d→%d, users %d→%d",
			docsBefore, docsAfter, usersBefore, usersAfter)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestImportRejectsExtraColumn(t *testing.T) {
	e := newBEnv(t)
	dbBytes := evilDBBytes(t, func(db *sql.DB) {
		db.Exec(`DROP TABLE documents`)
		db.Exec(`CREATE TABLE documents (
			id TEXT PRIMARY KEY, parent_id TEXT, space_id TEXT, slug TEXT NOT NULL,
			title TEXT NOT NULL, sort_key INTEGER NOT NULL DEFAULT 0, visibility TEXT NOT NULL,
			head_commit_id TEXT NOT NULL DEFAULT '', created_by TEXT NOT NULL,
			updated_by TEXT NOT NULL, created_at BIGINT NOT NULL, updated_at BIGINT NOT NULL,
			deleted_at BIGINT, deleted_by TEXT, purge_at BIGINT, evil TEXT)`)
	})
	importEvilAndAssertClean(t, e, dbBytes, "documents")
}

func TestImportRejectsInjectionShapedColumn(t *testing.T) {
	e := newBEnv(t)
	dbBytes := evilDBBytes(t, func(db *sql.DB) {
		db.Exec(`DROP TABLE comments`)
		db.Exec(`CREATE TABLE comments (
			id TEXT PRIMARY KEY, document_id TEXT NOT NULL, author_id TEXT NOT NULL,
			"content" TEXT NOT NULL, "x); DROP TABLE users;--" TEXT, created_at BIGINT NOT NULL)`)
	})
	importEvilAndAssertClean(t, e, dbBytes, "comments")
}

func TestImportRejectsMissingColumn(t *testing.T) {
	e := newBEnv(t)
	dbBytes := evilDBBytes(t, func(db *sql.DB) {
		db.Exec(`DROP TABLE users`)
		db.Exec(`CREATE TABLE users (
			id TEXT PRIMARY KEY, issuer TEXT NOT NULL, subject TEXT NOT NULL,
			email TEXT NOT NULL DEFAULT '', display_name TEXT NOT NULL, role TEXT NOT NULL,
			status TEXT NOT NULL, created_at BIGINT NOT NULL)`) // 缺 last_login_at
	})
	importEvilAndAssertClean(t, e, dbBytes, "users")
}

func TestImportRejectsMissingDataTable(t *testing.T) {
	e := newBEnv(t)
	dbBytes := evilDBBytes(t, func(db *sql.DB) {
		db.Exec(`DROP TABLE comments`)
	})
	importEvilAndAssertClean(t, e, dbBytes, "comments")
}

func TestImportToleratesMissingOperationalTable(t *testing.T) {
	e := newBEnv(t)
	dbBytes := evilDBBytes(t, func(db *sql.DB) {
		db.Exec(`DROP TABLE backup_jobs`) // 可选表缺失应跳过
	})
	bad := filepath.Join(e.root, "ok.zip")
	buildZipWithDB(t, bad, dbBytes)

	id, err := e.svc.StartImportOfZip(e.t.Context(), "ad", bad, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, func() bool {
		j, _ := e.svc.GetJob(e.t.Context(), id)
		return j.Status == "done" || j.Status == "failed"
	})
	j, _ := e.svc.GetJob(e.t.Context(), id)
	if j.Status != "done" {
		t.Fatalf("缺可选表应容忍, failed: %s", j.LastErr)
	}
}

func TestImportKeepsSelfJobRow(t *testing.T) {
	e := newBEnv(t)
	src := newBEnv(t)
	zipPath := exportToZip(t, src)

	jobID, err := e.svc.StartImportOfZip(e.t.Context(), "ad", zipPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, func() bool {
		j, _ := e.svc.GetJob(e.t.Context(), jobID)
		return j.Status == "done" || j.Status == "failed"
	})
	j, _ := e.svc.GetJob(e.t.Context(), jobID)
	if j.Status != "done" {
		t.Fatalf("导入失败: %s", j.LastErr)
	}
	var n int
	e.db.QueryRow(`SELECT COUNT(*) FROM backup_jobs WHERE id = ?`, jobID).Scan(&n)
	if n != 1 {
		t.Errorf("导入后应保留自身 job 行, count=%d", n)
	}
}
