// T5.6 service 侧验收：提及解析、门闩外规则、删除权限矩阵。
package docservice

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	sqlitestore "element-wiki/internal/store/sqlite"
)

func newCommentSvc(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	svc, db := newSvc(t)
	impl := sqlitestore.New(db)
	svc.SetCommentStore(impl, impl)
	return svc, db
}

func TestAddCommentWithMentionResolution(t *testing.T) {
	svc, db := newCommentSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "cm-svc", "C")
	svc.userLookup.CreateUser(ctx, &model.User{ID: "target", Issuer: "i",
		Subject: "target", Email: "ed@x.com", DisplayName: "Target",
		Role: permission.Viewer, Status: model.UserActive, CreatedAt: 1})

	c, err := svc.AddComment(ctx, act, d.ID,
		"@ed@x.com 看看 @ghost@x.com 这个")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Mentions) != 1 || c.Mentions[0] != "target" {
		t.Fatalf("应解析出 u1 一个提及: %v", c.Mentions)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM comment_mentions`).Scan(&n)
	if n != 1 {
		t.Errorf("mention 行数 = %d", n)
	}
	if strings.Contains(strings.ToLower(c.ID), "x") == false && false {
		t.Log("noop")
	}
}

func TestAddCommentValidation(t *testing.T) {
	svc, _ := newCommentSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "cm-val", "V")

	if _, err := svc.AddComment(ctx, act, d.ID, "   "); !errors.Is(err, ErrValidation) {
		t.Errorf("空白内容应校验失败: %v", err)
	}
	long := strings.Repeat("长", 8001)
	if _, err := svc.AddComment(ctx, act, d.ID, long); !errors.Is(err, ErrValidation) {
		t.Errorf("超长内容应校验失败: %v", err)
	}
	viewer := permission.NewActor("vw", permission.CodesFor(permission.Viewer))
	svc.userLookup.CreateUser(ctx, &model.User{ID: "vw", Issuer: "i", Subject: "vw",
		Role: permission.Viewer, Status: model.UserActive, CreatedAt: 1})
	// viewer 有 CommentCreate → 允许
	if _, err := svc.AddComment(ctx, viewer, d.ID, "hi"); err != nil {
		t.Errorf("viewer 评论应允许: %v", err)
	}
}

func TestDeleteCommentMatrix(t *testing.T) {
	svc, _ := newCommentSvc(t)
	ctx := context.Background()
	editor := editorActor()
	d, _ := svc.CreateDocument(ctx, editor, nil, "cm-del", "D")
	stranger := permission.NewActor("stg", permission.CodesFor(permission.Viewer))
	svc.userLookup.CreateUser(ctx, &model.User{ID: "stg", Issuer: "i", Subject: "stg",
		Role: permission.Viewer, Status: model.UserActive, CreatedAt: 1})

	c1, _ := svc.AddComment(ctx, editor, d.ID, "by editor")
	c2, _ := svc.AddComment(ctx, stranger, d.ID, "by stranger")

	// 他人评论：editor 无 Any 权限（模板里 admin 才有）→ 拒绝
	if err := svc.DeleteComment(ctx, editor, c2.ID); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("删他人应拒绝: %v", err)
	}
	// 作者本人
	if err := svc.DeleteComment(ctx, stranger, c2.ID); err != nil {
		t.Errorf("作者删自己应成功: %v", err)
	}
	// admin 删任意
	admin := permission.NewActor("ad", permission.CodesFor(permission.Admin))
	svc.userLookup.CreateUser(ctx, &model.User{ID: "ad", Issuer: "i", Subject: "ad",
		Role: permission.Admin, Status: model.UserActive, CreatedAt: 1})
	if err := svc.DeleteComment(ctx, admin, c1.ID); err != nil {
		t.Errorf("admin 删任意应成功: %v", err)
	}
	if err := svc.DeleteComment(ctx, admin, c1.ID); !IsNotFound(err) {
		t.Errorf("二次删除应 NotFound: %v", err)
	}
}

func editorActor() permission.Actor {
	return permission.NewActor("u1", permission.CodesFor(permission.Editor))
}

// ListComments 服务侧：升序、提及装配、权限拒绝。
func TestListCommentsService(t *testing.T) {
	svc, _ := newCommentSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "cm-list-svc", "L")
	svc.userLookup.CreateUser(ctx, &model.User{ID: "u9", Issuer: "i",
		Subject: "u9", Email: "nine@x.com", DisplayName: "N",
		Role: permission.Viewer, Status: model.UserActive, CreatedAt: 1})
	viewer := permission.NewActor("u9", permission.CodesFor(permission.Viewer))

	svc.AddComment(ctx, act, d.ID, "first @nine@x.com")
	time.Sleep(2 * time.Millisecond)
	svc.AddComment(ctx, act, d.ID, "second")

	list, err := svc.ListComments(ctx, viewer, d.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("list contents: %q, %q", list[0].Content, list[1].Content)
	if len(list) != 2 || !strings.Contains(list[0].Content, "first") {
		t.Fatalf("顺序异常: %+v", list)
	}
	if len(list[0].Mentions) != 1 || list[0].Mentions[0] != "u9" {
		t.Errorf("提及未装配: %+v", list[0].Mentions)
	}

	stranger := permission.Anonymous(false)
	if _, err := svc.ListComments(ctx, stranger, d.ID, 10); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("匿名应拒绝: %v", err)
	}
}
