package httpapi

import (
	"net/http"

	"element-wiki/internal/service/docservice"
)

func (d *Deps) handleListPendingDocuments(w http.ResponseWriter, r *http.Request) {
	items, err := d.Docs.PendingDocumentSubmissions(r.Context(), d.actor(r), 100)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (d *Deps) reviewDocumentSubmission(w http.ResponseWriter, r *http.Request, action string) {
	var req struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil && r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
	}
	err := d.Docs.ReviewDocumentSubmission(r.Context(), d.actor(r), pathID(r), action, req.Reason)
	if err != nil {
		if vc, ok := err.(*docservice.VersionConflictError); ok {
			writeJSON(w, http.StatusConflict, map[string]any{"detail": "submission is based on an outdated version", "head_commit_id": vc.HeadCommitID})
			return
		}
		mapServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (d *Deps) handleApproveDocumentSubmission(w http.ResponseWriter, r *http.Request) {
	d.reviewDocumentSubmission(w, r, "approve")
}
func (d *Deps) handleRejectDocumentSubmission(w http.ResponseWriter, r *http.Request) {
	d.reviewDocumentSubmission(w, r, "reject")
}

func (d *Deps) handleListPendingComments(w http.ResponseWriter, r *http.Request) {
	items, err := d.Docs.PendingComments(r.Context(), d.actor(r), 100)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (d *Deps) reviewComment(w http.ResponseWriter, r *http.Request, action string) {
	var req struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeJSON(w, r, &req) {
			return
		}
	}
	if err := d.Docs.ReviewComment(r.Context(), d.actor(r), pathID(r), action, req.Reason); err != nil {
		mapServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (d *Deps) handleApproveComment(w http.ResponseWriter, r *http.Request) {
	d.reviewComment(w, r, "approve")
}
func (d *Deps) handleRejectComment(w http.ResponseWriter, r *http.Request) {
	d.reviewComment(w, r, "reject")
}
