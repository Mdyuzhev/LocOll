package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	container := r.URL.Query().Get("container")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	events, err := h.Store.ReadEvents(container, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}
