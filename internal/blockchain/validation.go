package blockchain

import (
	"fmt"
	"strings"

	"github.com/joaquimrafael/go-chain/internal/transaction"
)

// ValidateStructure checks hashes, links, heights, and fixed difficulty in chain
// order. It does not validate transaction, account, reward, or balance rules,
// except that Genesis must contain no transactions.
func ValidateStructure(chain Blockchain) error {
	if len(chain.Blocks) == 0 {
		return ErrEmptyBlockchain
	}

	for i := range chain.Blocks {
		if err := validateBlockStructure(chain, i); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks structure and replays economic rules one block at a time.
// Account names are matched exactly against the supplied registry. Errors identify
// the first invalid block and, for transaction rules, its zero-based position.
// The chain and registry are never changed; replay balances are local derived state.
func Validate(chain Blockchain, accountNames map[string]struct{}) error {
	if len(chain.Blocks) == 0 {
		return ErrEmptyBlockchain
	}
	balances := make(map[string]int64)
	for i, block := range chain.Blocks {
		if err := validateBlockStructure(chain, i); err != nil {
			return err
		}
		if i == 0 {
			continue
		}
		if len(block.Transactions) == 0 || block.Transactions[0].Type != transaction.Reward {
			return fmt.Errorf("block %d: first transaction must be a mining reward", i)
		}
		for position, transfer := range block.Transactions {
			if err := validateConfirmedTransaction(transfer, position, accountNames, balances); err != nil {
				return fmt.Errorf("block %d: transaction %d: %w", i, position, err)
			}
			if err := applyBalanceTransaction(balances, transfer); err != nil {
				return fmt.Errorf("block %d: transaction %d: %w", i, position, err)
			}
		}
	}
	return nil
}

func validateConfirmedTransaction(transfer transaction.Transaction, position int, accounts map[string]struct{}, balances map[string]int64) error {
	switch transfer.Type {
	case transaction.Reward:
		if position != 0 {
			return fmt.Errorf("mining reward must occur exactly once, at position 0")
		}
		if transfer.From != "" {
			return fmt.Errorf("mining reward sender must be empty")
		}
		if transfer.Amount != transaction.RewardAmount {
			return fmt.Errorf("mining reward amount must be %d GOC, got %d", transaction.RewardAmount, transfer.Amount)
		}
		if _, exists := accounts[transfer.To]; !exists {
			return fmt.Errorf("receiver account %q does not exist", transfer.To)
		}
		return nil
	case transaction.Transfer:
		// Replay uses funds confirmed up to this position, with no pending pool.
		return transaction.ValidateTransfer(transfer, accounts, balances, nil)
	default:
		return fmt.Errorf("unknown transaction type %q", transfer.Type)
	}
}

func validateBlockStructure(chain Blockchain, i int) error {
	block := chain.Blocks[i]
	difficulty := chain.Blocks[0].Difficulty
	// Use the slice position in errors because the stored height may be corrupt.
	if block.Height != int64(i) {
		return fmt.Errorf("block %d: height is %d, want %d", i, block.Height, i)
	}
	if block.Difficulty < MinDifficulty || block.Difficulty > MaxDifficulty {
		return fmt.Errorf("block %d: difficulty must be between %d and %d", i, MinDifficulty, MaxDifficulty)
	}
	if block.Difficulty != difficulty {
		return fmt.Errorf("block %d: difficulty is %d, want Genesis difficulty %d", i, block.Difficulty, difficulty)
	}

	if i == 0 {
		if block.PreviousHash != GenesisPreviousHash {
			return fmt.Errorf("block 0: Genesis previous hash must be 64 zeroes")
		}
		if len(block.Transactions) != 0 {
			return fmt.Errorf("block 0: Genesis must contain no transactions")
		}
	} else if block.PreviousHash != chain.Blocks[i-1].Hash {
		return fmt.Errorf("block %d: previous hash does not match block %d hash", i, i-1)
	}

	if block.Hash != block.CalculateHash() {
		return fmt.Errorf("block %d: stored hash does not match calculated hash", i)
	}
	// Genesis is hashed normally but is exempt from Proof of Work.
	if i > 0 && !strings.HasPrefix(block.Hash, strings.Repeat("0", difficulty)) {
		return fmt.Errorf("block %d: hash does not satisfy difficulty %d", i, difficulty)
	}
	return nil
}
