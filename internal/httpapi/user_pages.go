package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	userpageservice "element-wiki/internal/service/userpageservice"
)

func (d *Deps) userPageResponse(w http.ResponseWriter, page *model.UserPage) {
	view := map[string]any{"user_id": page.UserID, "display_name": page.DisplayName, "updated_at": page.UpdatedAt}
	if page.Published != nil {
		html := ""
		if rendered, err := d.Render(page.Published.Content); err == nil {
			html = rendered.HTML
		}
		view["published"] = map[string]any{"id": page.Published.ID, "content": page.Published.Content, "html": html, "created_at": page.Published.CreatedAt}
	}
	if page.Pending != nil {
		html := ""
		if rendered, err := d.Render(page.Pending.Content); err == nil {
			html = rendered.HTML
		}
		view["pending"] = map[string]any{"id": page.Pending.ID, "content": page.Pending.Content, "html": html, "created_at": page.Pending.CreatedAt, "reason": page.Pending.Reason}
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_page": view})
}

func (d *Deps) userPageHistory(w http.ResponseWriter, r *http.Request, userID string) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	items, err := d.UserPages.History(r.Context(), d.actor(r), userID, limit)
	if errors.Is(err, userpageservice.ErrDisabled) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (d *Deps) handleGetMyPageRevisions(w http.ResponseWriter, r *http.Request) {
	d.userPageHistory(w, r, d.actor(r).UserID())
}
func (d *Deps) handleGetUserPageRevisions(w http.ResponseWriter, r *http.Request) {
	d.userPageHistory(w, r, r.PathValue("user_id"))
}

func (d *Deps) getUserPage(w http.ResponseWriter, r *http.Request, userID string) {
	page, err := d.UserPages.Get(r.Context(), d.actor(r), userID)
	if errors.Is(err, userpageservice.ErrDisabled) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if mapServiceErr(w, err) {
		return
	}
	d.userPageResponse(w, page)
}
func (d *Deps) handleGetMyPage(w http.ResponseWriter, r *http.Request) {
	actor := d.actor(r)
	if err := actor.Require(permission.UserPageManageOwn); err != nil {
		mapServiceErr(w, err)
		return
	}
	d.getUserPage(w, r, actor.UserID())
}
func (d *Deps) handleGetUserPage(w http.ResponseWriter, r *http.Request) {
	d.getUserPage(w, r, r.PathValue("user_id"))
}
func (d *Deps) saveUserPage(w http.ResponseWriter, r *http.Request, userID string) {
	var req struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	page, err := d.UserPages.Save(r.Context(), d.actor(r), userID, req.Content)
	if errors.Is(err, userpageservice.ErrDisabled) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if errors.Is(err, userpageservice.ErrInvalid) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": "validation failed", "fields": map[string]string{"content": "length must be 1-20000"}})
		return
	}
	if mapServiceErr(w, err) {
		return
	}
	d.userPageResponse(w, page)
}
func (d *Deps) handleSaveMyPage(w http.ResponseWriter, r *http.Request) {
	d.saveUserPage(w, r, d.actor(r).UserID())
}
func (d *Deps) handleAdminSaveUserPage(w http.ResponseWriter, r *http.Request) {
	d.saveUserPage(w, r, r.PathValue("user_id"))
}
func (d *Deps) handleDeleteMyPage(w http.ResponseWriter, r *http.Request) {
	err := d.UserPages.Delete(r.Context(), d.actor(r), d.actor(r).UserID())
	if mapServiceErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (d *Deps) reviewUserPage(w http.ResponseWriter, r *http.Request, action string) {
	var req struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeJSON(w, r, &req) {
			return
		}
	}
	err := d.UserPages.Review(r.Context(), d.actor(r), r.PathValue("user_id"), r.PathValue("revision_id"), action, req.Reason)
	switch {
	case errors.Is(err, userpageservice.ErrSelfReview):
		writeErr(w, http.StatusForbidden, "cannot review own submission")
	case errors.Is(err, userpageservice.ErrInvalid):
		writeErr(w, http.StatusUnprocessableEntity, "invalid review")
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	default:
		mapServiceErr(w, err)
	}
}
func (d *Deps) handleApproveUserPage(w http.ResponseWriter, r *http.Request) {
	d.reviewUserPage(w, r, "approve")
}
func (d *Deps) handleRejectUserPage(w http.ResponseWriter, r *http.Request) {
	d.reviewUserPage(w, r, "reject")
}
func (d *Deps) handleListPendingUserPages(w http.ResponseWriter, r *http.Request) {
	items, err := d.UserPages.Pending(r.Context(), d.actor(r), 50)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
