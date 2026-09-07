package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/joaquimrafael/go-chain/internal/blockchain"
	"github.com/joaquimrafael/go-chain/internal/transaction"
)

// LoadChain reconstructs the stored chain in height and transaction order.
// It preserves stored hashes; callers validate the chain separately.
func (s *Store) LoadChain(ctx context.Context) (blockchain.Blockchain, error) {
	// One SQL transaction keeps the block and transaction reads in one snapshot.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return blockchain.Blockchain{}, fmt.Errorf("begin chain load: %w", err)
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = 'blocks')`).Scan(&exists); err != nil {
		return blockchain.Blockchain{}, fmt.Errorf("check schema: %w", err)
	}
	if !exists {
		return blockchain.Blockchain{}, ErrNotInitialized
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT height, timestamp, previous_hash, hash, nonce, difficulty FROM blocks ORDER BY height`)
	if err != nil {
		return blockchain.Blockchain{}, fmt.Errorf("load blocks: %w", err)
	}
	defer rows.Close()
	var chain blockchain.Blockchain
	for rows.Next() {
		var block blockchain.Block
		var nonce string
		if err := rows.Scan(&block.Height, &block.Timestamp, &block.PreviousHash, &block.Hash, &nonce, &block.Difficulty); err != nil {
			return blockchain.Blockchain{}, fmt.Errorf("read block: %w", err)
		}
		block.Nonce, err = strconv.ParseUint(nonce, 10, 64)
		if err != nil {
			return blockchain.Blockchain{}, fmt.Errorf("block %d: invalid nonce: %w", block.Height, err)
		}
		chain.Blocks = append(chain.Blocks, block)
	}
	if err := rows.Err(); err != nil {
		return blockchain.Blockchain{}, fmt.Errorf("read blocks: %w", err)
	}
	if err := rows.Close(); err != nil {
		return blockchain.Blockchain{}, fmt.Errorf("close block rows: %w", err)
	}
	if len(chain.Blocks) == 0 {
		return blockchain.Blockchain{}, ErrNotInitialized
	}
	for i := range chain.Blocks {
		block := &chain.Blocks[i]
		// Close each result before starting the next query on this connection.
		transactions, err := loadTransactions(ctx, tx, block.Height)
		if err != nil {
			return blockchain.Blockchain{}, err
		}
		block.Transactions = transactions
	}
	if err := tx.Commit(); err != nil {
		return blockchain.Blockchain{}, fmt.Errorf("finish chain load: %w", err)
	}
	return chain, nil
}

func loadTransactions(ctx context.Context, tx *sql.Tx, height int64) ([]transaction.Transaction, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT type, sender, receiver, amount FROM block_transactions
		WHERE block_height = ? ORDER BY position`, height)
	if err != nil {
		return nil, fmt.Errorf("block %d: load transactions: %w", height, err)
	}
	defer rows.Close()
	// Genesis must reload as [] rather than nil: JSON hashing distinguishes them.
	transactions := make([]transaction.Transaction, 0)
	for rows.Next() {
		var transfer transaction.Transaction
		if err := rows.Scan(&transfer.Type, &transfer.From, &transfer.To, &transfer.Amount); err != nil {
			return nil, fmt.Errorf("block %d: read transaction: %w", height, err)
		}
		transactions = append(transactions, transfer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("block %d: read transactions: %w", height, err)
	}
	return transactions, nil
}
