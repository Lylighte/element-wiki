package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"element-wiki/internal/service/docservice"
)

// ---- 草稿 ----

func (d *Deps) handlePutDraft(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseCommitID string `json:"base_commit_id"`
		Content      string `json:"content"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := d.Docs.SaveDraft(r.Context(), d.actor(r), pathID(r), req.BaseCommitID, req.Content); mapServiceErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Deps) handleGetDraft(w http.ResponseWriter, r *http.Request) {
	draft, err := d.Docs.GetDraft(r.Context(), d.actor(r), pathID(r))
	if err != nil {
		if isNotFoundErr(err) {
			writeJSON(w, http.StatusOK, map[string]any{"draft": nil})
			return
		}
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (d *Deps) handleDeleteDraft(w http.ResponseWriter, r *http.Request) {
	err := d.Docs.DeleteDraft(r.Context(), d.actor(r), pathID(r))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case isNotFoundErr(err):
		writeErr(w, http.StatusNotFound, "not found")
	default:
		mapServiceErr(w, err)
	}
}

func isNotFoundErr(err error) bool { return docservice.IsNotFound(err) }

// ---- 版本 ----

type commitRequest struct {
	BaseCommitID string  `json:"base_commit_id"`
	Content      string  `json:"content"`
	Message      string  `json:"message"`
	Title        *string `json:"title"`
}

func (d *Deps) handleCommit(w http.ResponseWriter, r *http.Request) {
	var req commitRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	var call []string
	if req.Title != nil {
		call = append(call, *req.Title)
	}
	res, err := d.Docs.Commit(r.Context(), d.actor(r), pathID(r), req.BaseCommitID, req.Content, req.Message, call...)
	if err != nil {
		var vc *docservice.VersionConflictError
		if errors.As(err, &vc) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"detail":         "version conflict",
				"head_commit_id": vc.HeadCommitID,
				"base_commit_id": req.BaseCommitID,
			})
			return
		}
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"commit":     res.Commit,
		"dead_links": res.DeadLinks,
	})
}

func (d *Deps) handleListCommits(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n >= 1 && n <= 500 {
			limit = n
		} else {
			writeErr(w, http.StatusBadRequest, "limit 非法")
			return
		}
	}
	list, err := d.Docs.ListCommits(r.Context(), d.actor(r), pathID(r), limit)
	if mapServiceErr(w, err) {
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, c := range list {
		items = append(items, map[string]any{
			"id": c.ID, "commit_no": c.CommitNo, "message": c.Message,
			"author_id": c.AuthorID, "created_at": c.CreatedAt,
			"parent_commit_id": c.ParentCommitID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "has_next": false, "next_cursor": nil,
	})
}

func (d *Deps) handleRevert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CommitID string `json:"commit_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := d.Docs.Revert(r.Context(), d.actor(r), pathID(r), req.CommitID)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"commit":     res.Commit,
		"dead_links": res.DeadLinks,
	})
}
