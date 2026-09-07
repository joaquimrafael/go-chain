package blockchain

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/joaquimrafael/go-chain/internal/transaction"
)

func TestBalancesGenesisAndEmptyChain(t *testing.T) {
	chain := NewBlockchain(mustGenesis(t, 1))
	balances, err := chain.Balances()
	if err != nil || balances == nil || len(balances) != 0 {
		t.Fatalf("Genesis Balances() = %v, %v, want non-nil empty map", balances, err)
	}
	if balance, err := chain.Balance("alice"); err != nil || balance != 0 {
		t.Errorf("Genesis Balance(alice) = %d, %v, want 0", balance, err)
	}
	empty := Blockchain{}
	if balances, err := empty.Balances(); !errors.Is(err, ErrEmptyBlockchain) || balances != nil {
		t.Errorf("empty Balances() = %v, %v, want ErrEmptyBlockchain", balances, err)
	}
	if balance, err := empty.Balance("alice"); !errors.Is(err, ErrEmptyBlockchain) || balance != 0 {
		t.Errorf("empty Balance(alice) = %d, %v, want ErrEmptyBlockchain", balance, err)
	}
}

func TestBalancesReplayConfirmedHistory(t *testing.T) {
	chain := balanceTestChain(t,
		[]transaction.Transaction{{Type: transaction.Reward, To: "alice", Amount: 50}},
		[]transaction.Transaction{
			{Type: transaction.Reward, To: "alice", Amount: 50},
			{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 10},
		},
	)
	balances, err := chain.Balances()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"alice": 90, "bob": 10}
	if !reflect.DeepEqual(balances, want) {
		t.Fatalf("Balances() = %v, want %v", balances, want)
	}
	for _, tt := range []struct {
		name string
		want int64
	}{
		{name: "alice", want: 90},
		{name: "bob", want: 10},
		{name: "charlie", want: 0},
		{name: "Alice", want: 0},
	} {
		if got, err := chain.Balance(tt.name); err != nil || got != tt.want {
			t.Errorf("Balance(%q) = %d, %v, want %d", tt.name, got, err, tt.want)
		}
	}
	// Changing the derived map must not change the authoritative history.
	balances["alice"] = 999
	if got, err := chain.Balances(); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("repeated Balances() = %v, %v, want %v", got, err, want)
	}
}

func TestBalancesRespectTransactionOrder(t *testing.T) {
	chain := balanceTestChain(t,
		[]transaction.Transaction{{Type: transaction.Reward, To: "alice", Amount: 50}},
		[]transaction.Transaction{
			{Type: transaction.Reward, To: "miner", Amount: 50},
			{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 50},
			{Type: transaction.Transfer, From: "bob", To: "charlie", Amount: 50},
		},
	)
	balances, err := chain.Balances()
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int64{"alice": 0, "bob": 0, "charlie": 50, "miner": 50} {
		if balances[name] != want {
			t.Errorf("balance for %q = %d, want %d", name, balances[name], want)
		}
	}
	// Bob cannot spend funds that arrive later, even if the final totals match.
	transfers := chain.Blocks[2].Transactions
	transfers[1], transfers[2] = transfers[2], transfers[1]
	if got, err := chain.Balances(); err == nil || !strings.Contains(err.Error(), "block 2: transaction 1:") || got != nil {
		t.Fatalf("out-of-order Balances() = %v, %v, want first unfunded transfer error", got, err)
	}
}

func TestBalancesRejectInvalidReplay(t *testing.T) {
	for _, tt := range []struct {
		name     string
		transfer transaction.Transaction
		want     string
	}{
		{name: "unknown type", transfer: transaction.Transaction{Type: "unknown", To: "bob", Amount: 10}, want: "unknown transaction type"},
		{name: "zero amount", transfer: transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob"}, want: "amount must be positive"},
		{name: "negative transfer", transfer: transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: -10}, want: "amount must be positive"},
		{name: "negative reward", transfer: transaction.Transaction{Type: transaction.Reward, To: "bob", Amount: -50}, want: "amount must be positive"},
		{name: "overspend", transfer: transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 51}, want: `account "alice" has 50 GOC, cannot spend 51 GOC`},
		{name: "unfunded sender", transfer: transaction.Transaction{Type: transaction.Transfer, From: "bob", To: "alice", Amount: 1}, want: `account "bob" has 0 GOC, cannot spend 1 GOC`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			chain := balanceTestChain(t,
				[]transaction.Transaction{{Type: transaction.Reward, To: "alice", Amount: 50}},
				[]transaction.Transaction{{Type: transaction.Reward, To: "miner", Amount: 50}, tt.transfer},
				[]transaction.Transaction{{Type: "later invalid type"}},
			)
			want := "block 2: transaction 1: " + tt.want
			if got, err := chain.Balances(); err == nil || !strings.Contains(err.Error(), want) || got != nil {
				t.Errorf("Balances() = %v, %v, want no partial map and error containing %q", got, err, want)
			}
			// Even a lookup for an uninvolved account must propagate replay failure.
			if got, err := chain.Balance("charlie"); err == nil || !strings.Contains(err.Error(), want) || got != 0 {
				t.Errorf("Balance(charlie) = %d, %v, want error containing %q", got, err, want)
			}
		})
	}
}

func TestBalancesIntegerLimit(t *testing.T) {
	// Oversized rewards exercise arithmetic only; reward policy is validated later.
	for _, kind := range []transaction.Type{transaction.Reward, transaction.Transfer} {
		t.Run(string(kind), func(t *testing.T) {
			chain := balanceTestChain(t, []transaction.Transaction{
				{Type: transaction.Reward, To: "alice", Amount: math.MaxInt64 - 1},
				{Type: transaction.Reward, To: "bob", Amount: 2},
				{Type: kind, From: "bob", To: "alice", Amount: 1},
			})
			if got, err := chain.Balance("alice"); err != nil || got != math.MaxInt64 {
				t.Fatalf("Balance(alice) = %d, %v, want MaxInt64", got, err)
			}
			chain.Blocks[1].Transactions = append(chain.Blocks[1].Transactions,
				transaction.Transaction{Type: kind, From: "bob", To: "alice", Amount: 1})
			if got, err := chain.Balances(); err == nil || !strings.Contains(err.Error(), "block 1: transaction 3: balance for account") || got != nil {
				t.Fatalf("overflow Balances() = %v, %v, want range error and no partial map", got, err)
			}
		})
	}
}

// balanceTestChain supplies ordered history without mining. These tests exercise
// replay independently of structural validation and complete reward rules.
func balanceTestChain(t *testing.T, blocks ...[]transaction.Transaction) Blockchain {
	t.Helper()
	chain := NewBlockchain(mustGenesis(t, 1))
	for _, transactions := range blocks {
		chain.Blocks = append(chain.Blocks, Block{Height: int64(len(chain.Blocks)), Transactions: transactions})
	}
	return chain
}
