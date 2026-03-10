package store

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

var migrations = []string{
	// v1: initial schema
	`CREATE TABLE IF NOT EXISTS metrics (
		ts           INTEGER NOT NULL,
		cpu_pct      REAL,
		ram_used_mb  INTEGER,
		ram_total_mb INTEGER,
		disk_used_gb REAL,
		load_avg_1   REAL
	);
	CREATE INDEX IF NOT EXISTS idx_metrics_ts ON metrics(ts);

	CREATE TABLE IF NOT EXISTS events (
		ts        INTEGER NOT NULL,
		project   TEXT,
		container TEXT,
		event     TEXT,
		detail    TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);
	CREATE INDEX IF NOT EXISTS idx_events_container ON events(container);`,
}

func New(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`)
	if err != nil {
		return err
	}

	var current int
	row := s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&current); err != nil {
		return err
	}

	for i := current; i < len(migrations); i++ {
		slog.Info("applying migration", "version", i+1)
		if _, err := s.db.Exec(migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := s.db.Exec("INSERT INTO schema_version (version) VALUES (?)", i+1); err != nil {
			return err
		}
	}
	return nil
}
