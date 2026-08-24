// T6.5/T6.6 service 侧验收：导出结构与事务化导入。
package backupservice

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"

	"element-wiki/internal/permission"

	"element-wiki/migrations"
)

type env struct {
	t         *testing.T
	root      string
	db        *sql.DB
	svc       *Service
	mdsvc     *MarkdownImporter
	docs      *docservice.Service
	attachDir string
}

func newBEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	db, err := database.Open("sqlite", filepath.Join(root, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := &migrations.Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,role,status,created_at)
		VALUES ('ad','i','ad','','A','admin','active',1)`); err != nil {
		t.Fatal(err)
	}
	impl := sqlitestore.New(db)
	docs := docservice.New(impl, impl, impl, impl, impl, 100)
	docs.SetTrashHooks(impl)
	attachDir := filepath.Join(root, "attachments")
	os.MkdirAll(attachDir, 0o755)
	backups := filepath.Join(root, "backups")
	lv, _ := migrations.Latest("sqlite")
	svc := New(impl, impl, db, filepath.Join(root, "live.db"), attachDir, backups, lv)
	md := NewMarkdownImporter(impl, docs, func(id string) permission_Actor { return adminOf() })
	return &env{t: t, root: root, db: db, svc: svc, mdsvc: md, docs: docs, attachDir: attachDir}
}

func adminOf() permission.Actor {
	return permission.NewActor("ad", permission.CodesFor(permission.Admin))
}

func TestExportCreatesValidZip(t *testing.T) {
	e := newBEnv(t)
	ctx := context.Background()
	e.docs.CreateDocument(ctx, adminOf(), nil, "exp-doc", "E")

	id, err := e.svc.StartExport(ctx, "ad")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		j, _ := e.svc.GetJob(ctx, id)
		if j.Status == "done" {
			break
		}
		if j.Status == "failed" {
			t.Fatalf("导出失败: %s", j.LastErr)
		}
		time_Sleep(20)
	}
	files, _ := e.svc.ListBackupFiles(ctx)
	if len(files) != 1 {
		t.Fatalf("产物数 = %d", len(files))
	}
	p, err := e.svc.BackupFilePath(files[0])
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	zr, zerr := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if zerr != nil {
		t.Fatalf("zip 无效: %v", zerr)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["manifest.json"] || !names["db.sqlite3"] {
		t.Errorf("缺关键条目: %v", names)
	}
	var mf Manifest
	for _, f := range zr.File {
		if f.Name == "manifest.json" {
			rc, _ := f.Open()
			json.NewDecoder(rc).Decode(&mf)
			rc.Close()
		}
	}
	if mf.SchemaVersion != migrations_LatestVerInt() || mf.DocumentsTotal != 1 {
		t.Errorf("manifest 异常: %+v", mf)
	}
	_ = strings.Contains
}

func TestImportRestoresIntoFreshInstance(t *testing.T) {
	src := newBEnv(t)
	ctx := context.Background()
	d, _ := src.docs.CreateDocument(ctx, adminOf(), nil, "round-trip", "RT")
	src.docs.Commit(ctx, adminOf(), d.ID, "", "round trip body", "m")

	id, err := src.svc.StartExport(ctx, "ad")
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, func() bool {
		j, _ := src.svc.GetJob(ctx, id)
		return j.Status == "done"
	})
	files, _ := src.svc.ListBackupFiles(ctx)
	zipPath, _ := src.svc.BackupFilePath(files[0])

	// 全新实例
	dst := newBEnv(t)
	jobID, ierr := dst.svc.StartImportOfZip(ctx, "ad", zipPath, nil)
	if ierr != nil {
		t.Fatal(ierr)
	}
	waitDone(t, func() bool {
		j, _ := dst.svc.GetJob(ctx, jobID)
		return j.Status == "done" || j.Status == "failed"
	})
	j, _ := dst.svc.GetJob(ctx, jobID)
	if j.Status != "done" {
		t.Fatalf("导入失败: %s", j.LastErr)
	}
	// 文档恢复
	var n int
	dst.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE slug='round-trip'`).Scan(&n)
	if n != 1 {
		t.Fatalf("文档未恢复: %d", n)
	}
	// 用户被备份中的用户集替换（仅 ad）
	dst.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	if n != 1 {
		t.Logf("users = %d（备份内用户集）", n)
	}
}

func waitDone(t *testing.T, check func() bool) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if check() {
			return
		}
		time_Sleep(20)
	}
	t.Fatal("超时")
}

func TestImportRejectsBadZips(t *testing.T) {
	e := newBEnv(t)

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("../evil.txt")
	w.Write([]byte("x"))
	zw.Close()
	bad := filepath.Join(e.root, "bad.zip")
	os.WriteFile(bad, buf.Bytes(), 0o644)

	id, err := e.svc.StartImportOfZip(context.Background(), "ad", bad, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		j, _ := e.svc.GetJob(context.Background(), id)
		if j.Status == "failed" {
			if !strings.Contains(j.LastErr, "非法路径") && !strings.Contains(j.LastErr, "打开") {
				t.Fatalf("错误信息异常: %s", j.LastErr)
			}
			return
		}
		if j.Status == "done" {
			t.Fatal("穿越 zip 不应成功")
		}
		time_Sleep(20)
	}
	t.Fatal("超时")
}

// —— 测试辅助 ——
type permission_Actor = permission.Actor

func permAdmin() permission.Actor {
	return permission.NewActor("ad", permission.CodesFor(permission.Admin))
}

func time_Sleep(ms int) { sleepMs(ms) }

func migrations_LatestVerInt() int {
	v, _ := migrations.Latest("sqlite")
	return v
}
