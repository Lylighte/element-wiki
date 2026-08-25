// T11.1 验收：在线设置运行时即时生效——PATCH 后无需重启即改变行为。
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
	adminservice "element-wiki/internal/service/adminservice"
	authservice "element-wiki/internal/service/authservice"
	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"
	"element-wiki/migrations"
)

func newRuntimeEnv(t *testing.T) (*env, *httptest.Server) {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "rt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := (&migrations.Migrator{DB: db, Dialect: "sqlite"}).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"u1", "u2", "u3"} {
		db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,created_at)
			VALUES (?, 'i', ?, '', '', 1)`, u, u)
	}
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	svc.SetTrashHooks(impl)
	svc.SetAttachmentStore(impl, t.TempDir(), "png", 5) // config 值：5MB
	admin := adminservice.New(impl, impl, impl)
	svc.SetSettingsSource(admin)
	auth := authservice.New(impl, impl, impl, "https://idp.test", nil, false)
	deps := Deps{
		Docs: svc, Trees: impl, ActorFor: actorFor, Auth: auth, Admin: admin,
		SiteDefaults: SiteInfo{Title: "T", DefaultLang: "zh-CN", AnonymousRead: true},
	}
	srv := httptest.NewServer(NewRouter(deps))
	t.Cleanup(srv.Close)
	return &env{t: t, srv: srv}, srv
}

// uploadFile 以指定角色发送 multipart 文件上传。
func (e *env) uploadFile(path, role, filename string, content []byte) (*http.Response, map[string]any) {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", filename)
	fw.Write(content)
	mw.Close()
	req, _ := http.NewRequest("POST", e.srv.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Test-Role", role)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	dec := json.NewDecoder(resp.Body)
	_ = dec.Decode(&out)
	return resp, out
}

// 2MB 载荷（校验只看扩展名与大小，内容不敏感）
var bigPayload = bytes.Repeat([]byte{0x50}, 2*1024*1024)

func TestUploadLimitHotReload(t *testing.T) {
	e, _ := newRuntimeEnv(t)
	id := createDoc(t, e, "hot-limit", "Hot Limit", nil)

	// 初始 upload_max_mb=5（config）→ 大图上传成功
	resp, body := e.uploadFile("/v1/documents/"+id+"/attachments", "editor", "big.png", bigPayload)
	mustStatus(t, resp.StatusCode, 201, body)

	// PATCH upload_max_mb=1 → 无需重启即生效（缓存已失效）
	pResp, pBody := e.do("PATCH", "/v1/admin/settings", "admin",
		map[string]any{"upload_max_mb": "1"})
	mustStatus(t, pResp.StatusCode, 200, pBody)

	resp2, body2 := e.uploadFile("/v1/documents/"+id+"/attachments", "editor", "big.png", bigPayload)
	mustStatus(t, resp2.StatusCode, 413, body2)

	// 改回 5 → 恢复成功
	e.do("PATCH", "/v1/admin/settings", "admin", map[string]any{"upload_max_mb": "5"})
	resp3, _ := e.uploadFile("/v1/documents/"+id+"/attachments", "editor", "big.png", bigPayload)
	mustStatus(t, resp3.StatusCode, 201, map[string]any{})
}

func TestExtensionWhitelistHotReload(t *testing.T) {
	e, _ := newRuntimeEnv(t)
	id := createDoc(t, e, "hot-ext", "Hot Ext", nil)
	content := []byte(strings.Repeat("a", 64))

	// DB 种子白名单含 txt → 可传
	resp, _ := e.uploadFile("/v1/documents/"+id+"/attachments", "editor", "note.txt", content)
	mustStatus(t, resp.StatusCode, 201, map[string]any{})

	// 白名单收紧为仅 png → 立即拒绝（运行时生效）
	e.do("PATCH", "/v1/admin/settings", "admin",
		map[string]any{"allowed_extensions": "png"})
	resp2, _ := e.uploadFile("/v1/documents/"+id+"/attachments", "editor", "note.txt", content)
	mustStatus(t, resp2.StatusCode, 415, map[string]any{})

	e.do("PATCH", "/v1/admin/settings", "admin", map[string]any{"allowed_extensions": "png"})
	resp3, _ := e.uploadFile("/v1/documents/"+id+"/attachments", "editor", "note.txt", content)
	mustStatus(t, resp3.StatusCode, 415, map[string]any{})
}

func TestAnonReadProviderSwitch(t *testing.T) {
	auth := authservice.New(nil, nil, nil, "https://idp.test", nil, false)
	if auth.AnonymousEnabled() {
		t.Fatal("默认应关闭匿名")
	}
	auth.SetAnonReadProvider(func() bool { return true })
	if !auth.AnonymousEnabled() {
		t.Fatal("provider 注入后应即时生效")
	}
	actor := auth.AnonymousActor()
	if !actor.Has("document.read") {
		t.Fatal("匿名开启后应持有 document.read")
	}
}
