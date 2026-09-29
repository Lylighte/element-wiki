package sqlite

import (
	"context"
	"errors"
	"testing"

	store "element-wiki/internal/database"
	"element-wiki/internal/model"
)

func TestUserPageReviewKeepsPublishedContentUntilApproval(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, s)
	if _, err := s.db.Exec(`INSERT INTO users(id,issuer,subject,display_name,created_at) VALUES('admin','i','admin','Admin',1)`); err != nil {
		t.Fatal(err)
	}
	first := &model.UserPageRevision{ID: "r1", UserID: "u1", Content: "published v1", CreatedBy: "u1", CreatedAt: 10}
	if err := s.SubmitUserPageRevision(ctx, first, false); err != nil {
		t.Fatal(err)
	}
	p, err := s.GetUserPage(ctx, "u1")
	if err != nil || p.Published == nil || p.Published.Content != "published v1" {
		t.Fatalf("initial published page: %+v %v", p, err)
	}
	pending := &model.UserPageRevision{ID: "r2", UserID: "u1", Content: "pending v2", CreatedBy: "u1", CreatedAt: 20}
	if err := s.SubmitUserPageRevision(ctx, pending, true); err != nil {
		t.Fatal(err)
	}
	p, err = s.GetUserPage(ctx, "u1")
	if err != nil || p.Published.Content != "published v1" || p.Pending == nil || p.Pending.Content != "pending v2" {
		t.Fatalf("pending update replaced published page: %+v %v", p, err)
	}
	if err := s.ReviewUserPageRevision(ctx, "u1", "r2", "u1", "approve", "self review", 30); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("self review should be rejected: %v", err)
	}
	if err := s.ReviewUserPageRevision(ctx, "u1", "r2", "admin", "approve", "approved", 31); err != nil {
		t.Fatal(err)
	}
	p, err = s.GetUserPage(ctx, "u1")
	if err != nil || p.Published.Content != "pending v2" || p.Pending != nil {
		t.Fatalf("approved revision not promoted: %+v %v", p, err)
	}
	var actions int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM moderation_actions WHERE content_type='user_page'`).Scan(&actions); err != nil || actions != 1 {
		t.Fatalf("moderation audit missing: count=%d err=%v", actions, err)
	}
}
