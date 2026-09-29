package userpageservice

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	store "element-wiki/internal/database"
	sqlitestore "element-wiki/internal/database/sqlite"
	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/migrations"
)

type testSettings map[string]string

func (s testSettings) StrSetting(_ context.Context, key, fallback string) string {
	if v, ok := s[key]; ok {
		return v
	}
	return fallback
}

func newTestService(t *testing.T) (*Service, *sqlitestore.DB, testSettings) {
	t.Helper()
	raw, err := store.Open("sqlite", filepath.Join(t.TempDir(), "pages.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if err := (&migrations.Migrator{DB: raw, Dialect: "sqlite"}).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	db := sqlitestore.New(raw)
	for _, u := range []model.User{{ID: "author", Issuer: "i", Subject: "a", Email: "secret@example.com", DisplayName: "Author", Role: permission.Viewer, Status: model.UserActive, CreatedAt: 1}, {ID: "admin", Issuer: "i", Subject: "b", DisplayName: "Admin", Role: permission.Admin, Status: model.UserActive, CreatedAt: 1}} {
		if err := db.CreateUser(context.Background(), &u); err != nil {
			t.Fatal(err)
		}
	}
	settings := testSettings{"user_pages_enabled": "true", "user_pages_review_required": "true"}
	svc := New(db, db, settings)
	svc.now = func() int64 { return 10 }
	return svc, db, settings
}

func TestUserPageReviewVisibilityAndPermissions(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	author := permission.NewActor("author", permission.CodesFor(permission.Viewer))
	admin := permission.NewActor("admin", permission.CodesFor(permission.Admin))
	anon := permission.Anonymous(true)
	p, err := svc.Save(ctx, author, "author", "# Public profile")
	if err != nil || p.Pending == nil || p.Published != nil {
		t.Fatalf("submission should wait for review: %+v %v", p, err)
	}
	public, err := svc.Get(ctx, anon, "author")
	if err != nil {
		t.Fatal(err)
	}
	if public.Pending != nil || public.Published != nil {
		t.Fatalf("pending content visible to public: %+v", public)
	}
	if err := svc.Review(ctx, author, "author", p.Pending.ID, "approve", ""); !errors.Is(err, permission.ErrDenied) {
		t.Fatalf("non-admin reviewed page: %v", err)
	}
	if err := svc.Review(ctx, admin, "author", p.Pending.ID, "approve", "looks good"); err != nil {
		t.Fatal(err)
	}
	public, err = svc.Get(ctx, anon, "author")
	if err != nil || public.Published == nil || public.Published.Content != "# Public profile" || public.DisplayName != "Author" {
		t.Fatalf("approved page not public or author identity wrong: %+v %v", public, err)
	}
	if public.Published != nil && public.Published.Content == "secret@example.com" {
		t.Fatal("OIDC email leaked")
	}
}

func TestUserPageDisabledAndSelfApproval(t *testing.T) {
	svc, _, settings := newTestService(t)
	ctx := context.Background()
	author := permission.NewActor("author", permission.CodesFor(permission.Viewer))
	settings["user_pages_enabled"] = "false"
	if _, err := svc.Save(ctx, author, "author", "x"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled page feature allowed write: %v", err)
	}
	settings["user_pages_enabled"] = "true"
	settings["user_pages_review_required"] = "false"
	p, err := svc.Save(ctx, author, "author", "page")
	if err != nil || p.Published == nil {
		t.Fatalf("unmoderated setting should publish immediately: %+v %v", p, err)
	}
}

func TestUserPageHistoryFiltersUnpublishedAndPreservesRestoreTarget(t *testing.T) {
	svc, db, settings := newTestService(t)
	ctx := context.Background()
	author := permission.NewActor("author", permission.CodesFor(permission.Viewer))
	anon := permission.Anonymous(true)
	settings["user_pages_review_required"] = "false"
	if _, err := svc.Save(ctx, author, "author", "published v1"); err != nil {
		t.Fatal(err)
	}
	settings["user_pages_review_required"] = "true"
	_, err := svc.Save(ctx, author, "author", "pending v2")
	if err != nil {
		t.Fatal(err)
	}
	publicHistory, err := svc.History(ctx, anon, "author", 50)
	if err != nil || len(publicHistory) != 1 || publicHistory[0].Content != "published v1" {
		t.Fatalf("public history leaked pending revision: %+v %v", publicHistory, err)
	}
	ownerHistory, err := svc.History(ctx, author, "author", 50)
	if err != nil || len(ownerHistory) != 2 {
		t.Fatalf("owner history missing revisions: %+v %v", ownerHistory, err)
	}
	if err := db.SetContentPublished(ctx, "user_page", "author", false, "admin", "moderation", 30); err != nil {
		t.Fatal(err)
	}
	settings["user_pages_review_required"] = "false"
	if _, err := svc.Save(ctx, author, "author", "corrected v3"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetContentPublished(ctx, "user_page", "author", true, "admin", "restored", 40); err != nil {
		t.Fatal(err)
	}
	public, err := svc.Get(ctx, anon, "author")
	if err != nil || public.Published == nil || public.Published.Content != "corrected v3" {
		t.Fatalf("restore lost newer revision: %+v %v", public, err)
	}
}
