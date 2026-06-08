package registry

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("instance not found")

// Instance represents a provisioned Fox instance in the registry.
type Instance struct {
	ID            string `json:"id"`
	ImageDigest   string `json:"image_digest"`
	Port          int    `json:"port"`
	DataDir       string `json:"data_dir"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	SkillsetName  string `json:"skillset_name,omitempty"`
	PrincipalRole string `json:"principal_role,omitempty"`
	QueryToken    string `json:"-"`
}

// Registry is the SQLite-backed instance store.
type Registry struct {
	db *sql.DB
}

// Open creates or opens the registry database at the given path.
func Open(path string) (*Registry, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("registry: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("registry: enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("registry: set busy timeout: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Registry{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS instances (
		id              TEXT PRIMARY KEY,
		image_digest    TEXT NOT NULL,
		port            INTEGER NOT NULL UNIQUE,
		data_dir        TEXT NOT NULL,
		status          TEXT NOT NULL DEFAULT 'provisioning',
		created_at      TEXT NOT NULL,
		skillset_name   TEXT NOT NULL DEFAULT '',
		principal_role  TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return fmt.Errorf("registry: migrate: %w", err)
	}
	for _, col := range []string{"skillset_name", "principal_role", "query_token"} {
		_, _ = db.Exec(fmt.Sprintf(`ALTER TABLE instances ADD COLUMN %s TEXT NOT NULL DEFAULT ''`, col))
	}

	if err := backfillQueryTokens(db); err != nil {
		return fmt.Errorf("registry: backfill query tokens: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS signing_keys (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		key        BLOB NOT NULL,
		active     INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("registry: migrate signing_keys: %w", err)
	}

	return nil
}

// Create inserts a new instance or updates an existing one with the same ID
// (idempotent per conformance check 07).
func (r *Registry) Create(inst Instance) error {
	if inst.CreatedAt == "" {
		inst.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := r.db.Exec(`INSERT INTO instances (id, image_digest, port, data_dir, status, created_at, skillset_name, principal_role, query_token)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			image_digest = excluded.image_digest,
			port = excluded.port,
			data_dir = excluded.data_dir,
			status = excluded.status,
			skillset_name = excluded.skillset_name,
			principal_role = excluded.principal_role,
			query_token = excluded.query_token`,
		inst.ID, inst.ImageDigest, inst.Port, inst.DataDir, inst.Status, inst.CreatedAt, inst.SkillsetName, inst.PrincipalRole, inst.QueryToken)
	if err != nil {
		return fmt.Errorf("registry: create %s: %w", inst.ID, err)
	}
	return nil
}

// Get returns a single instance by ID.
func (r *Registry) Get(id string) (Instance, error) {
	var inst Instance
	err := r.db.QueryRow(
		`SELECT id, image_digest, port, data_dir, status, created_at, skillset_name, principal_role, query_token FROM instances WHERE id = ?`, id,
	).Scan(&inst.ID, &inst.ImageDigest, &inst.Port, &inst.DataDir, &inst.Status, &inst.CreatedAt, &inst.SkillsetName, &inst.PrincipalRole, &inst.QueryToken)
	if errors.Is(err, sql.ErrNoRows) {
		return Instance{}, ErrNotFound
	}
	if err != nil {
		return Instance{}, fmt.Errorf("registry: get %s: %w", id, err)
	}
	return inst, nil
}

// List returns all instances ordered by creation time.
func (r *Registry) List() ([]Instance, error) {
	rows, err := r.db.Query(
		`SELECT id, image_digest, port, data_dir, status, created_at, skillset_name, principal_role, query_token FROM instances ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("registry: list: %w", err)
	}
	defer rows.Close()
	var out []Instance
	for rows.Next() {
		var inst Instance
		if err := rows.Scan(&inst.ID, &inst.ImageDigest, &inst.Port, &inst.DataDir, &inst.Status, &inst.CreatedAt, &inst.SkillsetName, &inst.PrincipalRole, &inst.QueryToken); err != nil {
			return nil, fmt.Errorf("registry: list scan: %w", err)
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

// UpdateStatus changes the status of an existing instance.
func (r *Registry) UpdateStatus(id, status string) error {
	res, err := r.db.Exec(`UPDATE instances SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("registry: update status %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateImageDigest changes the image digest of an existing instance.
func (r *Registry) UpdateImageDigest(id, digest string) error {
	res, err := r.db.Exec(`UPDATE instances SET image_digest = ? WHERE id = ?`, digest, id)
	if err != nil {
		return fmt.Errorf("registry: update image %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes an instance from the registry.
func (r *Registry) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM instances WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("registry: delete %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UsedPorts returns a set of ports currently allocated to instances.
func (r *Registry) UsedPorts() (map[int]bool, error) {
	rows, err := r.db.Query(`SELECT port FROM instances`)
	if err != nil {
		return nil, fmt.Errorf("registry: used ports: %w", err)
	}
	defer rows.Close()
	ports := make(map[int]bool)
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("registry: used ports scan: %w", err)
		}
		ports[p] = true
	}
	return ports, rows.Err()
}

// GenerateQueryToken produces a 32-byte random token as unpadded base64url.
func GenerateQueryToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("registry: generate query token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ValidQueryToken returns true if the token matches any instance's query_token.
func (r *Registry) ValidQueryToken(token string) bool {
	if token == "" {
		return false
	}
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM instances WHERE query_token = ? AND query_token != ''`,
		token,
	).Scan(&count)
	return err == nil && count > 0
}

// UpdateQueryToken sets the query_token for an existing instance.
func (r *Registry) UpdateQueryToken(id, token string) error {
	res, err := r.db.Exec(`UPDATE instances SET query_token = ? WHERE id = ?`, token, id)
	if err != nil {
		return fmt.Errorf("registry: update query token %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetQueryToken returns the query_token for a given instance.
func (r *Registry) GetQueryToken(id string) (string, error) {
	var token string
	err := r.db.QueryRow(`SELECT query_token FROM instances WHERE id = ?`, id).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("registry: get query token %s: %w", id, err)
	}
	return token, nil
}

func backfillQueryTokens(db *sql.DB) error {
	rows, err := db.Query(`SELECT id FROM instances WHERE query_token = ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range ids {
		token, err := GenerateQueryToken()
		if err != nil {
			return err
		}
		if _, err := db.Exec(`UPDATE instances SET query_token = ? WHERE id = ?`, token, id); err != nil {
			return err
		}
	}
	if len(ids) > 0 {
		slog.Info("backfilled query tokens", "count", len(ids))
	}
	return nil
}

// ActiveSigningKey returns the currently active HMAC signing key.
func (r *Registry) ActiveSigningKey() ([]byte, error) {
	var key []byte
	err := r.db.QueryRow(`SELECT key FROM signing_keys WHERE active = 1 ORDER BY id DESC LIMIT 1`).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("registry: no active signing key")
	}
	if err != nil {
		return nil, fmt.Errorf("registry: active signing key: %w", err)
	}
	return key, nil
}

// EnsureSigningKey returns the active signing key, creating one if none exists.
func (r *Registry) EnsureSigningKey() ([]byte, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("registry: begin tx: %w", err)
	}
	defer tx.Rollback()

	var key []byte
	err = tx.QueryRow(`SELECT key FROM signing_keys WHERE active = 1 ORDER BY id DESC LIMIT 1`).Scan(&key)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("registry: active signing key: %w", err)
	}

	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("registry: generate signing key: %w", err)
	}
	_, err = tx.Exec(
		`INSERT INTO signing_keys (key, active, created_at) VALUES (?, 1, ?)`,
		newKey, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("registry: insert signing key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("registry: commit signing key: %w", err)
	}
	return newKey, nil
}

// RotateSigningKey deactivates the current key and creates a new one.
func (r *Registry) RotateSigningKey() ([]byte, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("registry: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE signing_keys SET active = 0`); err != nil {
		return nil, fmt.Errorf("registry: deactivate signing keys: %w", err)
	}
	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("registry: generate signing key: %w", err)
	}
	_, err = tx.Exec(
		`INSERT INTO signing_keys (key, active, created_at) VALUES (?, 1, ?)`,
		newKey, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("registry: insert rotated signing key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("registry: commit rotated signing key: %w", err)
	}
	return newKey, nil
}

// Close closes the database connection.
func (r *Registry) Close() error {
	return r.db.Close()
}
