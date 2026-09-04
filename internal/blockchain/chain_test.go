package blockchain

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewBlockchainContainsGenesis(t *testing.T) {
	genesis := mustGenesis(t, 3)

	chain := NewBlockchain(genesis)

	if len(chain.Blocks) != 1 {
		t.Fatalf("len(Blocks) = %d, want 1", len(chain.Blocks))
	}
	if !reflect.DeepEqual(chain.Blocks[0], genesis) {
		t.Errorf("Blocks[0] = %#v, want Genesis %#v", chain.Blocks[0], genesis)
	}
}

func TestBlockchainTipAndNextHeight(t *testing.T) {
	genesis := mustGenesis(t, 3)
	chain := NewBlockchain(genesis)
	latest := Block{Height: 1, PreviousHash: genesis.Hash, Difficulty: 3}
	chain.Blocks = append(chain.Blocks, latest)

	tip, err := chain.Tip()
	if err != nil {
		t.Fatalf("Tip() error = %v", err)
	}
	if tip.Height != latest.Height {
		t.Errorf("Tip().Height = %d, want %d", tip.Height, latest.Height)
	}
	if tip.PreviousHash != genesis.Hash {
		t.Errorf("Tip().PreviousHash = %q, want %q", tip.PreviousHash, genesis.Hash)
	}

	nextHeight, err := chain.NextHeight()
	if err != nil {
		t.Fatalf("NextHeight() error = %v", err)
	}
	if nextHeight != 2 {
		t.Errorf("NextHeight() = %d, want 2", nextHeight)
	}
}

func TestBlockchainDifficultyComesFromGenesis(t *testing.T) {
	genesis := mustGenesis(t, 3)
	chain := NewBlockchain(genesis)
	chain.Blocks = append(chain.Blocks, Block{Height: 1, Difficulty: 7})

	difficulty, err := chain.Difficulty()
	if err != nil {
		t.Fatalf("Difficulty() error = %v", err)
	}
	if difficulty != genesis.Difficulty {
		t.Errorf("Difficulty() = %d, want Genesis difficulty %d", difficulty, genesis.Difficulty)
	}
}

func TestEmptyBlockchainOperationsReturnError(t *testing.T) {
	chain := Blockchain{}

	if _, err := chain.Tip(); !errors.Is(err, ErrEmptyBlockchain) {
		t.Errorf("Tip() error = %v, want %v", err, ErrEmptyBlockchain)
	}
	if _, err := chain.NextHeight(); !errors.Is(err, ErrEmptyBlockchain) {
		t.Errorf("NextHeight() error = %v, want %v", err, ErrEmptyBlockchain)
	}
	if _, err := chain.Difficulty(); !errors.Is(err, ErrEmptyBlockchain) {
		t.Errorf("Difficulty() error = %v, want %v", err, ErrEmptyBlockchain)
	}
}

func mustGenesis(t *testing.T, difficulty int) Block {
	t.Helper()

	genesis, err := NewGenesisBlock(1_700_000_000, difficulty)
	if err != nil {
		t.Fatalf("NewGenesisBlock() error = %v", err)
	}

	return genesis
}
