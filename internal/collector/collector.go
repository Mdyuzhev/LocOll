package collector

import (
	"bytes"
	"context"
	"log/slog"
	"time"

	"locoll/internal/docker"
	"locoll/internal/sse"
	"locoll/internal/store"
	"locoll/internal/system"
)

type TemplRenderer interface {
	RenderServerCard(info system.Info, containers []docker.ContainerInfo) string
}

func Start(ctx context.Context, st *store.Store, dc *docker.Client, broker *sse.Broker, renderer TemplRenderer) {
	collect(ctx, st, dc, broker, renderer)

	ticker := time.NewTicker(60 * time.Second)
	purge := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	defer purge.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collect(ctx, st, dc, broker, renderer)
		case <-purge.C:
			cutoff := time.Now().Add(-30 * 24 * time.Hour)
			if n, err := st.PurgeOldMetrics(cutoff); err != nil {
				slog.Error("purge metrics", "err", err)
			} else if n > 0 {
				slog.Info("purged old metrics", "count", n)
			}
			if n, err := st.PurgeOldEvents(cutoff); err != nil {
				slog.Error("purge events", "err", err)
			} else if n > 0 {
				slog.Info("purged old events", "count", n)
			}
		}
	}
}

// prevStates tracks container states between collections for event detection
var prevStates = make(map[string]string)

func collect(ctx context.Context, st *store.Store, dc *docker.Client, broker *sse.Broker, renderer TemplRenderer) {
	info, err := system.Read()
	if err != nil {
		slog.Error("read system", "err", err)
		return
	}

	now := time.Now().Unix()
	m := store.Metric{
		Ts:         now,
		CPUPct:     info.CPUPct,
		RAMUsedMB:  info.RAMUsedMB,
		RAMTotalMB: info.RAMTotalMB,
		DiskUsedGB: info.DiskUsedGB,
		LoadAvg1:   info.LoadAvg1,
	}
	if err := st.WriteMetric(m); err != nil {
		slog.Error("write metric", "err", err)
	}

	containers, err := dc.ListContainers(ctx)
	if err != nil {
		slog.Error("list containers", "err", err)
		containers = nil
	}

	// Detect state changes and write events
	for _, c := range containers {
		prev, exists := prevStates[c.Name]
		if exists && prev != c.State {
			event := store.Event{
				Ts:        now,
				Project:   c.Project,
				Container: c.Name,
				EventType: c.State, // "running", "exited", etc.
				Detail:    c.Status,
			}
			if err := st.WriteEvent(event); err != nil {
				slog.Error("write event", "err", err)
			}
		}
		prevStates[c.Name] = c.State
	}

	// Broadcast SSE update
	if renderer != nil {
		html := renderer.RenderServerCard(info, containers)
		if html != "" {
			broker.Broadcast(html)
		}
	}
}

// BufferRenderer is a simple renderer that captures templ output to a string
type BufferRenderer struct {
	Buf *bytes.Buffer
}
