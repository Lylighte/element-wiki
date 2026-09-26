package httpapi

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"element-wiki/internal/permission"
	"element-wiki/internal/util"
)

var siteIconFilename = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}\.(png|ico|webp)$`)

func (d *Deps) handleUploadSiteIcon(w http.ResponseWriter, r *http.Request) {
	actor := d.actor(r)
	if err := actor.Require(permission.SettingsManage); err != nil {
		mapServiceErr(w, err)
		return
	}
	if d.Admin == nil || d.AttachDir == "" {
		writeErr(w, http.StatusServiceUnavailable, "site icon storage unavailable")
		return
	}
	maxBytes := d.Admin.IntSetting(r.Context(), "upload_max_mb", d.UploadMaxBytes/(1024*1024)) * 1024 * 1024
	if maxBytes < 1 {
		maxBytes = 1 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+(1<<20)) // multipart envelope allowance
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) || errors.Is(err, multipart.ErrMessageTooLarge) {
			writeErr(w, http.StatusRequestEntityTooLarge, "upload too large")
			return
		}
		writeErr(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()
	if header.Size > maxBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "upload too large")
		return
	}
	prefix := make([]byte, 12)
	n, readErr := io.ReadFull(file, prefix)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		writeErr(w, http.StatusBadRequest, "cannot read upload")
		return
	}
	prefix = prefix[:n]
	ext, mimeType, ok := siteIconType(header.Filename, prefix)
	if !ok {
		writeErr(w, http.StatusUnprocessableEntity, "site icon must be a valid PNG, ICO, or WebP image")
		return
	}

	root := filepath.Join(d.AttachDir, "_site-icons")
	if err := os.MkdirAll(root, 0o750); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot store site icon")
		return
	}
	filename := util.NewID() + "." + ext
	tmp, err := os.CreateTemp(root, ".upload-*")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot store site icon")
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	written, copyErr := io.Copy(tmp, io.LimitReader(io.MultiReader(bytes.NewReader(prefix), file), maxBytes+1))
	if copyErr == nil && written > maxBytes {
		copyErr = errors.New("upload too large")
	}
	if copyErr == nil {
		err = tmp.Sync()
	} else {
		err = copyErr
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		if written > maxBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "upload too large")
			return
		}
		writeErr(w, http.StatusInternalServerError, "cannot store site icon")
		return
	}
	finalPath := filepath.Join(root, filename)
	if err := os.Rename(tmpName, finalPath); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot store site icon")
		return
	}
	publicURL := "/v1/site/icon/" + filename
	oldURL := d.Admin.StrSetting(r.Context(), "site_icon_url", "")
	if err := d.Admin.UpdateSettings(r.Context(), actor, map[string]string{"site_icon_url": publicURL}); err != nil {
		_ = os.Remove(finalPath)
		mapServiceErr(w, err)
		return
	}
	removeStoredSiteIcon(d.AttachDir, oldURL)
	writeJSON(w, http.StatusCreated, map[string]string{"site_icon_url": publicURL, "mime_type": mimeType})
}

func siteIconType(filename string, content []byte) (string, string, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		return "png", "image/png", bytes.HasPrefix(content, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	case ".ico":
		return "ico", "image/x-icon", len(content) >= 6 && content[0] == 0 && content[1] == 0 && content[2] == 1 && content[3] == 0
	case ".webp":
		return "webp", "image/webp", len(content) >= 12 && string(content[:4]) == "RIFF" && string(content[8:12]) == "WEBP"
	default:
		return "", "", false
	}
}

func removeStoredSiteIcon(attachDir, siteIconURL string) {
	prefix := "/v1/site/icon/"
	if !strings.HasPrefix(siteIconURL, prefix) {
		return
	}
	filename := strings.TrimPrefix(siteIconURL, prefix)
	if !siteIconFilename.MatchString(filename) {
		return
	}
	_ = os.Remove(filepath.Join(attachDir, "_site-icons", filename))
}

func (d *Deps) handleSiteIcon(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if !siteIconFilename.MatchString(filename) || d.AttachDir == "" {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(d.AttachDir, "_site-icons", filename))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	ext := filepath.Ext(filename)
	contentType := map[string]string{".png": "image/png", ".ico": "image/x-icon", ".webp": "image/webp"}[ext]
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeContent(w, r, filename, modTimeZero(), f)
}
