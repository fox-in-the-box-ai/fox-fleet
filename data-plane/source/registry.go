package source

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("source not found")

type Source struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Collection  string            `json:"collection"`
	Status      string            `json:"status"`
	DocCount    int               `json:"doc_count"`
	ChunkCount  int               `json:"chunk_count"`
	ErrorDetail string            `json:"error_detail,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

type Registry struct {
	db *sql.DB
}

func OpenRegistry(db *sql.DB) (*Registry, error) {
	if err := migrate(db); err != nil {
		return nil, err
	}
	return &Registry{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS sources (
		id           TEXT PRIMARY KEY,
		type         TEXT NOT NULL,
		name         TEXT NOT NULL,
		collection   TEXT NOT NULL,
		status       TEXT NOT NULL DEFAULT 'pending',
		doc_count    INTEGER NOT NULL DEFAULT 0,
		chunk_count  INTEGER NOT NULL DEFAULT 0,
		error_detail TEXT NOT NULL DEFAULT '',
		config       TEXT NOT NULL DEFAULT '{}',
		created_at   TEXT NOT NULL,
		updated_at   TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("source: migrate sources: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS document_tracking (
		source_id    TEXT NOT NULL,
		doc_id       TEXT NOT NULL,
		content_hash TEXT NOT NULL,
		updated_at   TEXT NOT NULL,
		PRIMARY KEY (source_id, doc_id)
	)`)
	if err != nil {
		return fmt.Errorf("source: migrate document_tracking: %w", err)
	}
	return nil
}

func (r *Registry) GetDocHash(sourceID, docID string) (string, error) {
	var hash string
	err := r.db.QueryRow(
		`SELECT content_hash FROM document_tracking WHERE source_id = ? AND doc_id = ?`,
		sourceID, docID,
	).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (r *Registry) SetDocHash(sourceID, docID, hash string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(
		`INSERT INTO document_tracking (source_id, doc_id, content_hash, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (source_id, doc_id) DO UPDATE SET content_hash = excluded.content_hash, updated_at = excluded.updated_at`,
		sourceID, docID, hash, now,
	)
	return err
}

func (r *Registry) ListDocIDs(sourceID string) ([]string, error) {
	rows, err := r.db.Query(`SELECT doc_id FROM document_tracking WHERE source_id = ?`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Registry) DeleteDocTracking(sourceID, docID string) error {
	_, err := r.db.Exec(`DELETE FROM document_tracking WHERE source_id = ? AND doc_id = ?`, sourceID, docID)
	return err
}

func (r *Registry) DeleteAllDocTracking(sourceID string) error {
	_, err := r.db.Exec(`DELETE FROM document_tracking WHERE source_id = ?`, sourceID)
	return err
}

func (r *Registry) Create(src Source) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if src.CreatedAt == "" {
		src.CreatedAt = now
	}
	if src.UpdatedAt == "" {
		src.UpdatedAt = now
	}
	if src.Status == "" {
		src.Status = "pending"
	}
	cfgJSON, err := marshalConfig(src.Config)
	if err != nil {
		return fmt.Errorf("source: create %s: %w", src.ID, err)
	}
	_, err = r.db.Exec(`INSERT INTO sources (id, type, name, collection, status, doc_count, chunk_count, error_detail, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		src.ID, src.Type, src.Name, src.Collection, src.Status,
		src.DocCount, src.ChunkCount, src.ErrorDetail, cfgJSON,
		src.CreatedAt, src.UpdatedAt)
	if err != nil {
		return fmt.Errorf("source: create %s: %w", src.ID, err)
	}
	return nil
}

func (r *Registry) Get(id string) (Source, error) {
	var s Source
	var cfgJSON string
	err := r.db.QueryRow(
		`SELECT id, type, name, collection, status, doc_count, chunk_count, error_detail, config, created_at, updated_at
		 FROM sources WHERE id = ?`, id,
	).Scan(&s.ID, &s.Type, &s.Name, &s.Collection, &s.Status,
		&s.DocCount, &s.ChunkCount, &s.ErrorDetail, &cfgJSON,
		&s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{}, ErrNotFound
	}
	if err != nil {
		return Source{}, fmt.Errorf("source: get %s: %w", id, err)
	}
	s.Config = unmarshalConfig(cfgJSON)
	return s, nil
}

func (r *Registry) List() ([]Source, error) {
	rows, err := r.db.Query(
		`SELECT id, type, name, collection, status, doc_count, chunk_count, error_detail, config, created_at, updated_at
		 FROM sources ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("source: list: %w", err)
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var s Source
		var cfgJSON string
		if err := rows.Scan(&s.ID, &s.Type, &s.Name, &s.Collection, &s.Status,
			&s.DocCount, &s.ChunkCount, &s.ErrorDetail, &cfgJSON,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("source: list scan: %w", err)
		}
		s.Config = unmarshalConfig(cfgJSON)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Registry) UpdateStatus(id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`UPDATE sources SET status = ?, updated_at = ? WHERE id = ?`, status, now, id)
	if err != nil {
		return fmt.Errorf("source: update status %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Registry) UpdateCounts(id string, docCount, chunkCount int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(
		`UPDATE sources SET doc_count = ?, chunk_count = ?, updated_at = ? WHERE id = ?`,
		docCount, chunkCount, now, id)
	if err != nil {
		return fmt.Errorf("source: update counts %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Registry) SetError(id, detail string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(
		`UPDATE sources SET status = 'error', error_detail = ?, updated_at = ? WHERE id = ?`,
		detail, now, id)
	if err != nil {
		return fmt.Errorf("source: set error %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Registry) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("source: delete %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func marshalConfig(cfg map[string]string) (string, error) {
	if len(cfg) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}
	return string(b), nil
}

func unmarshalConfig(s string) map[string]string {
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}
