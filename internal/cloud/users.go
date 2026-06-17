package cloud

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

const bcryptCost = 12

type User struct {
	Username   string  `json:"username"`
	InstanceID *string `json:"instance_id"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) Create(username, password string) (User, error) {
	if username == "" {
		return User{}, fmt.Errorf("cloud: username must not be empty")
	}
	if password == "" {
		return User{}, fmt.Errorf("cloud: password must not be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return User{}, fmt.Errorf("cloud: hash password: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(
		`INSERT INTO cloud_users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		username, string(hash), now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUserExists
		}
		return User{}, fmt.Errorf("cloud: create user %s: %w", username, err)
	}

	return User{
		Username:  username,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *UserStore) Get(username string) (User, error) {
	var u User
	var instID sql.NullString
	err := s.db.QueryRow(
		`SELECT username, instance_id, created_at, updated_at FROM cloud_users WHERE username = ?`,
		username,
	).Scan(&u.Username, &instID, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("cloud: get user %s: %w", username, err)
	}
	if instID.Valid {
		u.InstanceID = &instID.String
	}
	return u, nil
}

func (s *UserStore) List() ([]User, error) {
	rows, err := s.db.Query(
		`SELECT username, instance_id, created_at, updated_at FROM cloud_users ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("cloud: list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var instID sql.NullString
		if err := rows.Scan(&u.Username, &instID, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("cloud: list users scan: %w", err)
		}
		if instID.Valid {
			u.InstanceID = &instID.String
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *UserStore) Update(username string, password *string, instanceID *string) (User, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return User{}, fmt.Errorf("cloud: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existing int
	err = tx.QueryRow(`SELECT 1 FROM cloud_users WHERE username = ?`, username).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("cloud: update user check: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if password != nil {
		if *password == "" {
			return User{}, fmt.Errorf("cloud: password must not be empty")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcryptCost)
		if err != nil {
			return User{}, fmt.Errorf("cloud: hash password: %w", err)
		}
		if _, err := tx.Exec(`UPDATE cloud_users SET password_hash = ?, updated_at = ? WHERE username = ?`, string(hash), now, username); err != nil {
			return User{}, fmt.Errorf("cloud: update password: %w", err)
		}
	}

	if instanceID != nil {
		if *instanceID == "" {
			if _, err := tx.Exec(`UPDATE cloud_users SET instance_id = NULL, updated_at = ? WHERE username = ?`, now, username); err != nil {
				return User{}, fmt.Errorf("cloud: clear instance_id: %w", err)
			}
		} else {
			if _, err := tx.Exec(`UPDATE cloud_users SET instance_id = ?, updated_at = ? WHERE username = ?`, *instanceID, now, username); err != nil {
				return User{}, fmt.Errorf("cloud: update instance_id: %w", err)
			}
		}
	}

	if password == nil && instanceID == nil {
		if _, err := tx.Exec(`UPDATE cloud_users SET updated_at = ? WHERE username = ?`, now, username); err != nil {
			return User{}, fmt.Errorf("cloud: touch updated_at: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("cloud: commit update: %w", err)
	}

	return s.Get(username)
}

func (s *UserStore) Delete(username string) error {
	res, err := s.db.Exec(`DELETE FROM cloud_users WHERE username = ?`, username)
	if err != nil {
		return fmt.Errorf("cloud: delete user %s: %w", username, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// Authenticate verifies the password for a user. Returns the User on success.
// Uses bcrypt comparison which provides constant-time behavior.
func (s *UserStore) Authenticate(username, password string) (User, error) {
	var u User
	var instID sql.NullString
	var hash string
	err := s.db.QueryRow(
		`SELECT username, password_hash, instance_id, created_at, updated_at FROM cloud_users WHERE username = ?`,
		username,
	).Scan(&u.Username, &hash, &instID, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// Perform a dummy bcrypt comparison to prevent timing-based username enumeration
		_ = bcrypt.CompareHashAndPassword(
			[]byte("$2a$12$000000000000000000000uGOCOSrFCmMUONHnnmiMGKjOqSUqH5G"),
			[]byte(password),
		)
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("cloud: authenticate %s: %w", username, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return User{}, ErrInvalidCredentials
	}

	if instID.Valid {
		u.InstanceID = &instID.String
	}
	return u, nil
}

// SetInstanceID assigns or clears the instance_id for a user.
func (s *UserStore) SetInstanceID(username, instanceID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var res sql.Result
	var err error
	if instanceID == "" {
		res, err = s.db.Exec(`UPDATE cloud_users SET instance_id = NULL, updated_at = ? WHERE username = ?`, now, username)
	} else {
		res, err = s.db.Exec(`UPDATE cloud_users SET instance_id = ?, updated_at = ? WHERE username = ?`, instanceID, now, username)
	}
	if err != nil {
		return fmt.Errorf("cloud: set instance_id for %s: %w", username, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "unique constraint")
}
