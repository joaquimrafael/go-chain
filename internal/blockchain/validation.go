package blockchain

import (
	"fmt"
	"strings"
)

// ValidateStructure checks hashes, links, heights, and fixed difficulty in chain
// order. It does not validate transaction, account, reward, or balance rules,
// except that Genesis must contain no transactions.
func ValidateStructure(chain Blockchain) error {
	if len(chain.Blocks) == 0 {
		return ErrEmptyBlockchain
	}

	difficulty := chain.Blocks[0].Difficulty
	for i, block := range chain.Blocks {
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
	}

	return nil
}
