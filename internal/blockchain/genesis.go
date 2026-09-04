package blockchain

import (
	"fmt"

	"github.com/joaquimrafael/go-chain/internal/transaction"
)

const (
	GenesisHeight       int64  = 0
	GenesisNonce        uint64 = 0
	GenesisPreviousHash        = "0000000000000000000000000000000000000000000000000000000000000000"
	MinDifficulty              = 1
	MaxDifficulty              = 64
)

// NewGenesisBlock creates the deterministic first block in a GoChain.
// Genesis is hashed normally, but it does not contain transactions or require
// Proof of Work.
func NewGenesisBlock(timestamp int64, difficulty int) (Block, error) {
	if difficulty < MinDifficulty || difficulty > MaxDifficulty {
		return Block{}, fmt.Errorf("difficulty must be between %d and %d", MinDifficulty, MaxDifficulty)
	}

	block := Block{
		Height:       GenesisHeight,
		Timestamp:    timestamp,
		PreviousHash: GenesisPreviousHash,
		Nonce:        GenesisNonce,
		Difficulty:   difficulty,
		Transactions: make([]transaction.Transaction, 0),
	}
	block.Hash = block.CalculateHash()

	return block, nil
}
