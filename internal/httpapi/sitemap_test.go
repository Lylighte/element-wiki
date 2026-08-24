// T5.7 验收：sitemap 仅含匿名可见 standard 文档；匿名关闭时 403。
package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"element-wiki/internal/model"
)

func TestSitemapVisibility(t *testing.T) {
	e, _ := newAnonEnv(t, true)
	ctx := context.Background()
	editor := actorOf(t, "ed")

	pub, cerr := e.svc.CreateDocument(ctx, editor, nil, "site-pub", "P")
	if cerr != nil {
		t.Fatal(cerr)
	}
	sec, cerr := e.svc.CreateDocument(ctx, editor, nil, "site-sec", "S")
	if cerr != nil {
		t.Fatal(cerr)
	}
	if err := e.svc.SetVisibility(ctx, editor, sec.ID, model.VisibilityRestricted); err != nil {
		t.Fatal(err)
	}

	r, _ := http.Get(e.srv.URL + "/sitemap.xml")
	raw := readAllBody(r)
	r.Body.Close()
	if r.StatusCode != 200 || !strings.Contains(raw, "/docs/"+pub.ID) {
		t.Fatalf("应包含公开文档: %d %s", r.StatusCode, raw)
	}
	if strings.Contains(raw, sec.ID) {
		t.Errorf("restricted 文档不得出现在 sitemap: %s", raw)
	}
}

func TestSitemapForbiddenWhenAnonymousOff(t *testing.T) {
	e, _ := newAnonEnv(t, false)
	r, _ := http.Get(e.srv.URL + "/sitemap.xml")
	readAllBody(r)
	if r.StatusCode != 403 {
		t.Errorf("匿名关闭时应 403, got %d", r.StatusCode)
	}
}
