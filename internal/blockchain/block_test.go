package blockchain

import (
	"testing"

	"github.com/joaquimrafael/go-chain/internal/transaction"
)

func TestCalculateHashIsDeterministicSHA256(t *testing.T) {
	block := testBlock()

	first := block.CalculateHash()
	second := block.CalculateHash()

	if first != second {
		t.Fatalf("CalculateHash() returned %q, then %q for the same block", first, second)
	}
	const want = "1e605d8701702d2a4aab4e2f43084a1a4c5f1675e525f2087abd24d0cf99beaa"
	if first != want {
		t.Errorf("CalculateHash() = %q, want SHA-256 hash %q", first, want)
	}
}

func TestCalculateHashIncludesEveryPayloadField(t *testing.T) {
	original := testBlock()
	originalHash := original.CalculateHash()

	tests := []struct {
		name   string
		change func(*Block)
	}{
		{name: "height", change: func(b *Block) { b.Height++ }},
		{name: "timestamp", change: func(b *Block) { b.Timestamp++ }},
		{name: "previous hash", change: func(b *Block) { b.PreviousHash = "different" }},
		{name: "nonce", change: func(b *Block) { b.Nonce++ }},
		{name: "difficulty", change: func(b *Block) { b.Difficulty++ }},
		{name: "transaction type", change: func(b *Block) { b.Transactions[0].Type = transaction.Reward }},
		{name: "transaction sender", change: func(b *Block) { b.Transactions[0].From = "carol" }},
		{name: "transaction receiver", change: func(b *Block) { b.Transactions[0].To = "dave" }},
		{name: "transaction amount", change: func(b *Block) { b.Transactions[0].Amount++ }},
		{name: "transaction order", change: func(b *Block) {
			b.Transactions[0], b.Transactions[1] = b.Transactions[1], b.Transactions[0]
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := testBlock()
			tt.change(&changed)

			if got := changed.CalculateHash(); got == originalHash {
				t.Errorf("CalculateHash() remained %q after changing %s", got, tt.name)
			}
		})
	}
}

func TestCalculateHashExcludesStoredHash(t *testing.T) {
	block := testBlock()
	originalHash := block.CalculateHash()

	block.Hash = "a stored value that must not enter the payload"

	if got := block.CalculateHash(); got != originalHash {
		t.Errorf("CalculateHash() = %q after changing stored Hash, want %q", got, originalHash)
	}
}

func testBlock() Block {
	return Block{
		Height:       1,
		Timestamp:    1_700_000_000,
		PreviousHash: "000abc",
		Hash:         "stored-hash-is-ignored",
		Nonce:        42,
		Difficulty:   3,
		Transactions: []transaction.Transaction{
			{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 10},
			{Type: transaction.Reward, From: "", To: "miner", Amount: 50},
		},
	}
}
