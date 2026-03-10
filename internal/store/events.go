package store

import "time"

type Event struct {
	Ts        int64  `json:"ts"`
	Project   string `json:"project"`
	Container string `json:"container"`
	EventType string `json:"event"`
	Detail    string `json:"detail"`
}

func (s *Store) WriteEvent(e Event) error {
	_, err := s.db.Exec(
		"INSERT INTO events (ts, project, container, event, detail) VALUES (?, ?, ?, ?, ?)",
		e.Ts, e.Project, e.Container, e.EventType, e.Detail,
	)
	return err
}

func (s *Store) ReadEvents(container string, limit int) ([]Event, error) {
	query := "SELECT ts, project, container, event, detail FROM events"
	var args []interface{}

	if container != "" {
		query += " WHERE container = ?"
		args = append(args, container)
	}
	query += " ORDER BY ts DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Ts, &e.Project, &e.Container, &e.EventType, &e.Detail); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (s *Store) PurgeOldEvents(olderThan time.Time) (int64, error) {
	res, err := s.db.Exec("DELETE FROM events WHERE ts < ?", olderThan.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
