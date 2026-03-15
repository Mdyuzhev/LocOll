package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

type Note struct {
	ID        int    `json:"id"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
	Done      bool   `json:"done"`
}

func getMCPBridgeURL() string {
	if url := os.Getenv("MCP_BRIDGE_URL"); url != "" {
		return url
	}
	return "http://localhost:8765/claude-bridge"
}

func callBridge(tool string, args map[string]interface{}) (json.RawMessage, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"tool": tool,
		"args": args,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(getMCPBridgeURL(), "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Bridge returns { "result": "..." } or { "error": "..." }
	var result struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("bridge response parse error: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("bridge error: %s", result.Error)
	}
	return result.Result, nil
}

func (h *Handler) Notes(w http.ResponseWriter, r *http.Request) {
	raw, err := callBridge("list_notes", map[string]interface{}{})
	if err != nil {
		slog.Error("notes bridge", "err", err)
		// Return empty HTML so the widget doesn't show
		if !wantsJSON(r) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<div id="notes-widget" hx-swap-oob="true"></div>`)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(raw)
		return
	}

	// Parse notes from bridge result (it's a text, try to extract structured data)
	// The bridge returns text, not JSON array. Parse the text into notes.
	var notes []Note
	// Try parsing as JSON array first
	if err := json.Unmarshal(raw, &notes); err != nil {
		// Bridge might return a string with the note list text
		var text string
		if err2 := json.Unmarshal(raw, &text); err2 == nil {
			// Render the raw text
			w.Header().Set("Content-Type", "text/html")
			if text == "" || text == "Нет активных заметок" {
				fmt.Fprint(w, `<div id="notes-widget" hx-swap-oob="true"></div>`)
				return
			}
			fmt.Fprintf(w, `<div id="notes-widget" hx-swap-oob="true">
				<div class="notes-card">
					<div class="notes-header">&#128221; Notes</div>
					<pre style="font-size:0.8rem;white-space:pre-wrap;color:var(--text-dim)">%s</pre>
				</div>
			</div>`, text)
			return
		}
		// Can't parse — hide widget
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div id="notes-widget" hx-swap-oob="true"></div>`)
		return
	}

	// Render structured notes
	w.Header().Set("Content-Type", "text/html")
	if len(notes) == 0 {
		fmt.Fprint(w, `<div id="notes-widget" hx-swap-oob="true"></div>`)
		return
	}

	fmt.Fprint(w, `<div id="notes-widget" hx-swap-oob="true"><div class="notes-card">`)
	fmt.Fprintf(w, `<div class="notes-header">&#128221; Notes (%d)</div>`, len(notes))
	for _, n := range notes {
		fmt.Fprintf(w, `<div class="note-row">
			<span class="note-text">%s</span>
			<span class="note-date">%s</span>
			<button class="note-complete"
				hx-post="/api/v1/notes/%d/complete"
				hx-target="#notes-widget"
				hx-swap="outerHTML">&#10003;</button>
		</div>`, n.Text, n.CreatedAt, n.ID)
	}
	fmt.Fprint(w, `</div></div>`)
}

func (h *Handler) NoteComplete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	_, err := callBridge("complete_note", map[string]interface{}{"id": idStr})
	if err != nil {
		slog.Error("complete note", "err", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Re-render notes widget
	h.Notes(w, r)
}
