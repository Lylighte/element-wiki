package httpapi

import (
	"errors"
	"net/http"

	"element-wiki/internal/permission"
	authservice "element-wiki/internal/service/authservice"
)

func (d *Deps) handleGetMyPreferences(w http.ResponseWriter, r *http.Request) {
	actor := d.actor(r)
	if err := actor.Require(permission.TokenManageOwn); err != nil {
		mapServiceErr(w, err)
		return
	}
	p, err := d.Auth.GetPreferences(r.Context(), actor.UserID())
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preferences": p})
}

func (d *Deps) handleSetMyPreferences(w http.ResponseWriter, r *http.Request) {
	actor := d.actor(r)
	if err := actor.Require(permission.TokenManageOwn); err != nil {
		mapServiceErr(w, err)
		return
	}
	var req struct {
		Language string `json:"language"`
		Theme    string `json:"theme"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	p, err := d.Auth.SetPreferences(r.Context(), actor.UserID(), req.Language, req.Theme)
	if err != nil {
		if errors.Is(err, authservice.ErrInvalidPreferences) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": "validation failed", "fields": map[string]string{"language": "must be zh-CN or en", "theme": "must be light, dark, or system"}})
			return
		}
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preferences": p})
}
