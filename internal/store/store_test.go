package store

import (
	"testing"
	"time"
)

func TestStoreMetrics(t *testing.T) {
	st, err := New("file::memory:?_journal=WAL")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Close()

	// Write metric
	m := Metric{
		Ts:         time.Now().Unix(),
		CPUPct:     45.5,
		RAMUsedMB:  4096,
		RAMTotalMB: 23000,
		DiskUsedGB: 61.0,
		LoadAvg1:   1.5,
	}
	if err := st.WriteMetric(m); err != nil {
		t.Fatalf("write metric: %v", err)
	}

	// Read back
	metrics, err := st.ReadMetrics(time.Now().Add(-1*time.Hour), 10)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].CPUPct != 45.5 {
		t.Errorf("expected CPU 45.5, got %f", metrics[0].CPUPct)
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	st, err := New("file::memory:?_journal=WAL")
	if err != nil {
		t.Fatalf("first init: %v", err)
	}

	// Run migrate again
	if err := st.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	st.Close()
}
