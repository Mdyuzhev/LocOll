package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) Containers(w http.ResponseWriter, r *http.Request) {
	containers, err := h.Docker.ListContainers(r.Context())
	if err != nil {
		slog.Error("list containers", "err", err)
		http.Error(w, "failed to list containers", http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(containers)
		return
	}

	// Group by project
	projects := make(map[string][]struct {
		ID, Name, Image, Status, State, Health, Ports string
	})
	for _, c := range containers {
		proj := c.Project
		if proj == "" {
			proj = "standalone"
		}
		projects[proj] = append(projects[proj], struct {
			ID, Name, Image, Status, State, Health, Ports string
		}{c.ID, c.Name, c.Image, c.Status, c.State, c.Health, c.Ports})
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<div id="containers-list" hx-swap-oob="true">`)
	for project, ctrs := range projects {
		fmt.Fprintf(w, `<div class="project-group"><h3 class="project-name">%s</h3><table class="container-table"><thead><tr><th>Status</th><th>Name</th><th>Image</th><th>State</th><th>Ports</th><th>Actions</th></tr></thead><tbody>`, project)
		for _, c := range ctrs {
			icon := stateIcon(c.State, c.Health)
			fmt.Fprintf(w, `<tr id="row-%s">
				<td>%s</td>
				<td>%s</td>
				<td class="image">%s</td>
				<td>%s</td>
				<td>%s</td>
				<td class="actions">
					<div x-data="{ confirm: false, action: '' }">
						<div x-show="!confirm" class="btn-group">`,
				c.ID, icon, c.Name, c.Image, c.Status, c.Ports)

			if c.State == "running" {
				fmt.Fprintf(w, `
							<button class="btn btn-warning btn-sm" @click="confirm=true; action='restart'">Restart</button>
							<button class="btn btn-danger btn-sm" @click="confirm=true; action='stop'">Stop</button>
							<button class="btn btn-info btn-sm" hx-get="/containers/%s/logs" hx-target="#log-output" hx-swap="innerHTML">Logs</button>
							<button class="btn btn-secondary btn-sm" onclick="openTerminal('%s')">Terminal</button>`,
					c.ID, c.ID)
			} else {
				fmt.Fprintf(w, `
							<button class="btn btn-success btn-sm" @click="confirm=true; action='start'">Start</button>`)
			}

			fmt.Fprintf(w, `
						</div>
						<div x-show="confirm" x-cloak class="confirm-group">
							<span>Sure?</span>
							<button class="btn btn-sm btn-primary"
								hx-post="/containers/%s/" x-bind:hx-post="'/containers/%s/' + action"
								hx-target="#row-%s" hx-swap="outerHTML"
								@click="confirm=false">Yes</button>
							<button class="btn btn-sm" @click="confirm=false">No</button>
						</div>
					</div>
				</td>
			</tr>`, c.ID, c.ID, c.ID)
		}
		fmt.Fprint(w, `</tbody></table></div>`)
	}
	fmt.Fprint(w, `</div>`)
}

func (h *Handler) ContainerRestart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fullID, _ := h.Docker.FindContainerID(r.Context(), id)
	if err := h.Docker.RestartContainer(r.Context(), fullID); err != nil {
		slog.Error("restart container", "id", id, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.containerRow(w, r, id)
}

func (h *Handler) ContainerStop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fullID, _ := h.Docker.FindContainerID(r.Context(), id)
	if err := h.Docker.StopContainer(r.Context(), fullID); err != nil {
		slog.Error("stop container", "id", id, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.containerRow(w, r, id)
}

func (h *Handler) ContainerStart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fullID, _ := h.Docker.FindContainerID(r.Context(), id)
	if err := h.Docker.StartContainer(r.Context(), fullID); err != nil {
		slog.Error("start container", "id", id, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.containerRow(w, r, id)
}

func (h *Handler) containerRow(w http.ResponseWriter, r *http.Request, id string) {
	containers, err := h.Docker.ListContainers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, c := range containers {
		if c.ID == id {
			icon := stateIcon(c.State, c.Health)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<tr id="row-%s">
				<td>%s</td>
				<td>%s</td>
				<td class="image">%s</td>
				<td>%s</td>
				<td>%s</td>
				<td class="actions">
					<div x-data="{ confirm: false, action: '' }">
						<div x-show="!confirm" class="btn-group">`, c.ID, icon, c.Name, c.Image, c.Status, c.Ports)

			if c.State == "running" {
				fmt.Fprintf(w, `
							<button class="btn btn-warning btn-sm" @click="confirm=true; action='restart'">Restart</button>
							<button class="btn btn-danger btn-sm" @click="confirm=true; action='stop'">Stop</button>
							<button class="btn btn-info btn-sm" hx-get="/containers/%s/logs" hx-target="#log-output" hx-swap="innerHTML">Logs</button>
							<button class="btn btn-secondary btn-sm" onclick="openTerminal('%s')">Terminal</button>`, c.ID, c.ID)
			} else {
				fmt.Fprintf(w, `
							<button class="btn btn-success btn-sm" @click="confirm=true; action='start'">Start</button>`)
			}

			fmt.Fprintf(w, `
						</div>
						<div x-show="confirm" x-cloak class="confirm-group">
							<span>Sure?</span>
							<button class="btn btn-sm btn-primary"
								hx-post="/containers/%s/" x-bind:hx-post="'/containers/%s/' + action"
								hx-target="#row-%s" hx-swap="outerHTML"
								@click="confirm=false">Yes</button>
							<button class="btn btn-sm" @click="confirm=false">No</button>
						</div>
					</div>
				</td>
			</tr>`, c.ID, c.ID, c.ID)
			return
		}
	}
	http.NotFound(w, r)
}

func stateIcon(state, health string) string {
	switch {
	case health == "healthy":
		return "&#9989;" // ✅
	case health == "unhealthy":
		return "&#128308;" // 🔴
	case state == "running":
		return "&#128994;" // 🟢
	case state == "restarting":
		return "&#128309;" // 🔵
	default:
		return "&#11035;" // ⬛
	}
}
