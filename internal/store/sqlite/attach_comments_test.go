// T5.4/T5.6 存储层验收：附件元数据 CRUD 与评论/提及写入。
package sqlite

import (
	"context"
	"errors"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

func TestAttachmentStoreCRUD(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, s)
	if _, err := s.db.Exec(`INSERT INTO documents (id,parent_id,slug,title,created_by,updated_by,created_at,updated_at)
		VALUES ('d1',NULL,'att','t','u1','u1',1,1)`); err != nil {
		t.Fatal(err)
	}

	a := &model.Attachment{ID: "a1", DocumentID: "d1", Filename: "f.png",
		StoragePath: "d1/f.png", MimeType: "image/png", Size: 12,
		SHA256: "zz", UploadedBy: "u1", CreatedAt: 7}
	if err := s.CreateAttachment(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetAttachment(ctx, "a1")
	if err != nil || got.Filename != "f.png" {
		t.Fatalf("Get: %+v %v", got, err)
	}
	list, _ := s.ListAttachments(ctx, "d1")
	if len(list) != 1 {
		t.Fatalf("list = %d", len(list))
	}
	if _, err := s.GetAttachment(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("缺失应 NotFound: %v", err)
	}
	if err := s.DeleteAttachment(ctx, "a1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAttachment(ctx, "a1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("重复删除应 NotFound: %v", err)
	}
}

func TestCommentStoreWithMentions(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, s)
	for _, q := range []string{
		`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,created_at)
		 VALUES ('u2','i','u2','','Two',1)`,
		`INSERT INTO documents (id,parent_id,slug,title,created_by,updated_by,created_at,updated_at)
		 VALUES ('d1',NULL,'cm','t','u1','u1',1,1)`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}

	c := &model.Comment{ID: "cm1", DocumentID: "d1", AuthorID: "u1",
		Content: "hello @u2", CreatedAt: 10}
	if err := s.CreateComment(ctx, c, []string{"u2"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetComment(ctx, "cm1")
	if err != nil || got.Content != "hello @u2" {
		t.Fatalf("Get: %+v %v", got, err)
	}
	mids, _ := s.MentionIDsOf(ctx, "cm1")
	if len(mids) != 1 || mids[0] != "u2" {
		t.Errorf("mentions = %v", mids)
	}
	list, _ := s.ListComments(ctx, "d1", 10)
	if len(list) != 1 || list[0].ID != "cm1" {
		t.Errorf("list = %+v", list)
	}
	if _, err := s.ListComments(ctx, "d1", 0); err == nil {
		t.Error("limit<1 应报错")
	}
	if err := s.DeleteComment(ctx, "cm1"); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM comment_mentions`).Scan(&n)
	if n != 0 {
		t.Errorf("提及应随评论级联删除: %d", n)
	}
	if err := s.DeleteComment(ctx, "cm1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("重复删除应 NotFound: %v", err)
	}
}
