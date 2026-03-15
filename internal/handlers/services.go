package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"locoll/internal/docker"
)

type ServiceStatus struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status string `json:"status"` // "up", "down", "unknown"
	Code   int    `json:"code"`
}

type ContainerGroup struct {
	Project    string            `json:"project"`
	Containers []ContainerBrief `json:"containers"`
}

type ContainerBrief struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Health string `json:"health"`
}

var defaultTargets = []struct {
	Name string
	URL  string
}{
	{"Ollama", "http://localhost:11434/api/tags"},
}

// parseServiceEnv parses LOCOLL_SERVICES env var.
// Format: "Name:URL,Name:URL,..."
func parseServiceEnv() []struct{ Name, URL string } {
	env := os.Getenv("LOCOLL_SERVICES")
	if env == "" {
		return nil
	}
	var result []struct{ Name, URL string }
	for _, entry := range strings.Split(env, ",") {
		entry = strings.TrimSpace(entry)
		if parts := strings.SplitN(entry, ":", 2); len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			url := strings.TrimSpace(parts[1])
			// Handle "Name:http://..." — the split eats the "http" part
			if idx := strings.Index(entry, ":http"); idx > 0 {
				name = strings.TrimSpace(entry[:idx])
				url = strings.TrimSpace(entry[idx+1:])
			}
			if name != "" && url != "" {
				result = append(result, struct{ Name, URL string }{name, url})
			}
		}
	}
	return result
}

func getServiceTargets() []struct{ Name, URL string } {
	if targets := parseServiceEnv(); len(targets) > 0 {
		return targets
	}
	return defaultTargets
}

func (h *Handler) Services(w http.ResponseWriter, r *http.Request) {
	statuses := checkServices()

	// Get container groups
	containers, _ := h.Docker.ListContainers(r.Context())
	groups := groupContainers(containers)

	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"services":   statuses,
			"containers": groups,
		})
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<div id="services-grid" hx-swap-oob="true">`)

	// HTTP Health Checks
	fmt.Fprint(w, `<h3 class="section-subtitle">HTTP Health Checks</h3><div class="services-grid">`)
	for _, s := range statuses {
		cssClass := "service-down"
		icon := "&#128308;"
		if s.Status == "up" {
			cssClass = "service-up"
			icon = "&#128994;"
		}
		fmt.Fprintf(w, `<div class="service-card %s">
			<span class="service-icon">%s</span>
			<span class="service-name">%s</span>
			<span class="service-status">%s</span>
		</div>`, cssClass, icon, s.Name, s.Status)
	}
	fmt.Fprint(w, `</div>`)

	// Docker Containers by project
	fmt.Fprint(w, `<h3 class="section-subtitle">Docker Containers</h3>`)
	for _, g := range groups {
		fmt.Fprintf(w, `<div class="container-group-card"><div class="group-header">%s</div><div class="group-items">`, g.Project)
		for _, c := range g.Containers {
			icon := "&#128994;" // 🟢
			cssClass := "ctr-running"
			if c.Health == "unhealthy" {
				icon = "&#128308;"
				cssClass = "ctr-unhealthy"
			} else if c.Health == "healthy" {
				icon = "&#9989;"
				cssClass = "ctr-healthy"
			} else if c.State != "running" {
				icon = "&#11035;"
				cssClass = "ctr-stopped"
			}
			fmt.Fprintf(w, `<span class="ctr-item %s">%s %s</span>`, cssClass, icon, c.Name)
		}
		fmt.Fprint(w, `</div></div>`)
	}
	fmt.Fprint(w, `</div>`)
}

func groupContainers(containers []docker.ContainerInfo) []ContainerGroup {
	m := make(map[string][]ContainerBrief)
	for _, c := range containers {
		proj := c.Project
		if proj == "" {
			proj = "standalone"
		}
		m[proj] = append(m[proj], ContainerBrief{
			Name:   c.Name,
			State:  c.State,
			Health: c.Health,
		})
	}

	var groups []ContainerGroup
	for proj, ctrs := range m {
		groups = append(groups, ContainerGroup{Project: proj, Containers: ctrs})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Project < groups[j].Project })
	return groups
}

func checkServices() []ServiceStatus {
	targets := getServiceTargets()
	var wg sync.WaitGroup
	results := make([]ServiceStatus, len(targets))

	client := &http.Client{Timeout: 3 * time.Second}

	for i, svc := range targets {
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
