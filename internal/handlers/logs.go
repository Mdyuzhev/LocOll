package handlers

import (
	"bufio"
	"fmt"
	"html"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) ContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fullID, _ := h.Docker.FindContainerID(r.Context(), id)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	reader, err := h.Docker.ContainerLogs(r.Context(), fullID, "100")
	if err != nil {
		slog.Error("container logs", "id", id, "err", err)
		fmt.Fprintf(w, "data: <div class=\"log-error\">Error: %s</div>\n\n", err.Error())
		flusher.Flush()
		return
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case <-r.Context().Done():
			return
		default:
			line := scanner.Text()
			// Docker log lines have 8-byte header for multiplexed streams
			if len(line) > 8 {
				line = line[8:]
			}
			escaped := html.EscapeString(line)
			fmt.Fprintf(w, "data: <div class=\"log-line\">%s</div>\n\n", escaped)
			flusher.Flush()
		}
	}
}
