// Package storage persists GoChain state in SQLite.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/joaquimrafael/go-chain/internal/blockchain"
	_ "modernc.org/sqlite"
)

var (
	ErrAlreadyInitialized = errors.New("blockchain is already initialized")
	ErrNotInitialized     = errors.New("blockchain is not initialized")
)

// Store owns a SQLite connection pool. Call Close when the command finishes.
type Store struct {
	db *sql.DB
}

// Open opens a database file without initializing its schema or blockchain.
func Open(ctx context.Context, path string) (*Store, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	// A URI safely handles filenames containing query characters. The driver
	// applies foreign_keys on every connection, including replacement connections.
	dsn := url.URL{Scheme: "file", Path: absPath}
	query := url.Values{"_pragma": {"foreign_keys(1)"}}
	dsn.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Nonces use decimal TEXT because SQLite INTEGER cannot hold all uint64 values.
const schema = `
CREATE TABLE IF NOT EXISTS accounts (
    name TEXT PRIMARY KEY NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS blocks (
    height INTEGER PRIMARY KEY,
    timestamp INTEGER NOT NULL,
    previous_hash TEXT NOT NULL,
    hash TEXT NOT NULL,
    nonce TEXT NOT NULL,
    difficulty INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS block_transactions (
    id INTEGER PRIMARY KEY,
    block_height INTEGER NOT NULL REFERENCES blocks(height),
    position INTEGER NOT NULL,
    type TEXT NOT NULL,
    sender TEXT NOT NULL,
    receiver TEXT NOT NULL,
    amount INTEGER NOT NULL,
    UNIQUE (block_height, position)
);
CREATE TABLE IF NOT EXISTS pending_transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sender TEXT NOT NULL,
    receiver TEXT NOT NULL,
    amount INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);`

// Initialize creates the schema and Genesis together, or leaves both unchanged.
// Timestamps are supplied by the caller to keep Genesis deterministic.
func (s *Store) Initialize(ctx context.Context, timestamp int64, difficulty int) error {
	genesis, err := blockchain.NewGenesisBlock(timestamp, difficulty)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin initialization: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM blocks)").Scan(&exists); err != nil {
		return fmt.Errorf("check initialization: %w", err)
	}
	if exists {
		return ErrAlreadyInitialized
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO blocks (height, timestamp, previous_hash, hash, nonce, difficulty)
		VALUES (?, ?, ?, ?, ?, ?)`,
		genesis.Height, genesis.Timestamp, genesis.PreviousHash, genesis.Hash,
		strconv.FormatUint(genesis.Nonce, 10), genesis.Difficulty); err != nil {
		return fmt.Errorf("store Genesis: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit initialization: %w", err)
	}
	return nil
}
