package storage

import (
	"context"
	"fmt"

	"github.com/joaquimrafael/go-chain/internal/transaction"
)

// PendingTransaction keeps persistence metadata separate from blockchain data.
// Only Transaction is included when constructing a block for mining.
type PendingTransaction struct {
	ID          int64
	CreatedAt   int64
	Transaction transaction.Transaction
}

// AddPending saves a transfer in an initialized database and returns its metadata.
// The caller supplies a Unix timestamp and must validate the transfer's accounts,
// amount, and available funds with transaction.ValidateTransfer before insertion.
// Storage rejects other types because the pending table represents transfers only.
func (s *Store) AddPending(ctx context.Context, transfer transaction.Transaction, timestamp int64) (PendingTransaction, error) {
	if transfer.Type != transaction.Transfer {
		return PendingTransaction{}, fmt.Errorf("pending transaction type must be transfer, got %q", transfer.Type)
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO pending_transactions (sender, receiver, amount, created_at)
		VALUES (?, ?, ?, ?)`, transfer.From, transfer.To, transfer.Amount, timestamp)
	if err != nil {
		return PendingTransaction{}, fmt.Errorf("add pending transaction: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return PendingTransaction{}, fmt.Errorf("read pending transaction ID: %w", err)
	}
	return PendingTransaction{ID: id, CreatedAt: timestamp, Transaction: transfer}, nil
}

// LoadPending reads transfers from an initialized database in ascending ID order.
// An empty pool returns a non-nil empty slice. Loading does not validate funding.
func (s *Store) LoadPending(ctx context.Context) ([]PendingTransaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, sender, receiver, amount, created_at
		FROM pending_transactions ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load pending transactions: %w", err)
	}
	defer rows.Close()
	pending := make([]PendingTransaction, 0)
	for rows.Next() {
		var entry PendingTransaction
		entry.Transaction.Type = transaction.Transfer
		if err := rows.Scan(&entry.ID, &entry.Transaction.From, &entry.Transaction.To,
			&entry.Transaction.Amount, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("read pending transaction: %w", err)
		}
		pending = append(pending, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending transactions: %w", err)
	}
	return pending, nil
}
