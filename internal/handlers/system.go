package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"locoll/internal/system"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) System(w http.ResponseWriter, r *http.Request) {
	info, err := system.Read()
	if err != nil {
		slog.Error("read system", "err", err)
		http.Error(w, "failed to read system info", http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
		return
	}

	// Return HTML fragment
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div id="server-metrics" hx-swap-oob="true">
		<div class="server-card">
			<div class="metric"><span class="label">CPU</span><div class="bar"><div class="fill %s" style="width:%.0f%%"></div></div><span class="value">%.1f%%</span></div>
			<div class="metric"><span class="label">RAM</span><div class="bar"><div class="fill %s" style="width:%.0f%%"></div></div><span class="value">%d / %d MB</span></div>
			<div class="metric"><span class="label">Disk</span><div class="bar"><div class="fill %s" style="width:%.0f%%"></div></div><span class="value">%.1f / %.1f GB</span></div>
			<div class="metric"><span class="label">Load</span><span class="value">%.2f / %.2f / %.2f</span></div>
			<div class="metric"><span class="label">Uptime</span><span class="value" id="uptime-value">%s</span></div>
		</div>
	</div>`,
		colorClass(info.CPUPct), info.CPUPct, info.CPUPct,
		colorClass(float64(info.RAMUsedMB)/float64(info.RAMTotalMB)*100), float64(info.RAMUsedMB)/float64(info.RAMTotalMB)*100, info.RAMUsedMB, info.RAMTotalMB,
		colorClass(info.DiskUsedGB/info.DiskTotalGB*100), info.DiskUsedGB/info.DiskTotalGB*100, info.DiskUsedGB, info.DiskTotalGB,
		info.LoadAvg1, info.LoadAvg5, info.LoadAvg15,
		formatUptime(info.UptimeSec),
	)
}

func (h *Handler) SSEEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.Broker.Subscribe()
	defer h.Broker.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			// SSE requires each line to start with "data: "
			escaped := strings.ReplaceAll(msg, "\n", "\ndata: ")
			fmt.Fprintf(w, "data: %s\n\n", escaped)
			flusher.Flush()
		}
	}
}

func colorClass(pct float64) string {
	switch {
	case pct >= 90:
		return "red"
	case pct >= 70:
		return "yellow"
	default:
		return "green"
	}
}

func formatUptime(seconds int) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	mins := (seconds % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}
