// T12.2 验收：manifest 缺失整体失败零残留；导入成功自动入队全量索引重建。
package backupservice

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildZip 生成仅含给定条目的 zip。
func buildZip(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, data := range entries {
		w, _ := zw.Create(name)
		w.Write(data)
	}
	zw.Close()
}

// snapshotBytes 通过 VACUUM INTO 生成当前库一致性快照字节。
func (e *env) snapshotBytes(t *testing.T) []byte {
	t.Helper()
	p := filepath.Join(e.root, "snap-tmp.db")
	if _, err := e.db.Exec(`VACUUM INTO ?`, p); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	os.Remove(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestImportWithoutManifestFailsClean(t *testing.T) {
	e := newBEnv(t)
	ctx := context.Background()

	var usersBefore int64
	e.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&usersBefore)

	bad := filepath.Join(e.root, "no-manifest.zip")
	buildZip(t, bad, map[string][]byte{
		"db.sqlite3": e.snapshotBytes(t), // 有 db 无 manifest
	})

	id, err := e.svc.StartImportOfZip(ctx, "ad", bad, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		j, _ := e.svc.GetJob(ctx, id)
		if j.Status == "failed" {
			if !strings.Contains(j.LastErr, "missing manifest.json") {
				t.Fatalf("应报 manifest 缺失: %s", j.LastErr)
			}
			// 零残留断言：原数据未被清空
			var n int64
			e.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
			if n != usersBefore {
				t.Fatalf("导入失败后事实来源被污染: users %d -> %d", usersBefore, n)
			}
			return
		}
		if j.Status == "done" {
			t.Fatal("缺 manifest 的 zip 不应成功")
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job 超时")
}

func TestSuccessfulImportEnqueuesFullReindex(t *testing.T) {
	e := newBEnv(t)
	ctx := context.Background()

	type rebuildCall struct {
		reason string
		full   bool
	}
	var calls []rebuildCall
	e.svc.SetRebuildHook(func(_ context.Context, docID *string, reason string) (string, error) {
		calls = append(calls, rebuildCall{reason: reason, full: docID == nil})
		return "rb-1", nil
	})

	// 导出一份合法备份（含 manifest + db + attachments/）
	e.docs.CreateDocument(ctx, adminOf(), nil, "rebuild-doc", "R")
	id, err := e.svc.StartExport(ctx, "ad")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 400; i++ {
		j, _ := e.svc.GetJob(ctx, id)
		if j.Status == "done" && j.Filename != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	files, _ := e.svc.ListBackupFiles(ctx)
	if len(files) == 0 {
		t.Fatal("导出无产物")
	}
	zipPath := filepath.Join(e.root, "backups", files[0])

	importID, err := e.svc.StartImportOfZip(ctx, "ad", zipPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 400; i++ {
		j, _ := e.svc.GetJob(ctx, importID)
		if j.Status == "done" || j.Status == "failed" {
			if j.Status != "done" {
				t.Fatalf("导入失败: %s", j.LastErr)
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(calls) != 1 || !calls[0].full || calls[0].reason != "post-import" {
		t.Fatalf("导入成功未入队全量重建: %+v", calls)
	}
}
