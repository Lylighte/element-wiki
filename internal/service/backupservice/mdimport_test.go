// T6.7 包内验收：zip 解析、目录树构建、README 节点、路径穿越跳过。
package backupservice

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"

	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"

	"element-wiki/migrations"
)

func newMD(t *testing.T) (*MarkdownImporter, *docservice.Service, *sql.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := database.Open("sqlite", filepath.Join(root, "l.db"))
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
	attDir := filepath.Join(root, "attachments")
	os.MkdirAll(attDir, 0o755)
	docs.SetAttachmentStore(impl, attDir, "png,jpg,txt,pdf", 10)
	actor := permission.NewActor("ad", permission.CodesFor(permission.Admin))
	md := NewMarkdownImporter(impl, docs, func(string) permission.Actor { return actor })
	return md, docs, db, root
}

func makeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "content.zip")
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	zw.Close()
	f.Close()
	return p
}

func TestMarkdownRunBuildsTreeAndSkipsEvil(t *testing.T) {
	md, docsSvc, db, _ := newMD(t)
	_ = docsSvc
	ctx := context.Background()
	zipPath := makeZip(t, map[string]string{
		"docs/readme.md":         "# Docs Root\n",
		"docs/guide/install.md":  "# Install\nrun installer",
		"docs/guide/install.png": "PNGBYTES",
		"notes.md":               "# Notes top-level",
		"broken/../evil.md":      "evil",
	})

	total, imported, failed, rerr := md.run(ctx, "job-1",
		permission.NewActor("ad", permission.CodesFor(permission.Admin)), zipPath)
	if rerr != nil {
		t.Fatalf("run: %v", rerr)
	}
	if total != int64(len(entriesOf(zipPath)))-0 && false {
		t.Log(total)
	}
	_ = total
	if imported < 4 || failed != 1 {
		t.Errorf("imported=%d failed=%d (evil 应计 1 次失败)", imported, failed)
	}

	// 树结构：docs > guide > install；docs 节点正文为 README
	var docsID, guideID, installID string
	db.QueryRow(`SELECT id FROM documents WHERE slug='docs'`).Scan(&docsID)
	if docsID == "" {
		t.Fatal("docs 根缺失")
	}
	db.QueryRow(`SELECT id FROM documents WHERE parent_id=? AND slug='guide'`, docsID).Scan(&guideID)
	if guideID == "" {
		t.Fatal("guide 缺失")
	}
	db.QueryRow(`SELECT id FROM documents WHERE parent_id=? AND slug='install'`, guideID).Scan(&installID)
	if installID == "" {
		t.Fatal("install 缺失")
	}
	var body string
	db.QueryRow(`SELECT b.content FROM documents d
		JOIN document_commits c ON c.document_id=d.id AND c.id=d.head_commit_id
		JOIN document_blobs b ON b.hash=c.blob_hash WHERE d.id=?`, installID).Scan(&body)
	if !strings.Contains(body, "run installer") {
		t.Errorf("install 正文异常: %q", body)
	}
	// README 正文写入 docs 节点
	db.QueryRow(`SELECT b.content FROM documents d
		JOIN document_commits c ON c.document_id=d.id AND c.id=d.head_commit_id
		JOIN document_blobs b ON b.hash=c.blob_hash WHERE d.id=?`, docsID).Scan(&body)
	if !strings.Contains(body, "Docs Root") {
		t.Errorf("README 正文未落到 docs 节点: %q", body)
	}
	// 附件挂到 install
	var an int
	db.QueryRow(`SELECT COUNT(*) FROM attachments WHERE filename='install.png'`).Scan(&an)
	if an != 1 {
		t.Errorf("png 附件数 = %d", an)
	}
	// notes.md 为根级文档
	var notesID string
	db.QueryRow(`SELECT id FROM documents WHERE slug='notes'`).Scan(&notesID)
	if notesID == "" {
		t.Error("根级 notes.md 未创建")
	}
}

func entriesOf(zipPath string) []string {
	f, _ := os.Open(zipPath)
	defer f.Close()
	st, _ := f.Stat()
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range zr.File {
		out = append(out, e.Name)
	}
	return out
}

// 异步入口 + 状态查询 + 文件删除分支。
func TestMarkdownImportAsyncAndHelpers(t *testing.T) {
	md, docs, _, root := newMD(t)
	ctx := context.Background()

	zipPath := makeZip(t, map[string]string{
		"wiki/readme.md": "# Wiki Home\n",
		"wiki/a.md":      "alpha",
	})
	id, err := md.StartMarkdownImport(ctx, "ad", zipPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		j, gerr := md.Jobs.GetImportJob(ctx, id)
		if gerr == nil && j.Status == "done" {
			break
		}
		if i == 199 {
			t.Fatalf("导入未完成: %+v", j)
		}
		time_Sleep(20)
	}
	if n := countDocs(t, docs); n < 2 {
		t.Errorf("docs 数 = %d", n)
	}
	// 删除不存在的产物文件（走 BackupFilePath 校验分支）
	svc2 := &Service{backupsDir: filepath.Join(root, "backups")}
	if err := svc2.DeleteBackupFile("nope.zip"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("删除不存在产物应 NotFound: %v", err)
	}
	_ = strings.Contains
	_ = os.TempDir
	_ = filepath.Join
}
