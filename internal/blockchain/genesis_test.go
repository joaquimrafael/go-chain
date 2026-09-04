package blockchain

import (
	"reflect"
	"strings"
	"testing"
)

func TestNewGenesisBlock(t *testing.T) {
	const (
		timestamp  = int64(1_700_000_000)
		difficulty = 3
	)

	block, err := NewGenesisBlock(timestamp, difficulty)
	if err != nil {
		t.Fatalf("NewGenesisBlock() error = %v", err)
	}

	if block.Height != GenesisHeight {
		t.Errorf("Height = %d, want %d", block.Height, GenesisHeight)
	}
	if block.Timestamp != timestamp {
		t.Errorf("Timestamp = %d, want %d", block.Timestamp, timestamp)
	}
	if block.PreviousHash != GenesisPreviousHash {
		t.Errorf("PreviousHash = %q, want %q", block.PreviousHash, GenesisPreviousHash)
	}
	if len(block.PreviousHash) != 64 || strings.Trim(block.PreviousHash, "0") != "" {
		t.Errorf("PreviousHash = %q, want 64 zero characters", block.PreviousHash)
	}
	if block.Nonce != GenesisNonce {
		t.Errorf("Nonce = %d, want %d", block.Nonce, GenesisNonce)
	}
	if block.Difficulty != difficulty {
		t.Errorf("Difficulty = %d, want %d", block.Difficulty, difficulty)
	}
	if block.Transactions == nil {
		t.Error("Transactions is nil, want an empty slice")
	}
	if len(block.Transactions) != 0 {
		t.Errorf("len(Transactions) = %d, want 0", len(block.Transactions))
	}
	if block.Hash != block.CalculateHash() {
		t.Errorf("Hash = %q, want calculated hash %q", block.Hash, block.CalculateHash())
	}
	const wantHash = "221e70ef2ec4cc0bf9050527c41194eb03340c467a6529a303bee33ee19f3e1a"
	if block.Hash != wantHash {
		t.Errorf("Hash = %q, want deterministic hash %q", block.Hash, wantHash)
	}
}

func TestNewGenesisBlockIsDeterministic(t *testing.T) {
	first, err := NewGenesisBlock(1_700_000_000, 3)
	if err != nil {
		t.Fatalf("first NewGenesisBlock() error = %v", err)
	}
	second, err := NewGenesisBlock(1_700_000_000, 3)
	if err != nil {
		t.Fatalf("second NewGenesisBlock() error = %v", err)
	}

	if first.Hash != second.Hash {
		t.Errorf("same Genesis inputs produced hashes %q and %q", first.Hash, second.Hash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("same Genesis inputs produced different blocks:\nfirst:  %#v\nsecond: %#v", first, second)
	}
}

func TestNewGenesisBlockDifficultyBounds(t *testing.T) {
	tests := []struct {
		name       string
		difficulty int
		wantError  bool
	}{
		{name: "minimum", difficulty: MinDifficulty},
		{name: "maximum", difficulty: MaxDifficulty},
		{name: "below minimum", difficulty: MinDifficulty - 1, wantError: true},
		{name: "above maximum", difficulty: MaxDifficulty + 1, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := NewGenesisBlock(1_700_000_000, tt.difficulty)

			if tt.wantError {
				if err == nil {
					t.Fatalf("NewGenesisBlock() error = nil, want an error")
				}
				if !reflect.DeepEqual(block, Block{}) {
					t.Errorf("NewGenesisBlock() block = %#v, want zero value", block)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewGenesisBlock() error = %v", err)
			}
		})
	}
}
