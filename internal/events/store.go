package events

import (
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func OpenStore(db *sql.DB) (*Store, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS events (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		type       TEXT NOT NULL,
		instance   TEXT NOT NULL DEFAULT '',
		message    TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("events: create table: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Insert(e Event) (uint64, error) {
	res, err := s.db.Exec(
		`INSERT INTO events (type, instance, message, created_at) VALUES (?, ?, ?, ?)`,
		e.Type, e.Instance, e.Message, e.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("events: insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

func (s *Store) Recent(n int) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT id, type, instance, message, created_at FROM events ORDER BY id DESC LIMIT ?`, n,
	)
	if err != nil {
		return nil, fmt.Errorf("events: recent: %w", err)
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.Instance, &e.Message, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("events: recent scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ByInstance(instanceID string, since string, limit int) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT id, type, instance, message, created_at FROM events
		 WHERE instance = ? AND created_at >= ?
		 ORDER BY id DESC LIMIT ?`, instanceID, since, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("events: by_instance: %w", err)
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.Instance, &e.Message, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("events: by_instance scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) SinceID(lastID uint64) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT id, type, instance, message, created_at FROM events WHERE id > ? ORDER BY id ASC`, lastID,
	)
	if err != nil {
		return nil, fmt.Errorf("events: since: %w", err)
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.Instance, &e.Message, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("events: since scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
