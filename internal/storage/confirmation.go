package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/joaquimrafael/go-chain/internal/blockchain"
	"github.com/joaquimrafael/go-chain/internal/transaction"
)

// ConfirmMinedBlock atomically stores a mined block and removes its pending rows.
// The caller must first validate the block and chain (normally with MineBlock).
// IDs must correspond, in order, to the transfers following the first reward.
// This method rechecks the persisted tip and pending contents, but does not
// repeat full blockchain validation. Mining must finish before this call.
func (s *Store) ConfirmMinedBlock(ctx context.Context, block blockchain.Block, includedPendingIDs []int64) error {
	if len(block.Transactions) == 0 || block.Transactions[0].Type != transaction.Reward {
		return fmt.Errorf("confirmation requires a first-position reward")
	}
	if len(includedPendingIDs) != len(block.Transactions)-1 {
		return fmt.Errorf("pending ID count must match block transfer count")
	}
	for _, transfer := range block.Transactions[1:] {
		if transfer.Type != transaction.Transfer {
			return fmt.Errorf("confirmation requires transfers after the reward")
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin mining confirmation: %w", err)
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = 'blocks')`).Scan(&exists); err != nil {
		return fmt.Errorf("check confirmation schema: %w", err)
	}
	if !exists {
		return ErrNotInitialized
	}
	var height int64
	var hash string
	if err := tx.QueryRowContext(ctx, `SELECT height, hash FROM blocks ORDER BY height DESC LIMIT 1`).Scan(&height, &hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotInitialized
		}
		return fmt.Errorf("read confirmation tip: %w", err)
	}
	if block.Height <= 0 || block.Height-1 != height || block.PreviousHash != hash {
		return fmt.Errorf("stale mining candidate: block %d does not extend stored tip %d (%s)", block.Height, height, hash)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO blocks (height, timestamp, previous_hash, hash, nonce, difficulty)
		VALUES (?, ?, ?, ?, ?, ?)`, block.Height, block.Timestamp, block.PreviousHash,
		block.Hash, strconv.FormatUint(block.Nonce, 10), block.Difficulty); err != nil {
		return fmt.Errorf("store mined block: %w", err)
	}
	for position, transfer := range block.Transactions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO block_transactions (block_height, position, type, sender, receiver, amount)
			VALUES (?, ?, ?, ?, ?, ?)`, block.Height, position, transfer.Type,
			transfer.From, transfer.To, transfer.Amount); err != nil {
			return fmt.Errorf("store block %d transaction %d: %w", block.Height, position, err)
		}
	}
	for i, id := range includedPendingIDs {
		transfer := block.Transactions[i+1]
		// Matching contents as well as ID prevents confirming one transfer while
		// deleting another. Missing, changed, or repeated IDs roll back all writes.
		result, err := tx.ExecContext(ctx, `DELETE FROM pending_transactions
			WHERE id = ? AND sender = ? AND receiver = ? AND amount = ?`,
			id, transfer.From, transfer.To, transfer.Amount)
		if err != nil {
			return fmt.Errorf("delete pending transaction %d: %w", id, err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check pending transaction %d deletion: %w", id, err)
		}
		if count != 1 {
			return fmt.Errorf("pending transaction %d is missing or does not match the mined transfer", id)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mining confirmation: %w", err)
	}
	return nil
}
