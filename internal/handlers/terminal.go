package handlers

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	ptypkg "locoll/internal/pty"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *Handler) Terminal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fullID, _ := h.Docker.FindContainerID(r.Context(), id)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade", "err", err)
		return
	}
	defer conn.Close()

	session, err := ptypkg.NewSession(fullID)
	if err != nil {
		slog.Error("pty session", "id", id, "err", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer session.Close()

	// PTY stdout -> WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := session.PTY.Read(buf)
			if err != nil {
				if err != io.EOF {
					slog.Error("pty read", "err", err)
				}
				conn.Close()
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// WebSocket -> PTY stdin
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if _, err := session.PTY.Write(msg); err != nil {
			return
		}
	}
}
