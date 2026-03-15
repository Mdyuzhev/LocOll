package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"locoll/internal/collector"
	"locoll/internal/docker"
	"locoll/internal/handlers"
	"locoll/internal/ollama"
	"locoll/internal/sse"
	"locoll/internal/store"
	"locoll/internal/system"
)

func main() {
	slog.Info("starting LocOll Portal")

	// SQLite
	dbPath := os.Getenv("LOCOLL_DB_PATH")
	if dbPath == "" {
		dbPath = "file:./data/portal.db?_journal=WAL"
	}
	st, err := store.New(dbPath)
	if err != nil {
		slog.Error("init store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	// Docker
	dc, err := docker.NewClient()
	if err != nil {
		slog.Error("init docker", "err", err)
		os.Exit(1)
	}
	defer dc.Close()

	// SSE Broker
	broker := sse.NewBroker()

	// Ollama
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	ollamaClient := ollama.NewClient(ollamaURL)

	// Handlers
	h := &handlers.Handler{
		Store:  st,
		Docker: dc,
		Broker: broker,
		Ollama: ollamaClient,
	}

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Pages
	r.Get("/", h.IndexPage)

	// SSE
	r.Get("/events", h.SSEEvents)

	// Fragments (htmx)
	r.Get("/fragments/server", h.System)
	r.Get("/fragments/containers", h.Containers)
	r.Get("/fragments/services", h.Services)

	// Container actions
	r.Post("/containers/{id}/restart", h.ContainerRestart)
	r.Post("/containers/{id}/stop", h.ContainerStop)
	r.Post("/containers/{id}/start", h.ContainerStart)
	r.Get("/containers/{id}/logs", h.ContainerLogs)

	// WebSocket terminal
	r.Get("/terminal/{id}", h.Terminal)

	// Metrics & Analytics
	r.Get("/metrics/history", h.MetricsHistory)

	// AI
	r.Post("/ai/analyze", h.AIAnalyze)

	// Notes
	r.Get("/api/v1/notes", h.Notes)
	r.Post("/api/v1/notes/{id}/complete", h.NoteComplete)

	// JSON API
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", h.Health)
		r.Get("/system", h.System)
		r.Get("/containers", h.Containers)
		r.Get("/services", h.Services)
		r.Get("/metrics", h.MetricsHistory)
		r.Get("/events", h.Events)
		r.Post("/analyze", h.AIAnalyze)
		r.Get("/models", h.AIModels)
	})

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Start collector
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	renderer := &simpleRenderer{}
	go collector.Start(ctx, st, dc, broker, renderer)

	// Server
	addr := os.Getenv("LOCOLL_ADDR")
	if addr == "" {
		addr = ":8010"
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		slog.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}

// simpleRenderer implements collector.TemplRenderer
type simpleRenderer struct{}

func (r *simpleRenderer) RenderServerCard(info system.Info, containers []docker.ContainerInfo) string {
	cpuColor := colorClass(info.CPUPct)
	ramPct := float64(info.RAMUsedMB) / float64(info.RAMTotalMB) * 100
	ramColor := colorClass(ramPct)
	diskPct := float64(0)
	if info.DiskTotalGB > 0 {
		diskPct = info.DiskUsedGB / info.DiskTotalGB * 100
	}
	diskColor := colorClass(diskPct)

	now := time.Now().Format("15:04:05")
	html := `<div id="server-metrics" hx-swap-oob="true">` +
		`<div class="server-card">` +
		metric("CPU", cpuColor, info.CPUPct, sprintf("%.1f%%", info.CPUPct)) +
		metric("RAM", ramColor, ramPct, sprintf("%d / %d MB", info.RAMUsedMB, info.RAMTotalMB)) +
		metric("Disk", diskColor, diskPct, sprintf("%.1f / %.1f GB", info.DiskUsedGB, info.DiskTotalGB)) +
		`<div class="metric"><span class="label">Load</span><span class="value">` + sprintf("%.2f / %.2f / %.2f", info.LoadAvg1, info.LoadAvg5, info.LoadAvg15) + `</span></div>` +
		`<div class="metric"><span class="label">Uptime</span><span class="value" id="uptime-value">` + formatUptime(info.UptimeSec) + `</span></div>` +
		`<div class="metric"><span class="label">Updated</span><span class="value" style="color:var(--text-dim);font-size:0.8rem">` + now + `</span></div>` +
		`</div></div>`

	// Alerts block (OOB swap)
	html += renderAlerts(containers)

	// Uptime header (OOB swap)
	html += `<span id="header-uptime" hx-swap-oob="true" class="header-uptime">` + "\u2191 " + formatUptime(info.UptimeSec) + `</span>`

	// Title with unhealthy count (OOB swap)
	unhealthyCount := 0
	for _, c := range containers {
		if c.Health == "unhealthy" {
			unhealthyCount++
		}
	}
	titlePrefix := ""
	if unhealthyCount > 0 {
		titlePrefix = sprintf("\u26a0\ufe0f (%d) ", unhealthyCount)
	}
	html += `<title id="page-title" hx-swap-oob="true">` + titlePrefix + `LocOll — Lab Portal</title>`

	return html
}

func renderAlerts(containers []docker.ContainerInfo) string {
	var alerts []docker.ContainerInfo
	for _, c := range containers {
		if c.Health == "unhealthy" || c.State != "running" {
			alerts = append(alerts, c)
		}
	}

	html := `<div id="alerts-block" hx-swap-oob="true">`
	if len(alerts) > 0 {
		html += `<div class="alerts-card">`
		html += sprintf(`<div class="alerts-header">&#9888;&#65039; Require attention (%d)</div>`, len(alerts))
		for _, a := range alerts {
			status := a.Health
			if status == "" {
				status = a.State
			}
			cssClass := "alert-unhealthy"
			if a.State != "running" {
				cssClass = "alert-stopped"
			}
			html += sprintf(`<div class="alert-row %s"><span class="alert-name">%s</span><span class="alert-status">%s</span></div>`, cssClass, a.Name, status)
		}
		html += `</div>`
	}
	html += `</div>`
	return html
}

func metric(label, color string, pct float64, value string) string {
	return `<div class="metric"><span class="label">` + label + `</span>` +
		`<div class="bar"><div class="fill ` + color + `" style="width:` + sprintf("%.0f", pct) + `%"></div></div>` +
		`<span class="value">` + value + `</span></div>`
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
		return sprintf("%dd %dh %dm", days, hours, mins)
	}
	return sprintf("%dh %dm", hours, mins)
}

func sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
