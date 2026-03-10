package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type ServiceStatus struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status string `json:"status"` // "up", "down", "unknown"
	Code   int    `json:"code"`
}

var serviceTargets = []struct {
	Name string
	URL  string
}{
	{"Warehouse API", "http://localhost:8080/health"},
	{"Grafana", "http://localhost:3001/api/health"},
	{"Prometheus", "http://localhost:9090/-/healthy"},
	{"ErrorLens", "http://localhost:8002/health"},
	{"Ollama", "http://localhost:11434/api/tags"},
}

func (h *Handler) Services(w http.ResponseWriter, r *http.Request) {
	statuses := checkServices()

	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statuses)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<div id="services-grid" hx-swap-oob="true"><div class="services-grid">`)
	for _, s := range statuses {
		cssClass := "service-down"
		icon := "&#128308;" // 🔴
		if s.Status == "up" {
			cssClass = "service-up"
			icon = "&#128994;" // 🟢
		}
		fmt.Fprintf(w, `<div class="service-card %s">
			<span class="service-icon">%s</span>
			<span class="service-name">%s</span>
			<span class="service-status">%s</span>
		</div>`, cssClass, icon, s.Name, s.Status)
	}
	fmt.Fprint(w, `</div></div>`)
}

func checkServices() []ServiceStatus {
	var wg sync.WaitGroup
	results := make([]ServiceStatus, len(serviceTargets))

	client := &http.Client{Timeout: 3 * time.Second}

	for i, svc := range serviceTargets {
		wg.Add(1)
		go func(idx int, name, url string) {
			defer wg.Done()
			status := ServiceStatus{Name: name, URL: url, Status: "down"}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				results[idx] = status
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				results[idx] = status
				return
			}
			defer resp.Body.Close()

			status.Code = resp.StatusCode
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				status.Status = "up"
			}
			results[idx] = status
		}(i, svc.Name, svc.URL)
	}

	wg.Wait()
	return results
}
