package docservice

import (
	"context"
	"testing"

	"element-wiki/internal/database/sqlite"
	"element-wiki/internal/permission"
)

type documentReviewSettings struct{}

func (documentReviewSettings) IntSetting(context.Context, string, int64) int64 { return 100 }
func (documentReviewSettings) StrSetting(_ context.Context, key, fallback string) string {
	if key == "document_review_required" {
		return "true"
	}
	return fallback
}

func TestDocumentReviewDoesNotPublishUntilApproval(t *testing.T) {
	svc, db := newSvc(t)
	impl := sqlite.New(db)
	svc.SetDocumentSubmissionStore(impl)
	svc.SetSettingsSource(documentReviewSettings{})
	ctx := context.Background()
	admin := permission.NewActor("admin", permission.CodesFor(permission.Admin))
	if _, err := db.Exec(`INSERT INTO users(id,issuer,subject,email,display_name,role,status,created_at) VALUES('admin','i','admin','','Admin','admin','active',1)`); err != nil {
		t.Fatal(err)
	}
	doc, err := svc.CreateDocument(ctx, editor(), nil, "review-me", "Review me")
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Commit(ctx, editor(), doc.ID, "", "candidate body", "submit")
	if err != nil || !res.Pending || res.Commit != nil {
		t.Fatalf("commit should enter review: %+v %v", res, err)
	}
	got, _ := svc.Get(ctx, editor(), doc.ID)
	if got.HeadCommitID != "" {
		t.Fatalf("pending content advanced HEAD: %+v", got)
	}
	queue, err := svc.PendingDocumentSubmissions(ctx, admin, 50)
	if err != nil || len(queue) != 1 || queue[0].Content != "candidate body" {
		t.Fatalf("pending queue: %+v %v", queue, err)
	}
	if err := svc.ReviewDocumentSubmission(ctx, editor(), res.SubmissionID, "approve", ""); err == nil {
		t.Fatal("editor approved submission")
	}
	if err := svc.ReviewDocumentSubmission(ctx, admin, res.SubmissionID, "approve", "looks good"); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(ctx, editor(), doc.ID)
	if got.HeadCommitID == "" {
		t.Fatal("approved submission did not publish")
	}
	body, _, err := svc.HeadContent(ctx, editor(), doc.ID)
	if err != nil || body != "candidate body" {
		t.Fatalf("published body=%q err=%v", body, err)
	}
	var audited int
	if err := db.QueryRow(`SELECT COUNT(*) FROM moderation_actions WHERE content_type='document' AND action='approve'`).Scan(&audited); err != nil || audited != 1 {
		t.Fatalf("audit count=%d err=%v", audited, err)
	}
}
