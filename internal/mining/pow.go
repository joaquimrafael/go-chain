// Package mining implements the sequential Proof-of-Work search.
package mining

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/joaquimrafael/go-chain/internal/blockchain"
)

// ProofOfWork searches from nonce zero and returns a block whose hash satisfies
// its difficulty. It leaves the input unchanged and returns a zero block on error.
// Only nonce and hash change; transaction contents must not be mutated by callers
// during the search. This function checks difficulty, not full block validity.
func ProofOfWork(ctx context.Context, block blockchain.Block) (blockchain.Block, error) {
	if block.Difficulty < blockchain.MinDifficulty || block.Difficulty > blockchain.MaxDifficulty {
		return blockchain.Block{}, fmt.Errorf("difficulty must be between %d and %d", blockchain.MinDifficulty, blockchain.MaxDifficulty)
	}
	prefix := strings.Repeat("0", block.Difficulty)
	block.Nonce = 0
	for {
		if err := ctx.Err(); err != nil {
			return blockchain.Block{}, err
		}
		hash := block.CalculateHash()
		if strings.HasPrefix(hash, prefix) {
			// Cancellation may have arrived while calculating the winning hash.
			if err := ctx.Err(); err != nil {
				return blockchain.Block{}, err
			}
			block.Hash = hash
			return block, nil
		}
		if block.Nonce == math.MaxUint64 {
			return blockchain.Block{}, fmt.Errorf("proof of work exhausted nonce range")
		}
		block.Nonce++
	}
}
