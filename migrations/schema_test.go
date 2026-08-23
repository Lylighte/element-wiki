package migrations

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"element-wiki/internal/database"
)

// T0.3 验收：v1 全部表结构 + 约束 + 种子设置的机器断言。

func v1DB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "v1.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := &Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(context.Background()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

func mustExec(t *testing.T, db *sql.DB, q string) sql.Result {
	t.Helper()
	res, err := db.Exec(q)
	if err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
	return res
}

func wantExecErr(t *testing.T, db *sql.DB, q string) error {
	t.Helper()
	_, err := db.Exec(q)
	if err == nil {
		t.Fatalf("期望报错: %s", q)
	}
	return err
}

func wantErrContains(t *testing.T, err error, subs ...string) {
	t.Helper()
	s := err.Error()
	for _, sub := range subs {
		if strings.Contains(strings.ToLower(s), strings.ToLower(sub)) {
			return
		}
	}
	t.Fatalf("错误 %q 未包含任一关键词 %v", s, subs)
}

const seedUser = `INSERT INTO users (id, issuer, subject, email, display_name, role, status, created_at)
VALUES ('u1','https://idp','sub1','a@x.com','A','admin','active',1)`

const insertDocTmpl = `INSERT INTO documents (id,parent_id,slug,title,created_by,updated_by,created_at,updated_at)
		VALUES ('%s',NULL,'doc','t','u1','u1',1,1)`

func insertDoc(id string) string { return strings.Replace(insertDocTmpl, "%s", id, 1) }

func TestSeedSettingsPresent(t *testing.T) {
	db := v1DB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&n); err != nil || n != 9 {
		t.Fatalf("种子键数量 = %d, err=%v, 期望 9", n, err)
	}
	var v string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key='comments_enabled'`).Scan(&v); err != nil || v != "false" {
		t.Errorf("comments_enabled 种子 = %q (err=%v), 必须 false", v, err)
	}
	db.QueryRow(`SELECT value FROM settings WHERE key='max_versions'`).Scan(&v)
	if v != "100" {
		t.Errorf("max_versions 种子 = %q", v)
	}
}

func TestUsersUniqueIssuerSubjectAndChecks(t *testing.T) {
	db := v1DB(t)
	mustExec(t, db, seedUser)

	err := wantExecErr(t, db,
		`INSERT INTO users (id, issuer, subject, display_name, created_at)
		 VALUES ('u2','https://idp','sub1','B',2)`)
	wantErrContains(t, err, "UNIQUE")

	err = wantExecErr(t, db,
		`INSERT INTO users (id, issuer, subject, display_name, role, created_at)
		 VALUES ('u3','i','s3','C','superuser',3)`)
	wantErrContains(t, err, "CHECK")
}

func TestDocumentSlugPartialUnique(t *testing.T) {
	db := v1DB(t)
	mustExec(t, db, seedUser)

	doc := func(id, parent, slug string) string {
		p := "NULL"
		if parent != "" {
			p = "'" + parent + "'"
		}
		return `INSERT INTO documents (id,parent_id,slug,title,created_by,updated_by,created_at,updated_at)
			VALUES ('` + id + `',` + p + `,'` + slug + `','t','u1','u1',1,1)`
	}
	mustExec(t, db, doc("d_root1", "", "guide"))
	mustExec(t, db, doc("d_p", "", "parent"))

	err := wantExecErr(t, db, doc("d_x", "", "guide"))
	wantErrContains(t, err, "UNIQUE") // 同父级（根）重复

	mustExec(t, db, doc("d_child", "d_p", "guide")) // 不同父级允许同 slug

	mustExec(t, db, `UPDATE documents SET deleted_at=100 WHERE id='d_root1'`)
	mustExec(t, db, doc("d_new", "", "guide")) // 回收站文档释放 slug
}

func TestCommitsUniquenessAndBlobFK(t *testing.T) {
	db := v1DB(t)
	mustExec(t, db, seedUser)
	mustExec(t, db, insertDoc("d1"))
	mustExec(t, db, `INSERT INTO document_blobs (hash,content,size,created_at) VALUES ('h1','c',1,1)`)

	commit := func(no int, blob string) string {
		return `INSERT INTO document_commits (id,document_id,commit_no,blob_hash,author_id,created_at)
			VALUES ('c` + strconv.Itoa(no) + `','d1',` + strconv.Itoa(no) + `,'` + blob + `','u1',1)`
	}
	mustExec(t, db, commit(1, "h1"))

	wantErrContains(t, wantExecErr(t, db, commit(1, "h1")), "UNIQUE")
	wantErrContains(t, wantExecErr(t, db, commit(2, "nope")), "FOREIGN", "CONSTRAINT")
}

func TestDraftCompositePK(t *testing.T) {
	db := v1DB(t)
	mustExec(t, db, seedUser)
	mustExec(t, db, insertDoc("d1"))

	draft := `INSERT INTO document_drafts (document_id,user_id,base_commit_id,content,updated_at)
		VALUES ('d1','u1','h','x',1)`
	mustExec(t, db, draft)
	wantErrContains(t, wantExecErr(t, db, draft), "PRIMARY", "UNIQUE")
}

func TestAttachmentCascadeOnDocumentDelete(t *testing.T) {
	db := v1DB(t)
	mustExec(t, db, seedUser)
	mustExec(t, db, insertDoc("d1"))
	mustExec(t, db, `INSERT INTO attachments (id,document_id,filename,storage_path,mime_type,size,sha256,uploaded_by,created_at)
		VALUES ('f1','d1','a.png','d1/a.png','image/png',10,'hh','u1',1)`)

	mustExec(t, db, `DELETE FROM documents WHERE id='d1'`)
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM attachments`).Scan(&n)
	if n != 0 {
		t.Errorf("删除文档后附件应级联清除, 剩 %d", n)
	}
}

func TestJobTableChecks(t *testing.T) {
	db := v1DB(t)
	mustExec(t, db, seedUser)
	wantErrContains(t, wantExecErr(t, db,
		`INSERT INTO search_reindex_jobs (id,reason,created_at) VALUES ('j1','magic',1)`), "CHECK")
	wantErrContains(t, wantExecErr(t, db,
		`INSERT INTO backup_jobs (id,kind,requested_by,created_at) VALUES ('b1','snapshot','u1',1)`), "CHECK")
}
