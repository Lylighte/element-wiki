package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"element-wiki/internal/permission"
)

func (d *Deps) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n >= 1 && n <= 100 {
			limit = n
		} else {
			writeErr(w, http.StatusBadRequest, "invalid limit")
			return
		}
	}
	hits, err := d.Search.Search(r.Context(), d.actor(r), q, limit)
	if mapServiceErr(w, err) {
		return
	}
	items := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		items = append(items, map[string]any{
			"document_id": h.DocumentID, "title": h.Title,
			"snippet": h.Snippet, "score": h.Score,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "has_next": false, "next_cursor": nil,
		"page_size": len(items),
	})
}

// T4.4：手动全量重建（admin）。
func (d *Deps) handleRebuildRequest(w http.ResponseWriter, r *http.Request) {
	actor := d.actor(r)
	if err := actor.Require(permission.SearchRebuild); err != nil {
		mapServiceErr(w, err)
		return
	}
	jobID, err := d.Jobs.EnqueueReindex(r.Context(), nil, "manual")
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (d *Deps) handleRebuildStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/admin/search/rebuild/")
	actor := d.actor(r)
	if err := actor.Require(permission.SearchRebuild); err != nil {
		mapServiceErr(w, err)
		return
	}
	job, err := d.Jobs.GetReindexJob(r.Context(), id)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, job)
}
