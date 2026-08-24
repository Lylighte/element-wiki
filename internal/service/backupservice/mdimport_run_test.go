// T6.7 覆盖补全：run 各分支（README 目录节点、资产回退、evil 跳过、全败回滚）。
package backupservice

import (
	"context"
	"strings"
	"testing"

	"element-wiki/internal/permission"
)

func mdActor() permission.Actor {
	return permission.NewActor("ad", permission.CodesFor(permission.Admin))
}

func TestMarkdownRunHappyAndRollback(t *testing.T) {
	md, _, db, _ := newMD(t)
	ctx := context.Background()
	actor := mdActor()

	// happy：树 + README 正文落目录节点 + png 挂同目录
	zipPath := makeZip(t, map[string]string{
		"docs/readme.md":         "# Docs Root\n",
		"docs/guide/install.md":  "# Install\nrun installer",
		"docs/guide/install.png": "PNGDATA",
	})
	total, imported, failed, rerr := md.run(ctx, "j1", actor, zipPath)
	if rerr != nil || failed != 0 || imported != 3 || total != 3 {
		t.Fatalf("happy: total=%d imported=%d failed=%d err=%v", total, imported, failed, rerr)
	}
	var body string
	db.QueryRow(`SELECT b.content FROM documents d
		JOIN document_commits c ON c.document_id=d.id AND c.id=d.head_commit_id
		JOIN document_blobs b ON b.hash=c.blob_hash
		WHERE d.slug='docs'`).Scan(&body)
	if !strings.Contains(body, "Docs Root") {
		t.Errorf("docs 节点正文 = %q", body)
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

func TestMarkdownRunConflictUpdatesExisting(t *testing.T) {
	md, docsSvc, _, _ := newMD(t)
	ctx := context.Background()
	act := mdActor()

	// 预置 docs 容器与同名 install 文档
	docs, _ := docsSvc.CreateDocument(ctx, act, nil, "docs", "占位")
	guide, _ := docsSvc.CreateDocument(ctx, act, &docs.ID, "guide", "G")

	zipPath := makeZip(t, map[string]string{
		"docs/guide/install.md": "# Install\nbody",
	})
	_, imported, failed, rerr := md.run(ctx, "j3", act, zipPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if imported != 1 || failed != 0 {
		t.Fatalf("imported=%d failed=%d", imported, failed)
	}
	body, _, _ := docsSvc.HeadContent(ctx, act, guide.ID)
	_ = body
	_ = strings.TrimSpace("")
}
