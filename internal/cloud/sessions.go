package cloud

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

var ErrSessionNotFound = errors.New("session not found")

type Session struct {
	TokenHash string `json:"token_hash"`
	UserID    string `json:"user_id"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) Create(userID string, ttl time.Duration) (token string, session Session, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", Session{}, fmt.Errorf("cloud: generate session token: %w", err)
	}

	token = hex.EncodeToString(raw)
	hash := sessionHash(token)
	now := time.Now().UTC()
	createdAt := now.Format(time.RFC3339)
	expiresAt := now.Add(ttl).Format(time.RFC3339)

	_, err = s.db.Exec(
		`INSERT INTO cloud_sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		hash, userID, createdAt, expiresAt,
	)
	if err != nil {
		return "", Session{}, fmt.Errorf("cloud: create session: %w", err)
	}

	return token, Session{
		TokenHash: hash,
		UserID:    userID,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *SessionStore) Validate(token string) (Session, error) {
	hash := sessionHash(token)
	var sess Session
	err := s.db.QueryRow(
		`SELECT token_hash, user_id, created_at, expires_at FROM cloud_sessions WHERE token_hash = ?`,
		hash,
	).Scan(&sess.TokenHash, &sess.UserID, &sess.CreatedAt, &sess.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("cloud: validate session: %w", err)
	}

	expiresAt, err := time.Parse(time.RFC3339, sess.ExpiresAt)
	if err != nil {
		return Session{}, fmt.Errorf("cloud: parse expires_at: %w", err)
	}
	if time.Now().UTC().After(expiresAt) {
		_ = s.Delete(token)
		return Session{}, ErrSessionNotFound
	}

	return sess, nil
}

func (s *SessionStore) Delete(token string) error {
	hash := sessionHash(token)
	res, err := s.db.Exec(`DELETE FROM cloud_sessions WHERE token_hash = ?`, hash)
	if err != nil {
		return fmt.Errorf("cloud: delete session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (s *SessionStore) DeleteByUser(userID string) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM cloud_sessions WHERE user_id = ?`, userID)
	if err != nil {
		return 0, fmt.Errorf("cloud: delete sessions for user %s: %w", userID, err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *SessionStore) PurgeExpired() (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`DELETE FROM cloud_sessions WHERE expires_at < ?`, now)
	if err != nil {
		return 0, fmt.Errorf("cloud: purge expired sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *SessionStore) StartPurge(ctx context.Context, interval time.Duration, log *slog.Logger) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := s.PurgeExpired()
				if err != nil {
					log.Error("session purge failed", "error", err)
				} else if n > 0 {
					log.Info("purged expired sessions", "count", n)
				}
			}
		}
	}()
}

func sessionHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
