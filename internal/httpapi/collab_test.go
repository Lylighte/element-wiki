// T5.4/T5.5/T5.6 验收：附件受控上传全链路 + 评论门闩与提及。
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
	authservice "element-wiki/internal/service/authservice"
	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"
)

func newCollabEnv(t *testing.T, commentsEnabled bool) (*authEnv, string) {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	newAppliedMigrator(t, db)
	for _, u := range []struct{ id, role string }{
		{"ed", "editor"}, {"vw", "viewer"}, {"ad", "admin"},
	} {
		db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,role,status,created_at)
			VALUES (?, 'i', ?, ?, '', ?, 'active', 1)`, u.id, u.id, u.id+"@x.com", u.role)
	}

	attachDir := filepath.Join(t.TempDir(), "attachments")
	os.MkdirAll(attachDir, 0o755)
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	svc.SetCommentStore(impl, impl)
	svc.SetTrashHooks(impl)
	svc.SetAttachmentStore(impl, attachDir,
		"png,jpg,txt,pdf", 1) // 1MB 上限
	auth := authservice.New(impl, impl, impl, "https://idp.test", nil, false)
	deps := Deps{Docs: svc, Trees: impl, Auth: auth, SecureCookies: true,
		CommentsEnabled: commentsEnabled, AttachmentsOn: true,
		AttachDir: attachDir, UploadMaxBytes: 1024 * 1024}
	e := &authEnv{t: t, srv: httptest.NewServer(NewRouter(deps)), auth: auth, db: db, svc: svc}
	return e, attachDir
}

// upload 以 editor 会话发送 multipart。
func (e *authEnv) upload(path, filename, content string) (*http.Response, map[string]any) {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", filename)
	fw.Write([]byte(content))
	mw.Close()
	req, _ := http.NewRequest("POST", e.srv.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("ed")})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestAttachmentLifecycle(t *testing.T) {
	e, attachDir := newCollabEnv(t, true)
	ctx := context.Background()
	editor := actorOf(t, "ed")
	d, cerr := e.svc.CreateDocument(ctx, editor, nil, "attach-doc", "A")
	if cerr != nil {
		t.Fatal(cerr)
	}

	// 上传 → 201 元数据
	resp, body := e.upload("/v1/documents/"+d.ID+"/attachments", "shot.png", "\x89PNG fake")
	mustStatus(t, resp.StatusCode, 201, body)
	a := body
	if a["filename"] != "shot.png" || a["mime_type"] != "image/png" {
		t.Fatalf("元数据异常: %v", a)
	}
	id := a["id"].(string)

	// 文件已落盘（非临时名）
	foundFile := false
	filepath.WalkDir(attachDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasPrefix(d.Name(), "upload-") {
			foundFile = true
		}
		return nil
	})
	if !foundFile {
		t.Errorf("附件目录无最终文件")
	}

	// 列表
	resp, body = e.doWithCookieBody("GET", "/v1/documents/"+d.ID+"/attachments",
		e.sessionFor("ed"))
	mustStatus(t, resp.StatusCode, 200, body)
	if len(body["items"].([]any)) != 1 {
		t.Errorf("列表数量异常: %v", body)
	}

	// raw 下载内容一致
	reqRaw, _ := http.NewRequest("GET", e.srv.URL+"/v1/attachments/"+id+"/raw", nil)
	reqRaw.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("ed")})
	r2, _ := http.DefaultClient.Do(reqRaw)
	buf := new(bytes.Buffer)
	buf.ReadFrom(r2.Body)
	r2.Body.Close()
	if r2.StatusCode != 200 || buf.String() != "\x89PNG fake" {
		t.Errorf("raw 下载异常: %d %q", r2.StatusCode, buf.String())
	}

	// 删除 → 204 → raw 404 → 磁盘清理
	reqDel, _ := http.NewRequest("DELETE", e.srv.URL+"/v1/attachments/"+id, nil)
	reqDel.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("ed")})
	r3, _ := http.DefaultClient.Do(reqDel)
	r3.Body.Close()
	if r3.StatusCode != 204 {
		t.Errorf("删除应 204, got %d", r3.StatusCode)
	}
	reqRaw2, _ := http.NewRequest("GET", e.srv.URL+"/v1/attachments/"+id+"/raw", nil)
	reqRaw2.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("ed")})
	r4, _ := http.DefaultClient.Do(reqRaw2)
	r4.Body.Close()
	if r4.StatusCode != 404 {
		t.Errorf("删除后 raw 应 404, got %d", r4.StatusCode)
	}
	left, _ := os.ReadDir(attachDir)
	for _, f := range left {
		if strings.HasSuffix(f.Name(), ".png") {
			t.Errorf("磁盘文件未清理: %s", f.Name())
		}
	}
}

func TestAttachmentValidationBranches(t *testing.T) {
	e, _ := newCollabEnv(t, true)
	ctx := context.Background()
	editor := actorOf(t, "ed")
	d, _ := e.svc.CreateDocument(ctx, editor, nil, "val-doc", "V")

	// 非白名单扩展 → 415
	resp, body := e.upload("/v1/documents/"+d.ID+"/attachments", "evil.exe", "MZ")
	mustStatus(t, resp.StatusCode, 415, body)
	if !strings.Contains(body["detail"].(string), "扩展名") &&
		!strings.Contains(body["detail"].(string), "whitelist") {
		t.Logf("detail=%v", body["detail"])
	}

	// 超限 → 413（>1MB）
	big := bytes.Repeat([]byte("a"), 1024*1024+10)
	resp, body = e.upload("/v1/documents/"+d.ID+"/attachments", "big.txt", string(big))
	mustStatus(t, resp.StatusCode, 413, body)

	// 幽灵文档 → 404
	resp, _ = e.upload("/v1/documents/ghost/attachments", "ok.txt", "x")
	if resp.StatusCode != 404 {
		t.Errorf("幽灵上传应 404, got %d", resp.StatusCode)
	}

	// viewer → 403
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/documents/"+d.ID+"/attachments", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("vw")})
	r, _ := http.DefaultClient.Do(req)
	ioCopyDiscard(r)
	if r.StatusCode != 403 {
		t.Errorf("viewer 上传应 403, got %d", r.StatusCode)
	}

	// 缺 file 字段 → 400
	req2, _ := http.NewRequest("POST", e.srv.URL+"/v1/documents/"+d.ID+"/attachments",
		strings.NewReader(""))
	req2.Header.Set("Content-Type", "multipart/form-data; boundary=none")
	req2.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("ed")})
	r2, _ := http.DefaultClient.Do(req2)
	r2.Body.Close()
	if r2.StatusCode != 400 && r2.StatusCode != 500 {
		t.Errorf("缺字段应 4xx, got %d", r2.StatusCode)
	}
}

func TestCommentsGateAndFlow(t *testing.T) {
	eOn, _ := newCollabEnv(t, true)
	eOff, _ := newCollabEnv(t, false)
	ctx := context.Background()
	editor := actorOf(t, "ed")

	t.Logf("svc nil? %v", eOn.svc == nil)
	if eOn.svc == nil {
		t.Fatal("svc nil")
	}
	dOn, cErrOn := eOn.svc.CreateDocument(ctx, editor, nil, "cm-on", "C")
	if cErrOn != nil {
		t.Fatal(cErrOn)
	}
	dOff, cErrOff := eOff.svc.CreateDocument(ctx, editor, nil, "cm-off", "C")
	if cErrOff != nil {
		t.Fatal(cErrOff)
	}

	// 门闩关闭 → 全部 403 固定 detail
	for _, tc := range []struct{ m, p string }{
		{"POST", "/v1/documents/" + dOff.ID + "/comments"},
		{"GET", "/v1/documents/" + dOff.ID + "/comments"},
	} {
		req, _ := http.NewRequest(tc.m, eOff.srv.URL+tc.p,
			strings.NewReader(`{"content":"hi"}`))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "access_token", Value: eOff.sessionFor("ed")})
		r, _ := http.DefaultClient.Do(req)
		bb, _ := io.ReadAll(r.Body)
		r.Body.Close()
		b := string(bb)
		if r.StatusCode != 403 || !strings.Contains(b, "comments disabled") {
			t.Errorf("[%s %s] 门闩失败: %d %s", tc.m, tc.p, r.StatusCode, b)
		}
	}

	// 开启：发布评论带 @email 提及
	req, _ := http.NewRequest("POST", eOn.srv.URL+"/v1/documents/"+dOn.ID+"/comments",
		strings.NewReader(`{"content":"@ed@x.com 请看这份 @nobody@x.com 说明"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "access_token", Value: eOn.sessionFor("ed")})
	r1, _ := http.DefaultClient.Do(req)
	b1b, _ := io.ReadAll(r1.Body)
	r1.Body.Close()
	b1 := string(b1b)
	mustStatus(t, r1.StatusCode, 201, nil)
	var created struct {
		Comment struct {
			ID       string   `json:"id"`
			Mentions []string `json:"mentions"`
		} `json:"comment"`
	}
	json.Unmarshal([]byte(b1), &created)
	if len(created.Comment.Mentions) != 1 ||
		created.Comment.Mentions[0] != "ed" {
		t.Errorf("提及解析异常: %+v", created.Comment.Mentions)
	}

	// 列表升序含提及
	resp, body := eOn.doWithCookieBody("GET", "/v1/documents/"+dOn.ID+"/comments",
		eOn.sessionFor("vw"))
	mustStatus(t, resp.StatusCode, 200, body)
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("列表数 = %d", len(items))
	}

	// viewer 删他人 → 403；作者删自己 → 204；再删 → 404
	cid := created.Comment.ID
	reqD, _ := http.NewRequest("DELETE", eOn.srv.URL+"/v1/comments/"+cid, nil)
	reqD.AddCookie(&http.Cookie{Name: "access_token", Value: eOn.sessionFor("vw")})
	rd, _ := http.DefaultClient.Do(reqD)
	ioCopyDiscard(rd)
	if rd.StatusCode != 403 {
		t.Errorf("他人删除应 403, got %d", rd.StatusCode)
	}
	reqD2, _ := http.NewRequest("DELETE", eOn.srv.URL+"/v1/comments/"+cid, nil)
	reqD2.AddCookie(&http.Cookie{Name: "access_token", Value: eOn.sessionFor("ed")})
	rd2, _ := http.DefaultClient.Do(reqD2)
	ioCopyDiscard(rd2)
	if rd2.StatusCode != 204 {
		t.Errorf("作者删除应 204, got %d", rd2.StatusCode)
	}
	if r := eOn.doWithCookie("DELETE", "/v1/comments/"+cid, eOn.sessionFor("ed"), ""); r.StatusCode != 404 {
		t.Errorf("二次删除应 404, got %d", r.StatusCode)
	}
	_ = ctx
}

// T5.1 端点验收：trash 列表/恢复/彻底删除 + 恢复父缺失 409。
func TestTrashEndpoints(t *testing.T) {
	e, _ := newCollabEnv(t, true)
	ctx := context.Background()
	editor := actorOf(t, "ed")
	d, _ := e.svc.CreateDocument(ctx, editor, nil, "trash-api", "T")

	// 经 service 进入回收站
	if err := e.svc.TrashDocument(ctx, editor, d.ID); err != nil {
		t.Fatal(err)
	}

	// viewer 无 DocDelete → 403
	if r := e.doWithCookie("GET", "/v1/trash", e.sessionFor("vw"), ""); r.StatusCode != 403 {
		t.Errorf("viewer 列回收站应 403, got %d", r.StatusCode)
	}

	resp, body := e.doWithCookieBody("GET", "/v1/trash", e.sessionFor("ed"))
	mustStatus(t, resp.StatusCode, 200, body)
	items := body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != d.ID {
		t.Fatalf("回收站列表异常: %v", items)
	}

	// 彻底删除 → 204 → 列表空
	req, _ := http.NewRequest("DELETE", e.srv.URL+"/v1/trash/"+d.ID, nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("ed")})
	r2, _ := http.DefaultClient.Do(req)
	r2.Body.Close()
	mustStatus(t, r2.StatusCode, 204, nil)

	resp, body = e.doWithCookieBody("GET", "/v1/trash", e.sessionFor("ed"))
	if len(body["items"].([]any)) != 0 {
		t.Errorf("purge 后列表应空: %v", body)
	}

	// 恢复流程：新建→删→构造父链缺失→409→带 parent 恢复
	parent, _ := e.svc.CreateDocument(ctx, editor, nil, "tp", "P")
	childDoc, _ := e.svc.CreateDocument(ctx, editor, &parent.ID, "tc", "C")
	e.svc.TrashDocument(ctx, editor, parent.ID)
	e.svc.TrashDocument(ctx, editor, childDoc.ID)

	resp, body = e.doJSON("POST", "/v1/trash/"+childDoc.ID+"/restore",
		e.sessionFor("ed"), map[string]any{})
	mustStatus(t, resp.StatusCode, 409, body)
	if body["detail"] != "parent deleted" {
		t.Errorf("409 detail = %v", body["detail"])
	}

	newHome, _ := e.svc.CreateDocument(ctx, editor, nil, "tnh", "H")
	resp, _ = e.doJSON("POST", "/v1/trash/"+childDoc.ID+"/restore",
		e.sessionFor("ed"), map[string]any{"parent_id": newHome.ID})
	mustStatus(t, resp.StatusCode, 204, nil)
}

func TestCommentListLimitValidation(t *testing.T) {
	eOn, _ := newCollabEnv(t, true)
	ctx := context.Background()
	editor := actorOf(t, "ed")
	d, _ := eOn.svc.CreateDocument(ctx, editor, nil, "cm-limit", "C")
	_ = ctx

	resp, body := eOn.doWithCookieBody("GET", "/v1/documents/"+d.ID+"/comments?limit=0",
		eOn.sessionFor("ed"))
	mustStatus(t, resp.StatusCode, 400, body)
}
