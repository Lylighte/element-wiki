// T12.3 验收：export.md 内容等于 HEAD 源码；restricted 匿名 404 掩护。
package httpapi

import (
	"context"
	"io"
	"net/http"
	"testing"

	"element-wiki/internal/model"
)

func TestExportMarkdownEndpoint(t *testing.T) {
	e := newEnv(t)
	id := createDoc(t, e, "exp-md", "Exp MD", nil)

	resp, body := e.do("POST", "/v1/documents/"+id+"/commits", "editor",
		map[string]any{"base_commit_id": "", "content": "# exported\nbody text", "message": "m"})
	mustStatus(t, resp.StatusCode, 201, body)

	req, _ := http.NewRequest("GET", e.srv.URL+"/v1/documents/"+id+"/export.md", nil)
	req.Header.Set("X-Test-Role", "viewer")
	hresp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != 200 {
		t.Fatalf("status = %d", hresp.StatusCode)
	}
	if cd := hresp.Header.Get("Content-Disposition"); cd != `attachment; filename="exp-md.md"` {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	raw, _ := io.ReadAll(hresp.Body)
	if string(raw) != "# exported\nbody text" {
		t.Fatalf("内容不符: %q", string(raw))
	}

	// restricted + viewer（无扩展读权限）→ 404 掩护
	e.do("PATCH", "/v1/documents/"+id, "editor", map[string]any{"visibility": "restricted"})
	req2, _ := http.NewRequest("GET", e.srv.URL+"/v1/documents/"+id+"/export.md", nil)
	req2.Header.Set("X-Test-Role", "viewer")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 404 {
		t.Fatalf("restricted 对 viewer 应 404, got %d", resp2.StatusCode)
	}
}

func TestExportMarkdownAnonymous(t *testing.T) {
	e, svc := newAnonEnv(t, true)
	ctx := context.Background()
	editor := actorOf(t, "ed")

	d, cerr := svc.CreateDocument(ctx, editor, nil, "anon-exp", "Anon Exp")
	if cerr != nil {
		t.Fatal(cerr)
	}
	if _, cerr := svc.Commit(ctx, editor, d.ID, "", "anon body", "m"); cerr != nil {
		t.Fatal(cerr)
	}

	// standard 文档匿名可导出
	r1, err := http.Get(e.srv.URL + "/v1/documents/" + d.ID + "/export.md")
	if err != nil {
		t.Fatal(err)
	}
	defer r1.Body.Close()
	if r1.StatusCode != 200 {
		t.Fatalf("standard 匿名导出应 200, got %d", r1.StatusCode)
	}

	// restricted 匿名 → 404 掩护
	if cerr := svc.SetVisibility(ctx, editor, d.ID, model.VisibilityRestricted); cerr != nil {
		t.Fatal(cerr)
	}
	r2, err := http.Get(e.srv.URL + "/v1/documents/" + d.ID + "/export.md")
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	if r2.StatusCode != 404 {
		t.Fatalf("restricted 匿名应 404, got %d", r2.StatusCode)
	}
}
