package handlers

import (
	"net/http"
	"strings"

	"locoll/internal/docker"
	"locoll/internal/ollama"
	"locoll/internal/sse"
	"locoll/internal/store"
)

type Handler struct {
	Store  *store.Store
	Docker *docker.Client
	Broker *sse.Broker
	Ollama *ollama.Client
}

func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}
