package blockchain

import (
	"errors"
	"strings"
	"testing"

	"github.com/joaquimrafael/go-chain/internal/transaction"
)

func TestValidateStructureEmptyChain(t *testing.T) {
	if err := ValidateStructure(Blockchain{}); !errors.Is(err, ErrEmptyBlockchain) {
		t.Errorf("ValidateStructure() error = %v, want %v", err, ErrEmptyBlockchain)
	}
}

func TestValidateStructureGenesisWithoutProofOfWork(t *testing.T) {
	genesis := mustGenesis(t, 3)
	if strings.HasPrefix(genesis.Hash, "000") {
		t.Fatal("fixture unexpectedly satisfies Proof of Work")
	}
	if err := ValidateStructure(NewBlockchain(genesis)); err != nil {
		t.Fatalf("ValidateStructure() error = %v", err)
	}
}

func TestValidateStructureValidChain(t *testing.T) {
	if err := ValidateStructure(structuralTestChain(t)); err != nil {
		t.Fatalf("ValidateStructure() error = %v", err)
	}
}

func TestValidateStructureCorruption(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Blockchain)
		want   string
	}{
		{
			name:   "Genesis height",
			mutate: func(chain *Blockchain) { chain.Blocks[0].Height = 1 },
			want:   "block 0: height",
		},
		{
			name:   "Genesis previous hash",
			mutate: func(chain *Blockchain) { chain.Blocks[0].PreviousHash = "" },
			want:   "block 0: Genesis previous hash",
		},
		{
			name: "Genesis transactions",
			mutate: func(chain *Blockchain) {
				chain.Blocks[0].Transactions = []transaction.Transaction{{Type: transaction.Reward, To: "alice", Amount: 50}}
			},
			want: "block 0: Genesis must contain no transactions",
		},
		{
			name:   "Genesis hash",
			mutate: func(chain *Blockchain) { chain.Blocks[0].Hash = "bad hash" },
			want:   "block 0: stored hash",
		},
		{
			name:   "Genesis difficulty below minimum",
			mutate: func(chain *Blockchain) { chain.Blocks[0].Difficulty = 0 },
			want:   "block 0: difficulty must be between",
		},
		{
			name:   "Genesis difficulty above maximum",
			mutate: func(chain *Blockchain) { chain.Blocks[0].Difficulty = 65 },
			want:   "block 0: difficulty must be between",
		},
		{
			name:   "nonsequential height",
			mutate: func(chain *Blockchain) { chain.Blocks[1].Height = 9 },
			want:   "block 1: height is 9, want 1",
		},
		{
			name:   "broken link",
			mutate: func(chain *Blockchain) { chain.Blocks[1].PreviousHash = GenesisPreviousHash },
			want:   "block 1: previous hash does not match block 0 hash",
		},
		{
			name:   "stored hash",
			mutate: func(chain *Blockchain) { chain.Blocks[1].Hash = GenesisPreviousHash },
			want:   "block 1: stored hash",
		},
		{
			name:   "nonce",
			mutate: func(chain *Blockchain) { chain.Blocks[1].Nonce++ },
			want:   "block 1: stored hash",
		},
		{
			name:   "transaction contents",
			mutate: func(chain *Blockchain) { chain.Blocks[1].Transactions[0].Amount++ },
			want:   "block 1: stored hash",
		},
		{
			name:   "changed difficulty",
			mutate: func(chain *Blockchain) { chain.Blocks[1].Difficulty = 2 },
			want:   "block 1: difficulty is 2, want Genesis difficulty 1",
		},
		{
			name:   "negative difficulty",
			mutate: func(chain *Blockchain) { chain.Blocks[1].Difficulty = -1 },
			want:   "block 1: difficulty must be between",
		},
		{
			name:   "excessive difficulty",
			mutate: func(chain *Blockchain) { chain.Blocks[1].Difficulty = 65 },
			want:   "block 1: difficulty must be between",
		},
		{
			name: "recalculated hash without Proof of Work",
			mutate: func(chain *Blockchain) {
				block := &chain.Blocks[1]
				for strings.HasPrefix(block.Hash, "0") {
					block.Nonce++
					block.Hash = block.CalculateHash()
				}
			},
			want: "block 1: hash does not satisfy difficulty 1",
		},
		{
			name: "first invalid block wins",
			mutate: func(chain *Blockchain) {
				chain.Blocks[1].Nonce++
				chain.Blocks[2].Height = 99
			},
			want: "block 1: stored hash",
		},
		{
			name:   "last block checked",
			mutate: func(chain *Blockchain) { chain.Blocks[2].Nonce++ },
			want:   "block 2: stored hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := structuralTestChain(t)
			tt.mutate(&chain)
			err := ValidateStructure(chain)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateStructure() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

// structuralTestChain builds a deterministic fixture at difficulty 1. The bounded
// nonce search is test setup; cancellable production mining comes in Milestone 12.
func structuralTestChain(t *testing.T) Blockchain {
	t.Helper()
	chain := NewBlockchain(mustGenesis(t, 1))
	for height := int64(1); height <= 2; height++ {
		block := Block{
			Height:       height,
			Timestamp:    1_700_000_000 + height,
			PreviousHash: chain.Blocks[len(chain.Blocks)-1].Hash,
			Difficulty:   1,
			Transactions: []transaction.Transaction{{Type: transaction.Reward, To: "alice", Amount: 50}},
		}
		for block.Nonce = 0; block.Nonce < 10_000; block.Nonce++ {
			block.Hash = block.CalculateHash()
			if strings.HasPrefix(block.Hash, "0") {
				break
			}
		}
		if !strings.HasPrefix(block.Hash, "0") {
			t.Fatal("could not find a valid nonce for the test fixture")
		}
		chain.Blocks = append(chain.Blocks, block)
	}
	return chain
}
