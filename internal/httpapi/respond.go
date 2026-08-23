package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"element-wiki/internal/permission"
	adminservice "element-wiki/internal/service/adminservice"
	"element-wiki/internal/service/docservice"
	"element-wiki/internal/store"
)

// writeJSON 统一 JSON 输出。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeErr 统一错误结构 {"detail": ...}。
func writeErr(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}

// mapServiceErr 把 service/store 错误映射为契约状态码（doc/02 §14）。
// 返回 true 表示已写出响应。
func mapServiceErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, docservice.ErrTooLarge):
		writeErr(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, docservice.ErrBadType):
		writeErr(w, http.StatusUnsupportedMediaType, err.Error())
	case errors.Is(err, permission.ErrDenied):
		writeErr(w, http.StatusForbidden, "permission denied")
	case docservice.IsNotFound(err) || errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrConflict):
		writeErr(w, http.StatusConflict, "conflict")
	default:
		var vc *docservice.VersionConflictError
		if errors.As(err, &vc) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"detail":         "version conflict",
				"head_commit_id": vc.HeadCommitID,
			})
			return true
		}
		var ave *adminservice.ValidationError
		if errors.As(err, &ave) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"detail": "validation failed",
				"fields": map[string]string{ave.Field: ave.Reason},
			})
			return true
		}
		if errors.Is(err, adminservice.ErrParentGoneLike) {
			writeErr(w, http.StatusConflict, "parent deleted")
			return true
		}
		var ve *docservice.ValidationError
		if errors.As(err, &ve) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"detail": "validation failed",
				"fields": map[string]string{ve.Field: ve.Reason},
			})
			return true
		}
		if errors.Is(err, docservice.ErrSelfChild) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"detail": "validation failed",
				"fields": map[string]string{"parent_id": docservice.ErrSelfChild.Error()},
			})
			return true
		}
		slog.Error("未预期错误", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
	}
	return true
}

// decodeJSON 严格解析请求体；失败时已写出 400。
func decodeJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}
