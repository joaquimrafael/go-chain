package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidAccountName   = errors.New("account name must not be blank")
	ErrAccountAlreadyExists = errors.New("account already exists")
)

// NormalizeAccountName trims surrounding whitespace and rejects blank names.
// Case and internal whitespace are preserved.
func NormalizeAccountName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalidAccountName
	}
	return name, nil
}

// CreateAccount saves an account in an initialized database and returns its
// normalized name. The caller supplies the Unix creation timestamp.
func (s *Store) CreateAccount(ctx context.Context, name string, timestamp int64) (string, error) {
	name, err := NormalizeAccountName(name)
	if err != nil {
		return "", err
	}
	// Let the unique constraint decide whether the account already exists.
	// A separate existence check could race with another CLI process.
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO accounts (name, created_at) VALUES (?, ?)
		ON CONFLICT(name) DO NOTHING`, name, timestamp)
	if err != nil {
		return "", fmt.Errorf("create account %q: %w", name, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("check account insertion: %w", err)
	}
	if count == 0 {
		return "", fmt.Errorf("%w: %q", ErrAccountAlreadyExists, name)
	}
	return name, nil
}

// AccountExists checks a normalized name in an initialized database.
// A missing account returns false; a blank name returns ErrInvalidAccountName.
func (s *Store) AccountExists(ctx context.Context, name string) (bool, error) {
	name, err := NormalizeAccountName(name)
	if err != nil {
		return false, err
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM accounts WHERE name = ?)", name).Scan(&exists); err != nil {
		return false, fmt.Errorf("check account %q: %w", name, err)
	}
	return exists, nil
}

// LoadAccountNames loads the current account names as a set from an initialized
// database. It does not load or calculate balances.
func (s *Store) LoadAccountNames(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT name FROM accounts")
	if err != nil {
		return nil, fmt.Errorf("load account names: %w", err)
	}
	defer rows.Close()
	names := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("read account name: %w", err)
		}
		names[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read account names: %w", err)
	}
	return names, nil
}
