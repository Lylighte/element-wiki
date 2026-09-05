package httpapi

import (
	"net/http"
)

func (d *Deps) handleListTrash(w http.ResponseWriter, r *http.Request) {
	list, err := d.Docs.ListTrash(r.Context(), d.actor(r), 100)
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

// handleRestoreTrash 恢复到「已恢复」容器（M18）：落位由 service 决定，无请求体。
func (d *Deps) handleRestoreTrash(w http.ResponseWriter, r *http.Request) {
	err := d.Docs.RestoreDocument(r.Context(), d.actor(r), pathID(r))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case isNotFoundErr(err):
		writeErr(w, http.StatusNotFound, "not found")
	default:
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
