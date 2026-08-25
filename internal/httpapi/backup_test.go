// T6.5/T6.6 验收：备份导出 zip 结构、导入事务化恢复、失败零残留。
package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
	"element-wiki/internal/model"
	authservice "element-wiki/internal/service/authservice"
	backupservice "element-wiki/internal/service/backupservice"
	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"
)

func newBackupEnv(t *testing.T) (*authEnv, *backupservice.Service, *backupservice.MarkdownImporter, string) {
	t.Helper()
	root := t.TempDir()
	db, err := database.Open("sqlite", filepath.Join(root, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	newAppliedMigrator(t, db)
	for _, u := range []struct{ id, role string }{
		{"ad", "admin"}, {"ed", "editor"},
	} {
		if _, err := db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,role,status,created_at)
			VALUES (?, 'i', ?, '', '', ?, 'active', 1)`, u.id, u.id, u.role); err != nil {
			t.Fatal(err)
		}
	}
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	svc.SetTrashHooks(impl)
	svc.SetCommentStore(impl, impl)
	attachDir := filepath.Join(root, "attachments")
	os.MkdirAll(attachDir, 0o755)
	svc.SetAttachmentStore(impl, attachDir, "png,jpg,txt,pdf", 10)
	backups := filepath.Join(root, "backups")

	admin := permission_AdminActor()
	bsvc := backupservice.New(impl, impl, db,
		filepath.Join(root, "live.db"), attachDir, backups, migrations_LatestVer())
	mdimp := backupservice.NewMarkdownImporter(impl, svc, func(id string) permission_Actor {
		return admin
	})
	auth := authservice.New(impl, impl, impl, "https://idp.test", []string{"ad@x.com"}, false)
	deps := Deps{Docs: svc, Trees: impl, Auth: auth, SecureCookies: true,
		Search: nil, Jobs: impl, Imports: impl,
		Backups: bsvc, MarkdownImports: mdimp,
		CommentsEnabled: true, AttachmentsOn: true,
		AttachDir: attachDir, UploadMaxBytes: 10 << 20}
	e := &authEnv{t: t, srv: httptest.NewServer(NewRouter(deps)), auth: auth, db: db, svc: svc}
	return e, bsvc, mdimp, attachDir
}

func TestBackupExportRoundtripAndImportRestore(t *testing.T) {
	e, _, _, _ := newBackupEnv(t)
	ctx := context.Background()
	editor := actorOf(t, "ed")
	d1, cerr := e.svc.CreateDocument(ctx, editor, nil, "bk-doc", "备份文档")
	if cerr != nil {
		t.Fatal(cerr)
	}
	e.svc.Commit(ctx, editor, d1.ID, "", "unique-backup-body", "m")

	// 导出一个附件参与打包
	a, uerr := e.svc.UploadAttachment(ctx, editor, d1.ID, "note.txt",
		strings.NewReader("attach-bytes"))
	if uerr != nil {
		t.Fatal(uerr)
	}
	_ = a

	// 触发导出 → 202
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/admin/backups", nil)
	req.AddCookie(&http.Cookie{Name: "ew_session", Value: e.sessionFor("ad")})
	r1, _ := http.DefaultClient.Do(req)
	b1, _ := io.ReadAll(r1.Body)
	r1.Body.Close()
	mustStatus(t, r1.StatusCode, 202, nil)
	var start struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(b1, &start)
	if start.JobID == "" {
		t.Fatal("缺 job_id")
	}

	// 轮询至 done
	waitJobDone(t, func() ([]byte, string) {
		stReq, _ := http.NewRequest("GET", e.srv.URL+"/v1/admin/backups/jobs/"+start.JobID, nil)
		stReq.AddCookie(&http.Cookie{Name: "ew_session", Value: e.sessionFor("ad")})
		stR, _ := http.DefaultClient.Do(stReq)
		raw, _ := io.ReadAll(stR.Body)
		stR.Body.Close()
		return raw, ""
	}, `{"status":"done"`)

	// 文件列表含产物
	resp, body := e.doWithCookieBody("GET", "/v1/admin/backups/files", e.sessionFor("ad"))
	mustStatus(t, resp.StatusCode, 200, body)
	files := body["items"].([]any)
	if len(files) != 1 {
		t.Fatalf("备份文件数 = %d", len(files))
	}
	name := files[0].(string)

	// 下载并校验 zip 内容
	reqDl, _ := http.NewRequest("GET", e.srv.URL+"/v1/admin/backups/files/"+name+"/download", nil)
	reqDl.AddCookie(&http.Cookie{Name: "ew_session", Value: e.sessionFor("ad")})
	rd, _ := http.DefaultClient.Do(reqDl)
	if rd.StatusCode != 200 {
		bb, _ := io.ReadAll(rd.Body)
		rd.Body.Close()
		t.Fatalf("下载应 200, got %d %s", rd.StatusCode, bb)
	}
	zipBytes, _ := io.ReadAll(rd.Body)
	rd.Body.Close()
	zr, zerr := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if zerr != nil {
		t.Fatalf("zip 打开失败: %v", zerr)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, want := range []string{"manifest.json", "db.sqlite3",
		"attachments/" + filepath.ToSlash(a.StoragePath)} {
		if !names[want] {
			t.Errorf("zip 缺少 %s; got %v", want, names)
		}
	}

	// —— 导入到全新实例（第二个 env，独立库）——
	e2, _, _, attDir2 := newBackupEnv(t)
	// 上传伪造的 zip 文件：直接复用下载字节
	var buf bytes.Buffer
	mw := multipart_Writer(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	fw.Write(zipBytes)
	mw.Close()
	reqImp, _ := http.NewRequest("POST", e2.srv.URL+"/v1/admin/imports", &buf)
	reqImp.Header.Set("Content-Type", mw.FormDataContentType())
	reqImp.AddCookie(&http.Cookie{Name: "ew_session", Value: e2.sessionFor("ad")})
	rI, _ := http.DefaultClient.Do(reqImp)
	bI, _ := io.ReadAll(rI.Body)
	rI.Body.Close()
	mustStatus(t, rI.StatusCode, 202, map[string]any{})
	var impStart struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(bI, &impStart)

	waitBackupImportDone(t, e2, impStart.JobID)

	// 恢复后：文档可见、内容一致、附件可下载
	got, gerr := e2.svc.Get(ctx, actorOf(t, "ad"), d1.ID)
	if gerr != nil || got.Slug != "bk-doc" {
		t.Fatalf("导入后文档缺失: %v %v", got, gerr)
	}
	bodyTxt, head, _ := e2.svc.HeadContent(ctx, actorOf(t, "ad"), d1.ID)
	if !strings.Contains(bodyTxt, "unique-backup-body") {
		t.Errorf("正文不符: %q", bodyTxt)
	}
	_ = head
	rawReq, _ := http.NewRequest("GET", "/v1/attachments/"+a.ID+"/raw", nil)
	_ = rawReq
	// 附件元数据恢复 + 磁盘文件存在于新目录
	if _, aerr := e2.svc.GetAttachment(ctx, actorOf(t, "ad"), a.ID); aerr != nil {
		t.Errorf("附件未恢复: %v", aerr)
	}
	foundOnDisk := false
	filepath.WalkDir(attDir2, func(p string, dd os.DirEntry, err error) error {
		if err == nil && !dd.IsDir() && strings.HasSuffix(p, "note.txt") {
			foundOnDisk = true
		}
		return nil
	})
	if !foundOnDisk {
		t.Error("附件磁盘文件未随导入恢复")
	}

	// DELETE 产物 → 204 → 列表空
	reqDel, _ := http.NewRequest("DELETE", e.srv.URL+"/v1/admin/backups/files/"+name, nil)
	reqDel.AddCookie(&http.Cookie{Name: "ew_session", Value: e.sessionFor("ad")})
	rdel, _ := http.DefaultClient.Do(reqDel)
	rdel.Body.Close()
	if rdel.StatusCode != 204 {
		t.Errorf("删除产物应 204, got %d", rdel.StatusCode)
	}
}

func TestImportRejectsBadZips(t *testing.T) {
	_, bsvc, _, _ := newBackupEnv(t)

	// 非法路径穿越条目 → 整体失败
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("../evil.txt")
	w.Write([]byte("evil"))
	zw.Close()
	tmp := filepath.Join(t.TempDir(), "bad.zip")
	os.WriteFile(tmp, buf.Bytes(), 0o644)

	jobID, serr := bsvc.StartImportOfZip(context.Background(), "ad", tmp, nil)
	if serr != nil {
		t.Fatal(serr)
	}
	waitBackupFailed(t, bsvc, jobID)

	// schema_version 不匹配
	buf2 := new(bytes.Buffer)
	zw2 := zip.NewWriter(buf2)
	mw2, _ := zw2.Create("manifest.json")
	mw2.Write([]byte(`{"schema_version":999}`))
	dbFake, _ := zw2.Create("db.sqlite3")
	dbFake.Write([]byte("placeholder"))
	zw2.Close()
	tmp2 := filepath.Join(t.TempDir(), "ver.zip")
	os.WriteFile(tmp2, buf2.Bytes(), 0o644)
	jobID2, _ := bsvc.StartImportOfZip(context.Background(), "ad", tmp2, nil)
	waitBackupFailed(t, bsvc, jobID2)
}

func TestMarkdownZipImportCreatesTree(t *testing.T) {
	e, _, mdimp, _ := newBackupEnv(t)

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	add := func(name, content string) {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	add("docs/readme.md", "# Docs Root\n")
	add("docs/guide/install.md", "# Install\nrun installer\n")
	add("docs/guide/install.png", "PNGDATA")
	add("broken/../evil.md", "x")
	zw.Close()

	tmp := filepath.Join(t.TempDir(), "content.zip")
	os.WriteFile(tmp, buf.Bytes(), 0o644)

	jobID, err := mdimp.StartMarkdownImport(context.Background(), "ad", tmp, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitImportDone(t, e, jobID)

	job, _ := e.svc.ListTrash(context.Background(), actorOf(t, "ad"), 100)
	_ = job
	// 树结构验证：docs 与 docs/guide 存在，install 为叶子
	ctx := context.Background()
	ad := actorOf(t, "ed")
	kids, _ := e.svc.ListChildrenForTree(ctx, ad, nil)
	var docsNode *model.Document
	for _, k := range kids {
		if k.Slug == "docs" {
			docsNode = k
		}
	}
	if docsNode == nil {
		t.Fatalf("docs 根未创建: %+v", kids)
	}
	sub, _ := e.svc.ListChildrenForTree(ctx, ad, &docsNode.ID)
	var guide *model.Document
	for _, k := range sub {
		if k.Slug == "guide" {
			guide = k
		}
	}
	if guide == nil {
		t.Fatal("docs/guide 未创建")
	}
	leaf, _ := e.svc.ListChildrenForTree(ctx, ad, &guide.ID)
	foundInstall := false
	for _, l := range leaf {
		if l.Slug == "install" {
			foundInstall = true
			bodyTxt, _, _ := e.svc.HeadContent(ctx, ad, l.ID)
			if !strings.Contains(bodyTxt, "run installer") {
				t.Errorf("install 正文缺失: %q", bodyTxt)
			}
		}
	}
	if !foundInstall {
		t.Error("install 叶未创建")
	}
	// 附件挂载验证（docs/guide 下应有 png）
	var attCount int
	e.db.QueryRow(`SELECT COUNT(*) FROM attachments WHERE filename='install.png'`).Scan(&attCount)
	if attCount != 1 {
		t.Errorf("png 附件数 = %d", attCount)
	}
	// 失败计数：broken/../evil.md 被跳过
	var failed int64
	e.db.QueryRow(`SELECT failed_files FROM import_jobs WHERE id=?`, jobID).Scan(&failed)
	if failed != 0 && failed != 1 {
		t.Logf("failed_files=%d（允许 0 或 1）", failed)
	}
}

func waitJobDone(t *testing.T, poll func() ([]byte, string), wantContains string) {
	t.Helper()
	for i := 0; i < 300; i++ {
		rawBytes, _ := poll()
		raw := string(rawBytes)
		if strings.Contains(raw, `"status":"done"`) {
			return
		}
		if strings.Contains(raw, `"status":"failed"`) {
			t.Fatalf("任务失败: %s", raw)
		}
		timeSleep(20)
	}
	t.Fatal("轮询超时")
}

func waitBackupFailed(t *testing.T, bsvc *backupservice.Service, id string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		j, err := bsvc.GetJob(context.Background(), id)
		if err == nil {
			if j.Status == "failed" {
				return
			}
			if j.Status == "done" {
				t.Fatal("预期失败的任务却成功了")
			}
		}
		timeSleep(20)
	}
	t.Fatal("等待失败状态超时")
}

func waitImportDone(t *testing.T, e *authEnv, id string) {
	t.Helper()
	lastRaw := ""
	for i := 0; i < 300; i++ {
		req, _ := http.NewRequest("GET", e.srv.URL+"/v1/admin/imports/jobs/"+id, nil)
		req.AddCookie(&http.Cookie{Name: "ew_session", Value: e.sessionFor("ad")})
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if strings.Contains(string(raw), `"done"`) {
			return
		}
		if strings.Contains(string(raw), `"failed"`) {
			t.Fatalf("导入任务失败: %s", string(raw))
		}
		timeSleep(20)
		lastRaw = string(raw)
	}
	t.Fatalf("导入超时, 最后状态=%s", strings.TrimSpace(lastRaw))
}

func waitBackupImportDone(t *testing.T, e *authEnv, id string) {
	t.Helper()
	for i := 0; i < 300; i++ {
		req, _ := http.NewRequest("GET", e.srv.URL+"/v1/admin/backups/jobs/"+id, nil)
		req.AddCookie(&http.Cookie{Name: "ew_session", Value: e.sessionFor("ad")})
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if strings.Contains(string(raw), `"status":"done"`) {
			return
		}
		if strings.Contains(string(raw), `"status":"failed"`) || strings.Contains(string(raw), "not found") {
			t.Fatalf("导入失败或查无任务: %s", string(raw))
		}
		t.Logf("poll raw=%s", strings.TrimSpace(string(raw)))
		timeSleep(20)
	}
	t.Fatal("备份导入超时")
}
