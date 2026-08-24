// 评论端点（T5.6）：comments_enabled=false 时一律 403。
package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (d *Deps) commentsGate(w http.ResponseWriter, r *http.Request) bool {
	if d.Admin != nil {
		if !d.Admin.CommentsEnabled(r.Context()) {
			writeErr(w, http.StatusForbidden, "comments disabled")
			return false
		}
		return true
	}
	if !d.CommentsEnabled {
		writeErr(w, http.StatusForbidden, "comments disabled")
		return false
	}
	return true
}

func (d *Deps) handleAddComment(w http.ResponseWriter, r *http.Request) {
	if !d.commentsGate(w, r) {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	c, err := d.Docs.AddComment(r.Context(), d.actor(r), pathID(r), req.Content)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"comment": c})
}

func (d *Deps) handleListComments(w http.ResponseWriter, r *http.Request) {
	if !d.commentsGate(w, r) {
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, perr := strconv.Atoi(l); perr == nil && n >= 1 && n <= 200 {
			limit = n
		} else {
			writeErr(w, http.StatusBadRequest, "limit 非法")
			return
		}
	}
	list, err := d.Docs.ListComments(r.Context(), d.actor(r), pathID(r), limit)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (d *Deps) handleDeleteComment(w http.ResponseWriter, r *http.Request) {
	if !d.commentsGate(w, r) {
		return
	}
	err := d.Docs.DeleteComment(r.Context(), d.actor(r), pathID(r))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case isNotFoundErr(err):
		writeErr(w, http.StatusNotFound, "not found")
	default:
		mapServiceErr(w, err)
	}
}

var _ = json.Marshal // 保持导入（encode 由 writeJSON 承担）
