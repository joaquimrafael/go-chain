package blockchain

import (
	"fmt"
	"math"

	"github.com/joaquimrafael/go-chain/internal/transaction"
)

// Balances reconstructs confirmed balances in block and transaction slice order.
// It checks replay arithmetic, not hashes, account existence, or reward rules.
// The returned map is derived state; it is rebuilt on every call and is never
// stored in the chain. An invalid transaction returns an error and no partial map.
func (bc Blockchain) Balances() (map[string]int64, error) {
	if len(bc.Blocks) == 0 {
		return nil, ErrEmptyBlockchain
	}
	balances := make(map[string]int64)
	for blockIndex, block := range bc.Blocks {
		for position, transfer := range block.Transactions {
			if err := applyBalanceTransaction(balances, transfer); err != nil {
				return nil, fmt.Errorf("block %d: transaction %d: %w", blockIndex, position, err)
			}
		}
	}
	return balances, nil
}

// Balance replays the complete chain and returns the confirmed balance for name.
// Names are matched exactly. No history means zero, which does not establish
// whether an account exists; callers check the account registry separately.
func (bc Blockchain) Balance(name string) (int64, error) {
	balances, err := bc.Balances()
	if err != nil {
		return 0, err
	}
	return balances[name], nil
}

func applyBalanceTransaction(balances map[string]int64, transfer transaction.Transaction) error {
	if transfer.Type != transaction.Reward && transfer.Type != transaction.Transfer {
		return fmt.Errorf("unknown transaction type %q", transfer.Type)
	}
	if transfer.Amount <= 0 {
		return fmt.Errorf("amount must be positive, got %d", transfer.Amount)
	}
	if transfer.Type == transaction.Transfer {
		if balances[transfer.From] < transfer.Amount {
			return fmt.Errorf("account %q has %d GOC, cannot spend %d GOC", transfer.From, balances[transfer.From], transfer.Amount)
		}
		balances[transfer.From] -= transfer.Amount
	}
	// Check before addition so signed integer overflow cannot corrupt a balance.
	if balances[transfer.To] > math.MaxInt64-transfer.Amount {
		return fmt.Errorf("balance for account %q exceeds int64 range", transfer.To)
	}
	balances[transfer.To] += transfer.Amount
	return nil
}
