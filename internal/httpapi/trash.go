package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	docservice "element-wiki/internal/service/docservice"
)

func (d *Deps) handleListTrash(w http.ResponseWriter, r *http.Request) {
	list, err := d.Docs.ListTrash(r.Context(), d.actor(r), 100)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (d *Deps) handleRestoreTrash(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentID *string `json:"parent_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	err := d.Docs.RestoreDocument(r.Context(), d.actor(r), pathID(r), req.ParentID)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case isNotFoundErr(err):
		writeErr(w, http.StatusNotFound, "not found")
	default:
		if errors.Is(err, docservice.ErrParentGone) {
			writeErr(w, http.StatusConflict, "parent deleted")
			return
		}
		mapServiceErr(w, err)
	}
}

func (d *Deps) handlePurgeTrash(w http.ResponseWriter, r *http.Request) {
	err := d.Docs.PurgeDocument(r.Context(), d.actor(r), pathID(r))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case isNotFoundErr(err):
		writeErr(w, http.StatusNotFound, "not found")
	default:
		mapServiceErr(w, err)
	}
}

var _ = json.Marshal
