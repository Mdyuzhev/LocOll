package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type analyzeRequest struct {
	ContainerID string `json:"container_id"`
	Model       string `json:"model"`
}

type analyzeResponse struct {
	Analysis string `json:"analysis"`
	Model    string `json:"model"`
}

func (h *Handler) AIAnalyze(w http.ResponseWriter, r *http.Request) {
	if h.Ollama == nil {
		http.Error(w, "Ollama not configured", http.StatusServiceUnavailable)
		return
	}

	var req analyzeRequest
	if r.Header.Get("Content-Type") == "application/json" {
		json.NewDecoder(r.Body).Decode(&req)
	} else {
		r.ParseForm()
		req.ContainerID = r.FormValue("container_id")
		req.Model = r.FormValue("model")
	}

	if req.ContainerID == "" {
		http.Error(w, "container_id required", http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		req.Model = "mistral:latest"
	}

	// Get container info
	fullID, _ := h.Docker.FindContainerID(r.Context(), req.ContainerID)
	info, err := h.Docker.InspectContainer(r.Context(), fullID)
	if err != nil {
		http.Error(w, "container not found: "+err.Error(), http.StatusNotFound)
		return
	}

	containerName := info.Name
	status := info.State.Status
	health := ""
	if info.State.Health != nil {
		health = info.State.Health.Status
	}

	// Get recent events
	events, _ := h.Store.ReadEvents(containerName, 50)
	var eventStrs []string
	for _, e := range events {
		eventStrs = append(eventStrs, fmt.Sprintf("[%s] %s: %s - %s",
			time.Unix(e.Ts, 0).Format("2006-01-02 15:04"), e.Container, e.EventType, e.Detail))
	}

	// Get recent metrics
	metrics, _ := h.Store.ReadMetrics(time.Now().Add(-1*time.Hour), 60)
	var metricStrs []string
	for _, m := range metrics {
		metricStrs = append(metricStrs, fmt.Sprintf("[%s] CPU: %.1f%%, RAM: %.1f GB",
			time.Unix(m.Ts, 0).Format("15:04"), m.CPUPct, float64(m.RAMUsedMB)/1024))
	}

	prompt := h.Ollama.BuildAnalysisPrompt(containerName, status, health, eventStrs, metricStrs)

	slog.Info("ai analyze", "container", containerName, "model", req.Model)
	response, err := h.Ollama.Generate(r.Context(), req.Model, prompt)
	if err != nil {
		slog.Error("ollama generate", "err", err)
		http.Error(w, "AI analysis failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analyzeResponse{Analysis: response, Model: req.Model})
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div id="ai-result" class="ai-result"><pre>%s</pre></div>`, response)
}

func (h *Handler) AIModels(w http.ResponseWriter, r *http.Request) {
	if h.Ollama == nil {
		json.NewEncoder(w).Encode([]string{})
		return
	}

	models, err := h.Ollama.ListModels(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}
