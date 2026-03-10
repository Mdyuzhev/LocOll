package store

import "time"

type Metric struct {
	Ts         int64   `json:"ts"`
	CPUPct     float64 `json:"cpu_pct"`
	RAMUsedMB  int     `json:"ram_used_mb"`
	RAMTotalMB int     `json:"ram_total_mb"`
	DiskUsedGB float64 `json:"disk_used_gb"`
	LoadAvg1   float64 `json:"load_avg_1"`
}

func (s *Store) WriteMetric(m Metric) error {
	_, err := s.db.Exec(
		"INSERT INTO metrics (ts, cpu_pct, ram_used_mb, ram_total_mb, disk_used_gb, load_avg_1) VALUES (?, ?, ?, ?, ?, ?)",
		m.Ts, m.CPUPct, m.RAMUsedMB, m.RAMTotalMB, m.DiskUsedGB, m.LoadAvg1,
	)
	return err
}

func (s *Store) ReadMetrics(since time.Time, limit int) ([]Metric, error) {
	rows, err := s.db.Query(
		"SELECT ts, cpu_pct, ram_used_mb, ram_total_mb, disk_used_gb, load_avg_1 FROM metrics WHERE ts > ? ORDER BY ts DESC LIMIT ?",
		since.Unix(), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.Ts, &m.CPUPct, &m.RAMUsedMB, &m.RAMTotalMB, &m.DiskUsedGB, &m.LoadAvg1); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (s *Store) PurgeOldMetrics(olderThan time.Time) (int64, error) {
	res, err := s.db.Exec("DELETE FROM metrics WHERE ts < ?", olderThan.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
