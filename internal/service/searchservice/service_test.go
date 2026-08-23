// T4.3 service 侧验收：可见性过滤、二次校验、存活跳过。
package searchservice

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"element-wiki/internal/database"
	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/search"

	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"
	"element-wiki/migrations"
)

type fakeQuery struct {
	hits []search.Hit
}

func (f *fakeQuery) Query(_ context.Context, _ string, limit int) ([]search.Hit, error) {
	out := []search.Hit{}
	for _, h := range f.hits {
		if len(out) == limit {
			break
		}
		out = append(out, h)
	}
	return out, nil
}

func newSvc(t *testing.T) (*Service, *fakeQuery, *sql.DB, map[string]string) {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := &migrations.Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,created_at)
		VALUES ('ed','i','ed','','Editor',1)`); err != nil {
		t.Fatal(err)
	}
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	editor := permission.NewActor("ed", permission.CodesFor(permission.Editor))

	pub, cerr1 := svc.CreateDocument(context.Background(), editor, nil, "open-doc", "公开")
	if cerr1 != nil {
		t.Fatalf("create open: %v", cerr1)
	}
	if _, cerr3 := svc.Commit(context.Background(), editor, pub.ID, "", "needle public", "m"); cerr3 != nil {
		t.Fatalf("commit pub: %v", cerr3)
	}
	sec, cerr2 := svc.CreateDocument(context.Background(), editor, nil, "closed-doc", "机密")
	if cerr2 != nil {
		t.Fatalf("create sec: %v", cerr2)
	}
	svc.SetVisibility(context.Background(), editor, sec.ID, model.VisibilityRestricted)
	svc.Commit(context.Background(), editor, sec.ID, "", "needle secret", "m")

	ids := map[string]string{}
	rows, rerr := db.Query(`SELECT slug, id FROM documents`)
	if rerr == nil {
		for rows.Next() {
			var sl, di string
			rows.Scan(&sl, &di)
			ids[sl] = di
		}
		rows.Close()
	}
	fq := &fakeQuery{}
	return New(fq, impl, impl), fq, db, ids
}

func editorOf() permission.Actor {
	return permission.NewActor("ed", permission.CodesFor(permission.Editor))
}

func TestSearchFiltersRestrictedForViewer(t *testing.T) {
	svc, fq, _, ids := newSvc(t)
	_ = ids
	viewer := permission.NewActor("v1", permission.CodesFor(permission.Viewer))
	fq.hits = []search.Hit{
		{DocumentID: ids["open-doc"], Score: 5},
		{DocumentID: ids["closed-doc"], Score: 4},
	}
	hits, err := svc.Search(context.Background(), viewer, "needle", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Title != "公开" {
		t.Fatalf("viewer 应仅见公开文档: %+v", hits)
	}
}

func TestSearchSeesAllForEditor(t *testing.T) {
	svc, fq, _, ids := newSvc(t)
	_ = ids
	fq.hits = []search.Hit{
		{DocumentID: ids["open-doc"], Score: 5},
		{DocumentID: ids["closed-doc"], Score: 4},
	}
	hits, err := svc.Search(context.Background(), editorOf(), "needle", 10)
	if err != nil || len(hits) != 2 {
		t.Fatalf("editor 应命中 2 篇: %+v %v", hits, err)
	}
}

func TestSearchSkipsTrashedAndMissing(t *testing.T) {
	svc, fq, db, ids := newSvc(t)
	// open-doc 进回收站；ghost 从未存在
	db.Exec(`UPDATE documents SET deleted_at=1 WHERE slug='open-doc'`)
	fq.hits = []search.Hit{
		{DocumentID: ids["open-doc"], Score: 5},
		{DocumentID: "ghost", Score: 4},
	}
	hits, err := svc.Search(context.Background(), editorOf(), "needle", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("回收站与缺失文档应被跳过: %+v", hits)
	}
}

func TestSearchPermissionDenied(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	anonOff := permission.Anonymous(false)
	if _, err := svc.Search(context.Background(), anonOff, "x", 5); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("无读权限应拒绝: %v", err)
	}
}

var _ = sql.ErrNoRows

func (s *fixture) idBySlug(t *testing.T, slug string) string { return "" }

type fixture struct{}
