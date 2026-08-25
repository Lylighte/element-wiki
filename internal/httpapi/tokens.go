package httpapi

import (
	"net/http"
	"strings"

	"element-wiki/internal/permission"
)

func (d *Deps) handleListTokens(w http.ResponseWriter, r *http.Request) {
	actor := d.actor(r)
	if err := actor.Require(permission.TokenManageOwn); err != nil {
		mapServiceErr(w, err)
		return
	}
	list, err := d.Auth.ListTokens(r.Context(), actor.UserID())
	if mapServiceErr(w, err) {
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, tk := range list {
		items = append(items, map[string]any{
			"id": tk.ID, "name": tk.Name, "prefix": tk.Prefix,
			"created_at": tk.CreatedAt, "last_used_at": tk.LastUsedAt,
			"revoked_at": tk.RevokedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (d *Deps) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Name) > 60 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"detail": "validation failed",
			"fields": map[string]string{"name": "length must be 1-60"},
		})
		return
	}
	issued, err := d.Auth.IssueToken(r.Context(), d.actor(r).UserID(), req.Name)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": issued.TokenRecord.ID, "name": issued.TokenRecord.Name,
		"prefix": issued.TokenRecord.Prefix, "token": issued.Plaintext,
		"created_at": issued.TokenRecord.CreatedAt,
	})
}

func (d *Deps) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	err := d.Auth.RevokeToken(r.Context(), pathID(r), d.actor(r).UserID())
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case isNotFoundErr(err):
		writeErr(w, http.StatusNotFound, "not found")
	default:
		mapServiceErr(w, err)
	}
}
