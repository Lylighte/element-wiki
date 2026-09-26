package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadSiteIconAndPublicRead(t *testing.T) {
	e, _ := newAdminEnv(t)
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("file", "brand.png")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3}
	if _, err := file.Write(png); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, e.srv.URL+"/v1/admin/site/icon", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "ew_session", Value: e.sessionFor("ad")})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status=%d", resp.StatusCode)
	}
	var uploaded struct {
		SiteIconURL string `json:"site_icon_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.SiteIconURL == "" {
		t.Fatal("upload response has no public URL")
	}

	publicReq, _ := http.NewRequest(http.MethodGet, e.srv.URL+uploaded.SiteIconURL, nil)
	publicResp, err := http.DefaultClient.Do(publicReq)
	if err != nil {
		t.Fatal(err)
	}
	defer publicResp.Body.Close()
	got, err := io.ReadAll(publicResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if publicResp.StatusCode != http.StatusOK || publicResp.Header.Get("Content-Type") != "image/png" || !bytes.Equal(got, png) {
		t.Fatalf("public icon response status=%d type=%q body=%v", publicResp.StatusCode, publicResp.Header.Get("Content-Type"), got)
	}
}

func TestUploadSiteIconRejectsInvalidFormatAndPermission(t *testing.T) {
	e, _ := newAdminEnv(t)
	for _, tc := range []struct {
		user, filename, data string
		want                 int
	}{
		{"ad", "bad.png", "not png", http.StatusUnprocessableEntity},
		{"ed", "good.png", "\x89PNG\r\n\x1a\n", http.StatusForbidden},
	} {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		f, _ := form.CreateFormFile("file", tc.filename)
		_, _ = f.Write([]byte(tc.data))
		_ = form.Close()
		req, err := http.NewRequest(http.MethodPost, e.srv.URL+"/v1/admin/site/icon", &body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", form.FormDataContentType())
		req.AddCookie(&http.Cookie{Name: "ew_session", Value: e.sessionFor(tc.user)})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("%s status=%d want=%d", tc.user, resp.StatusCode, tc.want)
		}
	}
}

func TestUploadSiteIconUsesCurrentAttachmentSizeLimit(t *testing.T) {
	e, _ := newAdminEnv(t)
	resp, body := e.doJSON(http.MethodPatch, "/v1/admin/settings", e.sessionFor("ad"), map[string]string{"upload_max_mb": "1"})
	mustStatus(t, resp.StatusCode, http.StatusOK, body)
	content := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, bytes.Repeat([]byte{1}, (1<<20))...)
	var requestBody bytes.Buffer
	form := multipart.NewWriter(&requestBody)
	f, _ := form.CreateFormFile("file", "large.png")
	_, _ = f.Write(content)
	_ = form.Close()
	req, err := http.NewRequest(http.MethodPost, e.srv.URL+"/v1/admin/site/icon", &requestBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "ew_session", Value: e.sessionFor("ad")})
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("upload status=%d want=%d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
	if _, err := os.Stat(filepath.Join(e.svc.AttachDir(), "_site-icons")); !os.IsNotExist(err) {
		t.Fatalf("oversized upload left icon storage behind: %v", err)
	}
}
