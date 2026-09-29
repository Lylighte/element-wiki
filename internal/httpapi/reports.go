package httpapi

import (
	"net/http"
	"strings"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/util"
)

func (d *Deps) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	actor := d.actor(r)
	if err := actor.Require(permission.ReportCreate); err != nil {
		mapServiceErr(w, err)
		return
	}
	var req struct {
		ContentType string `json:"content_type"`
		ContentID   string `json:"content_id"`
		Reason      string `json:"reason"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ContentType != "document" && req.ContentType != "comment" && req.ContentType != "user_page" || strings.TrimSpace(req.ContentID) == "" || len([]rune(strings.TrimSpace(req.Reason))) < 5 || len([]rune(req.Reason)) > 2000 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": "invalid report", "fields": map[string]string{"reason": "length must be 5-2000 and target must be valid"}})
		return
	}
	report := &model.ContentReport{ID: util.NewID(), ReporterID: actor.UserID(), ContentType: req.ContentType, ContentID: req.ContentID, Reason: strings.TrimSpace(req.Reason), Status: "pending", CreatedAt: util.NowMillis()}
	switch req.ContentType {
	case "document":
		if _, err := d.Docs.Get(r.Context(), actor, req.ContentID); err != nil {
			mapServiceErr(w, err)
			return
		}
	case "comment":
		if err := d.Docs.EnsureReportableComment(r.Context(), actor, req.ContentID); err != nil {
			mapServiceErr(w, err)
			return
		}
	case "user_page":
		if d.UserPages == nil {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		page, err := d.UserPages.Get(r.Context(), actor, req.ContentID)
		if err != nil || page.Published == nil {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
	}
	if err := d.Reports.CreateContentReport(r.Context(), report); err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"report": report})
}

func (d *Deps) handleListReports(w http.ResponseWriter, r *http.Request) {
	if err := d.actor(r).Require(permission.ReviewManage); err != nil {
		mapServiceErr(w, err)
		return
	}
	items, err := d.Reports.ListPendingContentReports(r.Context(), 100)
	if mapServiceErr(w, err) {
		return
	}
	views := make([]map[string]any, 0, len(items))
	for _, report := range items {
		view := map[string]any{"id": report.ID, "reporter_id": report.ReporterID, "content_type": report.ContentType, "content_id": report.ContentID, "reason": report.Reason, "status": report.Status, "created_at": report.CreatedAt}
		switch report.ContentType {
		case "document":
			if doc, e := d.Docs.Get(r.Context(), d.actor(r), report.ContentID); e == nil {
				view["title"] = doc.Title
				if body, _, e := d.Docs.HeadContent(r.Context(), d.actor(r), doc.ID); e == nil {
					if rendered, re := d.Render(body); re == nil {
						view["preview_html"] = rendered.HTML
					}
				}
			}
		case "comment":
			if comment, e := d.Docs.CommentForModeration(r.Context(), d.actor(r), report.ContentID); e == nil {
				view["preview_text"] = comment.Content
			}
		case "user_page":
			if d.UserPages != nil {
				if page, e := d.UserPages.Get(r.Context(), d.actor(r), report.ContentID); e == nil && page.Published != nil {
					if rendered, re := d.Render(page.Published.Content); re == nil {
						view["title"] = page.DisplayName
						view["preview_html"] = rendered.HTML
					}
				}
			}
		}
		views = append(views, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": views})
}
func (d *Deps) reviewReport(w http.ResponseWriter, r *http.Request, action string) {
	if err := d.actor(r).Require(permission.ReviewManage); err != nil {
		mapServiceErr(w, err)
		return
	}
	var req struct {
		Resolution string `json:"resolution"`
	}
	if r.Body != nil && r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
	}
	if len([]rune(strings.TrimSpace(req.Resolution))) < 2 || len([]rune(req.Resolution)) > 2000 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": "resolution is required"})
		return
	}
	if err := d.Reports.ReviewContentReport(r.Context(), pathID(r), d.actor(r).UserID(), action, strings.TrimSpace(req.Resolution), util.NowMillis()); err != nil {
		mapServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (d *Deps) handleResolveReport(w http.ResponseWriter, r *http.Request) {
	d.reviewReport(w, r, "resolve")
}
func (d *Deps) handleDismissReport(w http.ResponseWriter, r *http.Request) {
	d.reviewReport(w, r, "dismiss")
}

func (d *Deps) handleListHiddenContent(w http.ResponseWriter, r *http.Request) {
	if err := d.actor(r).Require(permission.ReviewManage); err != nil {
		mapServiceErr(w, err)
		return
	}
	items, err := d.ContentModeration.ListHiddenContent(r.Context(), 100)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (d *Deps) setContentPublished(w http.ResponseWriter, r *http.Request, published bool) {
	if err := d.actor(r).Require(permission.ReviewManage); err != nil {
		mapServiceErr(w, err)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil && r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
	}
	if len([]rune(strings.TrimSpace(req.Reason))) < 2 || len([]rune(req.Reason)) > 2000 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": "reason is required"})
		return
	}
	err := d.ContentModeration.SetContentPublished(r.Context(), r.PathValue("content_type"), r.PathValue("content_id"), published, d.actor(r).UserID(), strings.TrimSpace(req.Reason), util.NowMillis())
	if mapServiceErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (d *Deps) handleUnpublishContent(w http.ResponseWriter, r *http.Request) {
	d.setContentPublished(w, r, false)
}
func (d *Deps) handleRestoreContent(w http.ResponseWriter, r *http.Request) {
	d.setContentPublished(w, r, true)
}
