package registry

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
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
	// PlaneAuthToken and InstancePassword are stored in cleartext because the
	// proxy injects them as bearer credentials into outbound requests to Fox
	// instances. Unlike QueryToken (verified via hash), these must be
	// recoverable at runtime. The json:"-" tags prevent API serialization.
	PlaneAuthToken   string `json:"-"`
	InstancePassword string `json:"-"`
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
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("registry: enable foreign keys: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Registry{db: db}, nil
}

func schemaVersion(db *sql.DB) (int, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`)
	if err != nil {
		return 0, fmt.Errorf("registry: create schema_version: %w", err)
	}
	var v int
	err = db.QueryRow(`SELECT version FROM schema_version`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return v, err
}

func setSchemaVersion(tx *sql.Tx, v int) error {
	_, err := tx.Exec(`DELETE FROM schema_version`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO schema_version (version) VALUES (?)`, v)
	return err
}

var migrations = []func(tx *sql.Tx) error{
	func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS instances (
			id              TEXT PRIMARY KEY,
			image_digest    TEXT NOT NULL,
			port            INTEGER NOT NULL UNIQUE,
			data_dir        TEXT NOT NULL,
			status          TEXT NOT NULL DEFAULT 'provisioning',
			created_at      TEXT NOT NULL,
			skillset_name   TEXT NOT NULL DEFAULT '',
			principal_role  TEXT NOT NULL DEFAULT ''
		)`)
		return err
	},
	func(tx *sql.Tx) error {
		for _, col := range []string{"skillset_name", "principal_role", "query_token"} {
			_, _ = tx.Exec(fmt.Sprintf(`ALTER TABLE instances ADD COLUMN %s TEXT NOT NULL DEFAULT ''`, col))
		}
		return nil
	},
	func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS signing_keys (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			key        BLOB NOT NULL,
			active     INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL
		)`)
		return err
	},
	func(tx *sql.Tx) error {
		_, _ = tx.Exec(`ALTER TABLE instances ADD COLUMN query_token_hash TEXT NOT NULL DEFAULT ''`)
		_, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_instances_query_token_hash ON instances(query_token_hash)`)
		if err != nil {
			return err
		}
		rows, err := tx.Query(`SELECT id, query_token FROM instances WHERE query_token != '' AND query_token_hash = ''`)
		if err != nil {
			return err
		}
		defer rows.Close()
		type pair struct{ id, token string }
		var pairs []pair
		for rows.Next() {
			var p pair
			if err := rows.Scan(&p.id, &p.token); err != nil {
				return err
			}
			pairs = append(pairs, p)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, p := range pairs {
			h := tokenHash(p.token)
			if _, err := tx.Exec(`UPDATE instances SET query_token_hash = ? WHERE id = ?`, h, p.id); err != nil {
				return err
			}
		}
		return nil
	},
	func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS cloud_users (
			username       TEXT PRIMARY KEY COLLATE NOCASE,
			password_hash  TEXT NOT NULL,
			instance_id    TEXT REFERENCES instances(id) ON DELETE SET NULL,
			created_at     TEXT NOT NULL,
			updated_at     TEXT NOT NULL
		)`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_cloud_users_instance ON cloud_users(instance_id) WHERE instance_id IS NOT NULL`)
		return err
	},
	func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS cloud_sessions (
			token_hash  TEXT PRIMARY KEY,
			user_id     TEXT NOT NULL REFERENCES cloud_users(username) ON DELETE CASCADE,
			created_at  TEXT NOT NULL,
			expires_at  TEXT NOT NULL
		)`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_cloud_sessions_user ON cloud_sessions(user_id)`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_cloud_sessions_expires ON cloud_sessions(expires_at)`)
		return err
	},
	func(tx *sql.Tx) error {
		if _, err := tx.Exec(`ALTER TABLE instances ADD COLUMN plane_auth_token TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
		_, err := tx.Exec(`ALTER TABLE instances ADD COLUMN instance_password TEXT NOT NULL DEFAULT ''`)
		return err
	},
}

func migrate(db *sql.DB) error {
	current, err := schemaVersion(db)
	if err != nil {
		return fmt.Errorf("registry: read schema version: %w", err)
	}

	for i := current; i < len(migrations); i++ {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("registry: begin migration %d: %w", i+1, err)
		}
		if err := migrations[i](tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("registry: migration %d failed: %w", i+1, err)
		}
		if err := setSchemaVersion(tx, i+1); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("registry: set schema version %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("registry: commit migration %d: %w", i+1, err)
		}
		slog.Info("applied registry migration", "version", i+1)
	}

	if err := backfillQueryTokens(db); err != nil {
		return fmt.Errorf("registry: backfill query tokens: %w", err)
	}

	return nil
}

// Create inserts a new instance or updates an existing one with the same ID
// (idempotent per conformance check 07).
func (r *Registry) Create(inst Instance) error {
	if inst.CreatedAt == "" {
		inst.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	var hash string
	if inst.QueryToken != "" {
		hash = tokenHash(inst.QueryToken)
	}
	_, err := r.db.Exec(`INSERT INTO instances (id, image_digest, port, data_dir, status, created_at, skillset_name, principal_role, query_token, query_token_hash, plane_auth_token, instance_password)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			image_digest = excluded.image_digest,
			port = excluded.port,
			data_dir = excluded.data_dir,
			status = excluded.status,
			skillset_name = excluded.skillset_name,
			principal_role = excluded.principal_role,
			query_token = excluded.query_token,
			query_token_hash = excluded.query_token_hash,
			plane_auth_token = excluded.plane_auth_token,
			instance_password = excluded.instance_password`,
		inst.ID, inst.ImageDigest, inst.Port, inst.DataDir, inst.Status, inst.CreatedAt, inst.SkillsetName, inst.PrincipalRole, inst.QueryToken, hash, inst.PlaneAuthToken, inst.InstancePassword)
	if err != nil {
		return fmt.Errorf("registry: create %s: %w", inst.ID, err)
	}
	return nil
}

// Get returns a single instance by ID.
func (r *Registry) Get(id string) (Instance, error) {
	var inst Instance
	err := r.db.QueryRow(
		`SELECT id, image_digest, port, data_dir, status, created_at, skillset_name, principal_role, query_token, plane_auth_token, instance_password FROM instances WHERE id = ?`, id,
	).Scan(&inst.ID, &inst.ImageDigest, &inst.Port, &inst.DataDir, &inst.Status, &inst.CreatedAt, &inst.SkillsetName, &inst.PrincipalRole, &inst.QueryToken, &inst.PlaneAuthToken, &inst.InstancePassword)
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
		`SELECT id, image_digest, port, data_dir, status, created_at, skillset_name, principal_role, query_token, plane_auth_token, instance_password FROM instances ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("registry: list: %w", err)
	}
	defer rows.Close()
	var out []Instance
	for rows.Next() {
		var inst Instance
		if err := rows.Scan(&inst.ID, &inst.ImageDigest, &inst.Port, &inst.DataDir, &inst.Status, &inst.CreatedAt, &inst.SkillsetName, &inst.PrincipalRole, &inst.QueryToken, &inst.PlaneAuthToken, &inst.InstancePassword); err != nil {
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

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
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
// Uses hash-indexed lookup (O(1)) followed by constant-time comparison.
func (r *Registry) ValidQueryToken(token string) bool {
	if token == "" {
		return false
	}
	h := tokenHash(token)
	var stored string
	err := r.db.QueryRow(`SELECT query_token FROM instances WHERE query_token_hash = ?`, h).Scan(&stored)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(stored)) == 1
}

// UpdateQueryToken sets the query_token for an existing instance.
func (r *Registry) UpdateQueryToken(id, token string) error {
	hash := tokenHash(token)
	res, err := r.db.Exec(`UPDATE instances SET query_token = ?, query_token_hash = ? WHERE id = ?`, token, hash, id)
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
		hash := tokenHash(token)
		if _, err := db.Exec(`UPDATE instances SET query_token = ?, query_token_hash = ? WHERE id = ?`, token, hash, id); err != nil {
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
	defer func() { _ = tx.Rollback() }()

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
	defer func() { _ = tx.Rollback() }()

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

// DB returns the underlying database connection for use by other stores
// that share the same SQLite file (e.g. cloud user/session stores).
func (r *Registry) DB() *sql.DB {
	return r.db
}

// Close closes the database connection.
func (r *Registry) Close() error {
	return r.db.Close()
}
