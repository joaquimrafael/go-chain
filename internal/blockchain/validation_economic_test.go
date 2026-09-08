package blockchain

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/joaquimrafael/go-chain/internal/transaction"
)

func TestValidateEmptyAndGenesis(t *testing.T) {
	if err := Validate(Blockchain{}, nil); !errors.Is(err, ErrEmptyBlockchain) {
		t.Errorf("Validate(empty) = %v, want ErrEmptyBlockchain", err)
	}
	if err := Validate(NewBlockchain(mustGenesis(t, 3)), nil); err != nil {
		t.Errorf("Validate(Genesis) = %v; no accounts or Proof of Work required", err)
	}
}

func TestValidateRewardOnlyChain(t *testing.T) {
	chain := structuralTestChain(t)
	if err := Validate(chain, map[string]struct{}{"alice": {}}); err != nil {
		t.Fatalf("Validate(reward-only chain) = %v", err)
	}
	if err := Validate(chain, nil); err == nil || !strings.Contains(err.Error(), "block 1: transaction 0: receiver account") {
		t.Errorf("Validate(without accounts) = %v, want missing miner error", err)
	}
}

func TestValidateOrderedReplay(t *testing.T) {
	chain := structuralTestChain(t)
	chain.Blocks[1].Transactions = append(chain.Blocks[1].Transactions,
		transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 50},
		transaction.Transaction{Type: transaction.Transfer, From: "bob", To: "carol", Amount: 20})
	chain.Blocks[2].Transactions = append(chain.Blocks[2].Transactions,
		transaction.Transaction{Type: transaction.Transfer, From: "carol", To: "alice", Amount: 20})
	remineValidationChain(t, &chain)
	accounts := map[string]struct{}{"alice": {}, "bob": {}, "carol": {}}
	before := make([]Block, len(chain.Blocks))
	for i, block := range chain.Blocks {
		before[i] = block
		before[i].Transactions = append([]transaction.Transaction{}, block.Transactions...)
	}
	if err := Validate(chain, accounts); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if !reflect.DeepEqual(chain.Blocks, before) || !reflect.DeepEqual(accounts, map[string]struct{}{"alice": {}, "bob": {}, "carol": {}}) {
		t.Error("Validate changed its inputs")
	}
	balances, err := chain.Balances()
	if err != nil || !reflect.DeepEqual(balances, map[string]int64{"alice": 70, "bob": 30, "carol": 0}) {
		t.Errorf("Balances() = %v, %v; want alice=70, bob=30, carol=0", balances, err)
	}
	// Moving Bob's spending ahead of his incoming confirmed transfer is invalid.
	chain.Blocks[1].Transactions[1], chain.Blocks[1].Transactions[2] = chain.Blocks[1].Transactions[2], chain.Blocks[1].Transactions[1]
	remineValidationChain(t, &chain)
	if err := Validate(chain, accounts); err == nil || !strings.Contains(err.Error(), "block 1: transaction 1: account \"bob\" has 0 GOC") {
		t.Errorf("Validate(reordered spending) = %v, want unfunded Bob error", err)
	}
}

func TestValidateEconomicRules(t *testing.T) {
	reward := transaction.Transaction{Type: transaction.Reward, To: "alice", Amount: 50}
	transfer := transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 10}
	tests := []struct {
		name   string
		mutate func(*Blockchain)
		want   string
	}{
		{"empty block", func(c *Blockchain) { c.Blocks[1].Transactions = nil }, "block 1: first transaction must be a mining reward"},
		{"missing reward", func(c *Blockchain) { c.Blocks[1].Transactions = []transaction.Transaction{transfer} }, "block 1: first transaction must be a mining reward"},
		{"misplaced reward", func(c *Blockchain) { c.Blocks[1].Transactions = []transaction.Transaction{transfer, reward} }, "block 1: first transaction must be a mining reward"},
		{"duplicate reward", func(c *Blockchain) { c.Blocks[1].Transactions = []transaction.Transaction{reward, reward} }, "block 1: transaction 1: mining reward must occur exactly once"},
		{"reward sender", func(c *Blockchain) { c.Blocks[1].Transactions[0].From = "SYSTEM" }, "block 1: transaction 0: mining reward sender must be empty"},
		{"reward amount", func(c *Blockchain) { c.Blocks[1].Transactions[0].Amount = 51 }, "block 1: transaction 0: mining reward amount must be 50 GOC"},
		{"zero reward", func(c *Blockchain) { c.Blocks[1].Transactions[0].Amount = 0 }, "block 1: transaction 0: mining reward amount must be 50 GOC"},
		{"negative reward", func(c *Blockchain) { c.Blocks[1].Transactions[0].Amount = -50 }, "block 1: transaction 0: mining reward amount must be 50 GOC"},
		{"unknown miner", func(c *Blockchain) { c.Blocks[1].Transactions[0].To = "Alice" }, "block 1: transaction 0: receiver account \"Alice\" does not exist"},
		{"empty miner", func(c *Blockchain) { c.Blocks[1].Transactions[0].To = "" }, "block 1: transaction 0: receiver account \"\" does not exist"},
		{"unknown type", func(c *Blockchain) { c.Blocks[1].Transactions[1].Type = "other" }, "block 1: transaction 1: unknown transaction type"},
		{"unknown sender", func(c *Blockchain) { c.Blocks[1].Transactions[1].From = "carol" }, "block 1: transaction 1: sender account \"carol\" does not exist"},
		{"unknown receiver", func(c *Blockchain) { c.Blocks[1].Transactions[1].To = "carol" }, "block 1: transaction 1: receiver account \"carol\" does not exist"},
		{"same account", func(c *Blockchain) { c.Blocks[1].Transactions[1].To = "alice" }, "block 1: transaction 1: sender and receiver must be different"},
		{"zero transfer", func(c *Blockchain) { c.Blocks[1].Transactions[1].Amount = 0 }, "block 1: transaction 1: amount must be positive"},
		{"negative transfer", func(c *Blockchain) { c.Blocks[1].Transactions[1].Amount = -1 }, "block 1: transaction 1: amount must be positive"},
		{"future reward cannot fund earlier spend", func(c *Blockchain) { c.Blocks[1].Transactions[1].Amount = 51 }, "block 1: transaction 1: account \"alice\" has 50 GOC available"},
		{"cumulative spending", func(c *Blockchain) {
			c.Blocks[1].Transactions = append(c.Blocks[1].Transactions, transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 41})
		}, "block 1: transaction 2: account \"alice\" has 40 GOC available"},
		{"later block overspend", func(c *Blockchain) {
			c.Blocks[2].Transactions = append(c.Blocks[2].Transactions, transaction.Transaction{Type: transaction.Transfer, From: "bob", To: "alice", Amount: 11})
		}, "block 2: transaction 1: account \"bob\" has 10 GOC available"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := structuralTestChain(t)
			chain.Blocks[1].Transactions = append(chain.Blocks[1].Transactions, transfer)
			tt.mutate(&chain)
			remineValidationChain(t, &chain)
			if err := ValidateStructure(chain); err != nil {
				t.Fatalf("economic fixture has invalid structure: %v", err)
			}
			if err := Validate(chain, map[string]struct{}{"alice": {}, "bob": {}}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateFirstInvalidBlockAcrossRules(t *testing.T) {
	chain := structuralTestChain(t)
	chain.Blocks[1].Transactions[0].Amount = 49
	remineValidationChain(t, &chain)
	chain.Blocks[2].Hash = "corrupt"
	if err := Validate(chain, map[string]struct{}{"alice": {}}); err == nil || !strings.Contains(err.Error(), "block 1: transaction 0: mining reward amount") {
		t.Errorf("Validate() = %v, want earlier reward error before later structural error", err)
	}
	chain.Blocks[1].Hash = "corrupt"
	if err := Validate(chain, nil); err == nil || !strings.Contains(err.Error(), "block 1: stored hash") {
		t.Errorf("Validate() = %v, want structural error before economic rules in same block", err)
	}
}

func remineValidationChain(t *testing.T, chain *Blockchain) {
	t.Helper()
	for i := 1; i < len(chain.Blocks); i++ {
		block := &chain.Blocks[i]
		block.PreviousHash = chain.Blocks[i-1].Hash
		for block.Nonce = 0; block.Nonce < 10_000; block.Nonce++ {
			block.Hash = block.CalculateHash()
			if strings.HasPrefix(block.Hash, "0") {
				break
			}
		}
		if !strings.HasPrefix(block.Hash, "0") {
			t.Fatal("could not mine difficulty-1 validation fixture")
		}
	}
}
