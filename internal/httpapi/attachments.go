// 附件端点（T5.4/T5.5）。
package httpapi

import (
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"element-wiki/internal/permission"
)

func (d *Deps) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	// 先鉴权后解析：权限拒绝优先于报文错误
	if err := d.actor(r).Require(permission.AttachmentUpload); err != nil {
		mapServiceErr(w, err)
		return
	}
	if err := r.ParseMultipartForm(d.maxUploadBytes()); err != nil {
		if errors.Is(err, multipart.ErrMessageTooLarge) {
			writeErr(w, http.StatusRequestEntityTooLarge, "upload too large")
			return
		}
		writeErr(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "缺少 file 字段")
		return
	}
	defer file.Close()
	a, err := d.Docs.UploadAttachment(r.Context(), d.actor(r), pathID(r), header.Filename, file)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (d *Deps) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	list, err := d.Docs.ListAttachments(r.Context(), d.actor(r), pathID(r))
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (d *Deps) handleRawAttachment(w http.ResponseWriter, r *http.Request) {
	a, err := d.Docs.GetAttachment(r.Context(), d.actor(r), pathID(r))
	if mapServiceErr(w, err) {
		return
	}
	f, err := os.Open(filepath.Join(d.attachRoot(), a.StoragePath))
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", a.MimeType)
	w.Header().Set("Content-Disposition",
		"attachment; filename=\""+sanitizeHeaderFilename(a.Filename)+"\"")
	http.ServeContent(w, r, a.Filename, modTimeZero(), f)
}

func (d *Deps) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	err := d.Docs.DeleteAttachment(r.Context(), d.actor(r), pathID(r))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case isNotFoundErr(err):
		writeErr(w, http.StatusNotFound, "not found")
	default:
		mapServiceErr(w, err)
	}
}
