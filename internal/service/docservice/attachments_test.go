// T5.4/T5.5 service 侧验收：受控提交、校验分支、清理语义。
package docservice

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
	sqlitestore "element-wiki/internal/store/sqlite"
)

type failStore struct{}

func (failStore) CreateAttachment(context.Context, *model.Attachment) error {
	return errors.New("boom")
}
func (failStore) GetAttachment(context.Context, string) (*model.Attachment, error) {
	return nil, store.ErrNotFound
}
func (failStore) ListAttachments(context.Context, string) ([]*model.Attachment, error) {
	return nil, nil
}
func (failStore) DeleteAttachment(context.Context, string) error { return nil }

func newAttachSvc(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	svc, db := newSvc(t)
	dir := filepath.Join(t.TempDir(), "att")
	os.MkdirAll(dir, 0o755)
	impl := sqlitestore.New(db)
	svc.SetCommentStore(impl, impl)
	svc.SetAttachmentStore(impl, dir, "png,jpg,txt,pdf", 1)
	return svc, db
}

func TestUploadAttachmentHappyPath(t *testing.T) {
	svc, _ := newAttachSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "att-happy", "A")

	a, err := svc.UploadAttachment(ctx, act, d.ID, "报告 final.PDF", strings.NewReader("hello pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if a.MimeType != "application/pdf" || a.Size != 9 {
		t.Errorf("元数据异常: %+v", a)
	}
	full := filepath.Join(svc.attachDir, a.StoragePath)
	data, rerr := os.ReadFile(full)
	if rerr != nil || string(data) != "hello pdf" {
		t.Fatalf("落盘内容不符: %q %v", data, rerr)
	}

	list, _ := svc.ListAttachments(ctx, act, d.ID)
	if len(list) != 1 || list[0].ID != a.ID {
		t.Errorf("列表: %+v", list)
	}
	gotMeta, gerr := svc.GetAttachment(ctx, act, a.ID)
	if gerr != nil || gotMeta.Filename != "报告 final.PDF" {
		t.Errorf("Get: %+v %v", gotMeta, gerr)
	}
	if derr := svc.DeleteAttachment(ctx, act, a.ID); derr != nil {
		t.Fatal(derr)
	}
	if _, serr := os.Stat(full); !errors.Is(serr, os.ErrNotExist) {
		t.Errorf("删除后文件残留: %v", serr)
	}
	if derr := svc.DeleteAttachment(ctx, act, a.ID); !IsNotFound(derr) {
		t.Errorf("二次删除应 NotFound: %v", derr)
	}
}

func TestUploadValidationAndCleanup(t *testing.T) {
	svc, _ := newAttachSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "att-val", "V")

	before := dirEntries(t, svc.attachDir)

	if _, err := svc.UploadAttachment(ctx, act, d.ID, "evil.exe",
		strings.NewReader("MZ")); !errors.Is(err, ErrBadType) {
		t.Errorf("exe 应拒绝: %v", err)
	}
	big := strings.Repeat("a", int(svc.maxBytes)+10)
	if _, err := svc.UploadAttachment(ctx, act, d.ID, "big.txt",
		strings.NewReader(big)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("超限应拒绝: %v", err)
	}
	if _, err := svc.UploadAttachment(ctx, act, "ghost", "ok.txt",
		strings.NewReader("x")); !IsNotFound(err) {
		t.Errorf("幽灵上传应 NotFound: %v", err)
	}

	if after := dirEntries(t, svc.attachDir); len(after) != len(before) {
		t.Errorf("失败路径留下文件: before=%v after=%v", before, after)
	}
}

// DB 写入失败 → 已落盘文件必须被清理（AGENTS §6）。
func TestUploadDBFailureCleansFile(t *testing.T) {
	svc, _ := newSvc(t)
	dir := filepath.Join(t.TempDir(), "att-fail")
	os.MkdirAll(dir, 0o755)
	svc.SetAttachmentStore(failStore{}, dir, "txt", 1)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "dbfail", "D")

	_, err := svc.UploadAttachment(ctx, act, d.ID, "ok.txt", strings.NewReader("data"))
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("应透出 DB 错误: %v", err)
	}
	if entries := dirEntries(t, dir); len(entries) != 0 {
		t.Errorf("DB 失败后应无孤儿文件: %v", entries)
	}
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	out := []string{}
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, filepath.Base(p))
		}
		return nil
	})
	return out
}

func TestSanitizeFilenameEdges(t *testing.T) {
	cases := map[string]string{
		"":            "file",
		"..":          "file",
		"a/b\\c:d\"e": "a-b-c-d-e",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("sanitize(%q)=%q want %q", in, got, want)
		}
	}
}

// viewer 无上传权限、有读权限的分支。
func TestAttachmentReadVsWritePerms(t *testing.T) {
	svc, _ := newAttachSvc(t)
	ctx := context.Background()
	act := editor()
	viewer := permission.NewActor("vw", permission.CodesFor(permission.Viewer))
	svc.userLookup.CreateUser(ctx, &model.User{ID: "vw", Issuer: "i", Subject: "vw",
		Role: permission.Viewer, Status: model.UserActive, CreatedAt: 1})
	d, _ := svc.CreateDocument(ctx, act, nil, "perm-att", "P")
	a, uerr := svc.UploadAttachment(ctx, act, d.ID, "ok.txt", strings.NewReader("z"))
	if uerr != nil {
		t.Fatal(uerr)
	}
	if _, err := svc.ListAttachments(ctx, viewer, d.ID); err != nil {
		t.Errorf("viewer 列表应允许: %v", err)
	}
	if _, err := svc.GetAttachment(ctx, viewer, a.ID); err != nil {
		t.Errorf("viewer 读取应允许: %v", err)
	}
	if err := svc.DeleteAttachment(ctx, viewer, a.ID); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("viewer 删除应拒绝: %v", err)
	}
}

// 幽灵文档的列表/读取/删除分支。
func TestAttachmentGhostDocBranches(t *testing.T) {
	svc, _ := newAttachSvc(t)
	ctx := context.Background()
	act := editor()
	if _, err := svc.ListAttachments(ctx, act, "ghost"); !IsNotFound(err) {
		t.Errorf("list: %v", err)
	}
	if err := svc.DeleteAttachment(ctx, act, "ghost"); err == nil {
		t.Error("ghost delete 应报错")
	}
}

func TestServiceTestHelpers(t *testing.T) {
	svc, db := newSvc(t)
	if svc.AttachDir() != "" {
		t.Errorf("未注入时 AttachDir 应为空")
	}
	if svc.RawDBForTest() == nil {
		t.Error("RawDBForTest 应返回底层连接")
	}
	_ = db
	var alive int
	n := svc.CountAliveForTest(context.Background())
	_ = n
	alive = n
	_ = alive
}
