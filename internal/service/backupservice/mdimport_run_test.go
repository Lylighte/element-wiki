// T17.1 覆盖补全：run 各分支（隔离根、README 容器正文、资产挂载、
// 冲突计失败零覆盖、CJK 文件名自动 slug、空 zip 失败、全败回滚、反斜杠宽容）。
package backupservice

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"element-wiki/internal/permission"
)

func mdActor() permission.Actor {
	return permission.NewActor("ad", permission.CodesFor(permission.Admin))
}

// importRootSlug 查找本次导入的隔离根（唯一 import- 前缀根级文档）。
func importRootSlug(t *testing.T, db *sql.DB) string {
	t.Helper()
	var slug string
	if err := db.QueryRow(
		`SELECT slug FROM documents WHERE parent_id IS NULL AND deleted_at IS NULL AND slug LIKE 'import-%'
		 ORDER BY created_at DESC LIMIT 1`).Scan(&slug); err != nil {
		t.Fatalf("隔离根缺失: %v", err)
	}
	return slug
}

func TestMarkdownRunHappyAndRollback(t *testing.T) {
	md, _, db, _ := newMD(t)
	ctx := context.Background()
	actor := mdActor()

	// happy：树 + README 正文落目录节点 + png 挂同 stem 文档，全部在隔离根下
	zipPath := makeZip(t, map[string]string{
		"docs/readme.md":         "# Docs Root\n",
		"docs/guide/install.md":  "# Install\nrun installer",
		"docs/guide/install.png": "PNGDATA",
	})
	total, imported, failed, rerr := md.run(ctx, "j1", actor, zipPath)
	if rerr != nil || failed != 0 || imported != 3 || total != 3 {
		t.Fatalf("happy: total=%d imported=%d failed=%d err=%v", total, imported, failed, rerr)
	}
	rootSlug := importRootSlug(t, db)

	// docs 容器挂在隔离根下
	var docsID, rootID string
	if err := db.QueryRow(`SELECT id FROM documents WHERE slug='docs' AND deleted_at IS NULL`).Scan(&docsID); err != nil {
		t.Fatalf("docs 容器缺失: %v", err)
	}
	if err := db.QueryRow(`SELECT id FROM documents WHERE slug=? AND deleted_at IS NULL`, rootSlug).Scan(&rootID); err != nil {
		t.Fatalf("隔离根缺失: %v", err)
	}
	var parentID string
	db.QueryRow(`SELECT parent_id FROM documents WHERE id=?`, docsID).Scan(&parentID)
	if parentID != rootID {
		t.Errorf("docs 应挂在隔离根下: parent=%s root=%s", parentID, rootID)
	}

	var body string
	db.QueryRow(`SELECT b.content FROM documents d
		JOIN document_commits c ON c.document_id=d.id AND c.id=d.head_commit_id
		JOIN document_blobs b ON b.hash=c.blob_hash
		WHERE d.id=?`, docsID).Scan(&body)
	if !strings.Contains(body, "Docs Root") {
		t.Errorf("README 正文未落到 docs 容器: %q", body)
	}

	// 附件挂到同 stem 文档 install
	var an int
	db.QueryRow(`SELECT COUNT(*) FROM attachments WHERE filename='install.png'`).Scan(&an)
	if an != 1 {
		t.Errorf("png 附件数 = %d", an)
	}
}

func TestMarkdownRunFullFailureRollsBack(t *testing.T) {
	md, docs, _, _ := newMD(t)
	ctx := context.Background()

	before := countDocs(t, docs)

	// 全部条目失败：唯一 md 是路径穿越
	zipPath := makeZip(t, map[string]string{
		"broken/../evil.md": "x",
	})
	total, imported, failed, rerr := md.run(ctx, "j2", mdActor(), zipPath)
	if rerr != nil {
		t.Fatalf("run 不应返回 error（失败记录在 job）: %v", rerr)
	}
	if imported != 0 || failed != 1 {
		t.Errorf("imported=%d failed=%d", imported, failed)
	}
	if after := countDocs(t, docs); after != before {
		t.Fatalf("回滚后文档数变化: %d -> %d", before, after)
	}
	_ = total
}

// 隔离根语义反转：与站点既有同名文档零冲突，既有内容绝不被触碰。
func TestMarkdownRunQuarantineNeverTouchesExisting(t *testing.T) {
	md, docsSvc, db, _ := newMD(t)
	ctx := context.Background()
	act := mdActor()

	docs, _ := docsSvc.CreateDocument(ctx, act, nil, "docs", "占位")
	guide, _ := docsSvc.CreateDocument(ctx, act, &docs.ID, "guide", "G")
	install, _ := docsSvc.CreateDocument(ctx, act, &guide.ID, "install", "旧内容")
	origBody, _, _ := docsSvc.HeadContent(ctx, act, install.ID)

	zipPath := makeZip(t, map[string]string{
		"docs/guide/install.md": "# Install\n新正文",
	})
	_, imported, failed, rerr := md.run(ctx, "j3", act, zipPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if imported != 1 || failed != 0 {
		t.Fatalf("imported=%d failed=%d", imported, failed)
	}

	// 既有文档内容原封不动
	newBody, _, _ := docsSvc.HeadContent(ctx, act, install.ID)
	if newBody != origBody {
		t.Errorf("既有文档被触碰: %q -> %q", origBody, newBody)
	}

	// 导入内容在隔离根下有独立副本
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM documents WHERE slug='install' AND deleted_at IS NULL`).Scan(&n)
	if n != 2 {
		t.Errorf("应存在两个 install（既有 + 隔离根副本）, got %d", n)
	}
}

// zip 自身同名 slug（大小写变体）→ 冲突计失败，不覆盖先导入者。
func TestMarkdownRunZipInternalSlugConflictFails(t *testing.T) {
	md, _, db, _ := newMD(t)
	ctx := context.Background()

	zipPath := makeZip(t, map[string]string{
		"docs/a.md":  "# first\nAAA",
		"docs/A.md":  "# second\nBBB",
	})
	_, imported, failed, rerr := md.run(ctx, "j4", mdActor(), zipPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if imported != 1 || failed != 1 {
		t.Fatalf("imported=%d failed=%d", imported, failed)
	}
	var bodies int
	db.QueryRow(`SELECT COUNT(*) FROM document_blobs WHERE content LIKE '%AAA%' OR content LIKE '%BBB%'`).Scan(&bodies)
	if bodies != 1 {
		t.Errorf("只应有一个 md 正文落库, got %d", bodies)
	}
}

// CJK 文件名/目录名：slug 自动生成，中文标题保留。
func TestMarkdownRunCJKFilenames(t *testing.T) {
	md, _, db, _ := newMD(t)
	ctx := context.Background()

	zipPath := makeZip(t, map[string]string{
		"指南/安装.md": "# 安装指南\n步骤",
	})
	_, imported, failed, rerr := md.run(ctx, "j5", mdActor(), zipPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if imported != 1 || failed != 0 {
		t.Fatalf("imported=%d failed=%d", imported, failed)
	}

	var title, slug string
	if err := db.QueryRow(`SELECT title, slug FROM documents WHERE title='安装指南' AND deleted_at IS NULL`).Scan(&title, &slug); err != nil {
		t.Fatalf("中文标题文档缺失: %v", err)
	}
	if slug != "doc" && !strings.HasPrefix(slug, "doc-") {
		t.Errorf("CJK 文件名应走自动 slug, got %q", slug)
	}
	// 目录容器保留中文标题
	var dirTitle string
	if err := db.QueryRow(`SELECT title FROM documents WHERE title='指南' AND deleted_at IS NULL`).Scan(&dirTitle); err != nil {
		t.Errorf("CJK 目录容器缺失: %v", err)
	}
}

// 空 zip → job 失败，不建任何文档。
func TestMarkdownRunEmptyZipFails(t *testing.T) {
	md, docs, _, _ := newMD(t)
	ctx := context.Background()

	before := countDocs(t, docs)
	zipPath := makeZip(t, map[string]string{})
	total, imported, failed, rerr := md.run(ctx, "j6", mdActor(), zipPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if total != 0 || imported != 0 || failed != 0 {
		t.Errorf("total=%d imported=%d failed=%d", total, imported, failed)
	}
	if after := countDocs(t, docs); after != before {
		t.Errorf("空 zip 不应建文档: %d -> %d", before, after)
	}
}

// Windows 反斜杠路径宽容归一：docs\a.md 按 docs/a.md 导入。
func TestMarkdownRunBackslashPaths(t *testing.T) {
	md, _, db, _ := newMD(t)
	ctx := context.Background()

	zipPath := makeZip(t, map[string]string{
		"docs\\a.md": "# A\nbody",
	})
	_, imported, failed, rerr := md.run(ctx, "j7", mdActor(), zipPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if imported != 1 || failed != 0 {
		t.Fatalf("imported=%d failed=%d", imported, failed)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM documents WHERE slug='docs' AND deleted_at IS NULL`).Scan(&n)
	if n != 1 {
		t.Errorf("反斜杠路径应归一为 docs 容器, got %d", n)
	}
}

// README 大小写变体：首个生效，其余计失败。
func TestMarkdownRunReadmeVariants(t *testing.T) {
	md, _, db, _ := newMD(t)
	ctx := context.Background()

	zipPath := makeZip(t, map[string]string{
		"docs/readme.md": "# first\nAAA",
		"docs/README.md": "# second\nBBB",
	})
	_, imported, failed, rerr := md.run(ctx, "j8", mdActor(), zipPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if imported != 1 || failed != 1 {
		t.Fatalf("imported=%d failed=%d", imported, failed)
	}
	// 条目按名称字节序稳定排序：README.md（大写 R）先于 readme.md，首个生效
	var body string
	db.QueryRow(`SELECT b.content FROM documents d
		JOIN document_commits c ON c.document_id=d.id AND c.id=d.head_commit_id
		JOIN document_blobs b ON b.hash=c.blob_hash
		WHERE d.slug='docs' AND d.deleted_at IS NULL`).Scan(&body)
	if !strings.Contains(body, "BBB") {
		t.Errorf("README 应取排序首个变体（README.md）: %q", body)
	}
}
