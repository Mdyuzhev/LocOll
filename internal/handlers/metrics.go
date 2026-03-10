package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func (h *Handler) MetricsHistory(w http.ResponseWriter, r *http.Request) {
	hours := 1
	if hs := r.URL.Query().Get("hours"); hs != "" {
		if v, err := strconv.Atoi(hs); err == nil && v > 0 && v <= 720 {
			hours = v
		}
	}

	limit := 1000
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 10000 {
			limit = v
		}
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	metrics, err := h.Store.ReadMetrics(since, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}
