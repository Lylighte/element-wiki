package sqlite

import (
	"context"
	"testing"

	"element-wiki/internal/model"
)

func TestContentModerationUnpublishesAndRestoresAllTypes(t *testing.T) {
	db := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, db)
	if _, err := db.db.Exec(`INSERT INTO users(id,issuer,subject,email,display_name,role,status,created_at) VALUES('admin','i','admin','','Admin','admin','active',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO documents(id,parent_id,slug,title,created_by,updated_by,created_at,updated_at) VALUES('d1',NULL,'d1','Document','u1','u1',1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO comments(id,document_id,author_id,content,created_at,status) VALUES('c1','d1','u1','comment',1,'published')`); err != nil {
		t.Fatal(err)
	}
	rev := &model.UserPageRevision{ID: "r1", UserID: "u1", Content: "profile", CreatedBy: "u1", CreatedAt: 1}
	if err := db.SubmitUserPageRevision(ctx, rev, false); err != nil {
		t.Fatal(err)
	}
	for _, target := range [][2]string{{"document", "d1"}, {"comment", "c1"}, {"user_page", "u1"}} {
		if err := db.SetContentPublished(ctx, target[0], target[1], false, "admin", "report upheld", 2); err != nil {
			t.Fatalf("unpublish %v: %v", target, err)
		}
	}
	hidden, err := db.ListHiddenContent(ctx, 50)
	if err != nil || len(hidden) != 3 {
		t.Fatalf("hidden=%v err=%v", hidden, err)
	}
	for _, target := range [][2]string{{"document", "d1"}, {"comment", "c1"}, {"user_page", "u1"}} {
		if err := db.SetContentPublished(ctx, target[0], target[1], true, "admin", "reinstated", 3); err != nil {
			t.Fatalf("restore %v: %v", target, err)
		}
	}
	hidden, err = db.ListHiddenContent(ctx, 50)
	if err != nil || len(hidden) != 0 {
		t.Fatalf("hidden after restore=%v err=%v", hidden, err)
	}
	var visibility, status, published string
	if err := db.db.QueryRow(`SELECT visibility FROM documents WHERE id='d1'`).Scan(&visibility); err != nil || visibility != string(model.VisibilityStandard) {
		t.Fatalf("document visibility=%q err=%v", visibility, err)
	}
	if err := db.db.QueryRow(`SELECT status FROM comments WHERE id='c1'`).Scan(&status); err != nil || status != "published" {
		t.Fatalf("comment status=%q err=%v", status, err)
	}
	if err := db.db.QueryRow(`SELECT published_revision_id FROM user_pages WHERE user_id='u1'`).Scan(&published); err != nil || published != "r1" {
		t.Fatalf("published revision=%q err=%v", published, err)
	}
	var n int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM moderation_actions WHERE action IN ('unpublish','restore')`).Scan(&n); err != nil || n != 6 {
		t.Fatalf("audit rows=%d err=%v", n, err)
	}
}
