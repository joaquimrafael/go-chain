package mining

import (
	"context"
	"fmt"

	"github.com/joaquimrafael/go-chain/internal/blockchain"
	"github.com/joaquimrafael/go-chain/internal/transaction"
)

// MineBlock validates a snapshot and returns its next mined block without I/O.
// Pending contains domain transfers in queue order, without storage metadata.
// Inputs are not changed and must not be mutated concurrently. The returned
// block owns its transaction slice; any error returns a zero block.
func MineBlock(ctx context.Context, chain blockchain.Blockchain, pending []transaction.Transaction, miner string, accounts map[string]struct{}, timestamp int64) (blockchain.Block, error) {
	if err := ctx.Err(); err != nil {
		return blockchain.Block{}, err
	}
	if err := blockchain.Validate(chain, accounts); err != nil {
		return blockchain.Block{}, fmt.Errorf("validate existing chain: %w", err)
	}
	if _, exists := accounts[miner]; !exists {
		return blockchain.Block{}, fmt.Errorf("miner account %q does not exist", miner)
	}
	confirmed, err := chain.Balances()
	if err != nil {
		return blockchain.Block{}, fmt.Errorf("replay confirmed balances: %w", err)
	}
	for i, transfer := range pending {
		if err := ctx.Err(); err != nil {
			return blockchain.Block{}, err
		}
		// Reserve earlier outgoing transfers against pre-mining funds. Neither
		// incoming pending transfers nor this block's reward add spendable funds.
		if err := transaction.ValidateTransfer(transfer, accounts, confirmed, pending[:i]); err != nil {
			return blockchain.Block{}, fmt.Errorf("pending transaction %d: %w", i, err)
		}
	}

	// Validation guarantees a nonempty chain with sequential heights and a
	// fixed difficulty, so the tip supplies the next block's linking fields.
	tip := chain.Blocks[len(chain.Blocks)-1]
	transactions := make([]transaction.Transaction, 1, len(pending)+1)
	transactions[0] = transaction.Transaction{Type: transaction.Reward, To: miner, Amount: transaction.RewardAmount}
	transactions = append(transactions, pending...)
	candidate := blockchain.Block{
		Height: tip.Height + 1, Timestamp: timestamp, PreviousHash: tip.Hash,
		Difficulty: tip.Difficulty, Transactions: transactions,
	}
	mined, err := ProofOfWork(ctx, candidate)
	if err != nil {
		return blockchain.Block{}, fmt.Errorf("mine candidate: %w", err)
	}
	// Allocate a separate slice so appending cannot overwrite the caller's
	// backing array, even when its chain slice has spare capacity.
	blocks := make([]blockchain.Block, len(chain.Blocks)+1)
	copy(blocks, chain.Blocks)
	blocks[len(chain.Blocks)] = mined
	if err := blockchain.Validate(blockchain.Blockchain{Blocks: blocks}, accounts); err != nil {
		return blockchain.Block{}, fmt.Errorf("validate mined chain: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return blockchain.Block{}, err
	}
	return mined, nil
}
