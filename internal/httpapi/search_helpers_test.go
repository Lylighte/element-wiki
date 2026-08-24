package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
)

func nopSlog() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func actorOf(t *testing.T, userID string) permission.Actor {
	t.Helper()
	role := map[string]permission.Role{
		"ed": permission.Editor, "vw": permission.Viewer, "ad": permission.Admin,
	}[userID]
	return permission.NewActor(userID, permission.CodesFor(role))
}

func docVisibilityRestricted() model.Visibility { return model.VisibilityRestricted }

var _ = strings.Contains

// editorCreate 经 service 直建并提交一篇文档。
func editorCreate(e *authEnv, ctx context.Context, slug, title, body string) (*model.Document, error) {
	editor := actorOf(t_of(e), "ed")
	d, err := e.svc.CreateDocument(ctx, editor, nil, slug, title)
	if err != nil {
		return nil, err
	}
	if _, err := e.svc.Commit(ctx, editor, d.ID, "", body, "seed"); err != nil {
		return nil, err
	}
	return d, nil
}

func t_of(e *authEnv) *testing.T { return e.t }

func ioReadAllBody(r *http.Response) string {
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	return string(b)
}

func ioCopyDiscard(r *http.Response) {
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()
}

func (e *authEnv) doJSON(method, path, cookie string, body any) (*http.Response, map[string]any) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = strings.NewReader(string(b))
	} else {
		rd = strings.NewReader("")
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "ew_session", Value: cookie})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}
