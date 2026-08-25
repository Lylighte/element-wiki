// 备份与导入端点（T6.5~T6.7，admin 权限）。
package httpapi

import (
	"io"
	"net/http"
	"os"
)

// pgBackupUnsupported 报告当前驱动是否不支持备份域（C6）；
// true 表示已写出 501 响应。
func (d *Deps) pgBackupUnsupported(w http.ResponseWriter) bool {
	if d.DBDriver == "postgres" {
		writeErr(w, http.StatusNotImplemented, "backup not supported for postgres")
		return true
	}
	return false
}

func (d *Deps) handleStartBackup(w http.ResponseWriter, r *http.Request) {
	if d.pgBackupUnsupported(w) {
		return
	}
	actor := d.actor(r)
	jobID, err := d.Backups.StartExport(r.Context(), actor.UserID())
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (d *Deps) handleBackupJobStatus(w http.ResponseWriter, r *http.Request) {
	job, err := d.Backups.GetJob(r.Context(), pathID(r))
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (d *Deps) handleListBackupFiles(w http.ResponseWriter, r *http.Request) {
	files, err := d.Backups.ListBackupFiles(r.Context())
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": files})
}

func (d *Deps) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	p, err := d.Backups.BackupFilePath(r.PathValue("name"))
	if mapServiceErr(w, err) {
		return
	}
	f, err := os.Open(p)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/zip")
	if fi, serr := f.Stat(); serr == nil {
		w.Header().Set("Content-Length", itoa64(fi.Size()))
	}
	io.Copy(w, f)
}

func (d *Deps) handleDeleteBackupFile(w http.ResponseWriter, r *http.Request) {
	err := d.Backups.DeleteBackupFile(r.PathValue("name"))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case isNotFoundErr(err):
		writeErr(w, http.StatusNotFound, "not found")
	default:
		mapServiceErr(w, err)
	}
}

func (d *Deps) handleImportBackup(w http.ResponseWriter, r *http.Request) {
	if d.pgBackupUnsupported(w) {
		return
	}
	src, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer src.Close()
	tmp, err := os.CreateTemp("", "ew-import-*.zip")
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	if _, err := io.Copy(tmp, src); err != nil {
		os.Remove(tmp.Name())
		writeErr(w, http.StatusInternalServerError, "failed to stage import")
		return
	}
	tmp.Close()
	actor := d.actor(r)
	tmpPath := tmp.Name()
	jobID, err := d.Backups.StartImportOfZip(r.Context(), actor.UserID(), tmpPath,
		func() { os.Remove(tmpPath) })
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to start import")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (d *Deps) handleStartMarkdownImport(w http.ResponseWriter, r *http.Request) {
	if d.pgBackupUnsupported(w) {
		return
	}
	src, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer src.Close()
	tmp, err := os.CreateTemp("", "ew-md-*.zip")
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	// T12.2 race fix: remove temp file only after the import goroutine finished reading it
	cleanup := func() { os.Remove(tmp.Name()) }
	if _, cerr := io.Copy(tmp, src); cerr != nil {
		tmp.Close()
		cleanup()
		mapServiceErr(w, cerr)
		return
	}
	if cerr := tmp.Close(); cerr != nil {
		cleanup()
		mapServiceErr(w, cerr)
		return
	}
	actor := d.actor(r)
	jobID, err := d.MarkdownImports.StartMarkdownImport(r.Context(), actor.UserID(), tmp.Name(), cleanup)
	if err != nil {
		cleanup()
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (d *Deps) handleImportJobStatus(w http.ResponseWriter, r *http.Request) {
	job, err := d.Imports.GetImportJob(r.Context(), pathID(r))
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	var buf [21]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
