// 管理域端点（T6.1~T6.4）。
package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"element-wiki/internal/permission"
	adminservice "element-wiki/internal/service/adminservice"
)

func (d *Deps) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	all, err := d.Admin.AllSettings(r.Context(), d.actor(r))
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, all)
}

func (d *Deps) handlePatchSettings(w http.ResponseWriter, r *http.Request) {
	var patch map[string]string
	if !decodeJSON(w, r, &patch) {
		return
	}
	if err := d.Admin.UpdateSettings(r.Context(), d.actor(r), patch); mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"detail": "updated"})
}

func (d *Deps) handleListUsers(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, perr := strconv.Atoi(l); perr == nil && n >= 1 && n <= 500 {
			limit = n
		} else {
			writeErr(w, http.StatusBadRequest, "limit 非法")
			return
		}
	}
	list, err := d.Admin.ListUsers(r.Context(), d.actor(r), r.URL.Query().Get("q"), limit)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (d *Deps) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	var raw struct {
		Role   *string `json:"role"`
		Status *string `json:"status"`
	}
	if !decodeJSON(w, r, &raw) {
		return
	}
	var role *permission.Role
	if raw.Role != nil {
		rv := permission.Role(*raw.Role)
		role = &rv
	}
	u, err := d.Admin.UpdateUser(r.Context(), d.actor(r), pathID(r), role, raw.Status)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

type adminRole = string

func (d *Deps) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := d.Admin.Dashboard(r.Context(), d.actor(r))
	if mapServiceErr(w, err) {
		return
	}
	raw, _ := json.Marshal(stats)
	var out map[string]any
	json.Unmarshal(raw, &out)
	writeJSON(w, http.StatusOK, out)
}

var _ = adminservice.ErrValidation
