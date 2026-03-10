package pty

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type Session struct {
	PTY *os.File
	Cmd *exec.Cmd
}

// NewSession starts docker exec -it in a PTY
func NewSession(containerID string) (*Session, error) {
	cmd := exec.Command("docker", "exec", "-it", containerID, "/bin/sh")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	return &Session{
		PTY: ptmx,
		Cmd: cmd,
	}, nil
}

func (s *Session) Close() {
	if s.PTY != nil {
		s.PTY.Close()
	}
	if s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
	}
}

// Resize changes the PTY window size
func (s *Session) Resize(rows, cols uint16) error {
	return pty.Setsize(s.PTY, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}
