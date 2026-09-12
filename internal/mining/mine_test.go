package mining

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/joaquimrafael/go-chain/internal/blockchain"
	"github.com/joaquimrafael/go-chain/internal/transaction"
)

func TestMineBlockRewardOnly(t *testing.T) {
	chain, accounts := miningChain(t, 2)
	got, err := MineBlock(t.Context(), chain, nil, "alice", accounts, 101)
	if err != nil {
		t.Fatal(err)
	}
	if got.Height != 1 || got.Timestamp != 101 || got.PreviousHash != chain.Blocks[0].Hash || got.Difficulty != 2 {
		t.Errorf("unexpected block header: %+v", got)
	}
	want := []transaction.Transaction{{Type: transaction.Reward, To: "alice", Amount: 50}}
	if !reflect.DeepEqual(got.Transactions, want) {
		t.Errorf("transactions = %+v, want %+v", got.Transactions, want)
	}
	chain.Blocks = append(chain.Blocks, got)
	if err := blockchain.Validate(chain, accounts); err != nil {
		t.Fatalf("mined chain is invalid: %v", err)
	}
	again, err := MineBlock(t.Context(), blockchain.NewBlockchain(chain.Blocks[0]), nil, "alice", accounts, 101)
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Errorf("same snapshot produced different result: %+v, %v", again, err)
	}
}

func TestMineBlockOrderedTransfersAndInputOwnership(t *testing.T) {
	chain, accounts := fundedMiningChain(t)
	pending := []transaction.Transaction{
		{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 20},
		{Type: transaction.Transfer, From: "alice", To: "carol", Amount: 30},
	}
	wantPending := slices.Clone(pending)
	wantAccounts := map[string]struct{}{"alice": {}, "bob": {}, "carol": {}}
	// A sentinel beyond the slice length detects an append into caller storage.
	backing := append(slices.Clone(chain.Blocks), blockchain.Block{Hash: "sentinel"})
	chain.Blocks = backing[:2]
	wantBacking := slices.Clone(backing)
	for i := range wantBacking {
		wantBacking[i].Transactions = slices.Clone(backing[i].Transactions)
	}
	got, err := MineBlock(t.Context(), chain, pending, "bob", accounts, 102)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]transaction.Transaction{{Type: transaction.Reward, To: "bob", Amount: 50}}, wantPending...)
	if !reflect.DeepEqual(got.Transactions, want) {
		t.Errorf("transactions = %+v, want %+v", got.Transactions, want)
	}
	if got.Height != 2 || got.PreviousHash != chain.Blocks[1].Hash || got.Difficulty != 1 {
		t.Errorf("unexpected block header: %+v", got)
	}
	result := blockchain.Blockchain{Blocks: append(slices.Clone(chain.Blocks), got)}
	if err := blockchain.Validate(result, accounts); err != nil {
		t.Fatal(err)
	}
	balances, err := result.Balances()
	if err != nil || !reflect.DeepEqual(balances, map[string]int64{"alice": 0, "bob": 70, "carol": 30}) {
		t.Errorf("balances = %v, %v", balances, err)
	}
	if !reflect.DeepEqual(backing, wantBacking) || !reflect.DeepEqual(pending, wantPending) || !reflect.DeepEqual(accounts, wantAccounts) {
		t.Fatal("mining changed the input snapshot")
	}
	got.Transactions[1].Amount = 999
	if !reflect.DeepEqual(pending, wantPending) || !reflect.DeepEqual(backing, wantBacking) {
		t.Error("returned transactions alias input storage")
	}
}

func TestMineBlockRejectsInvalidSnapshot(t *testing.T) {
	tests := []struct {
		name    string
		miner   string
		pending []transaction.Transaction
		corrupt bool
		want    string
	}{
		{name: "unknown miner", miner: "Alice", want: `miner account "Alice" does not exist`},
		{name: "corrupt history", miner: "alice", corrupt: true, want: "validate existing chain: block 1"},
		{name: "missing sender", pending: []transaction.Transaction{{Type: transaction.Transfer, From: "missing", To: "bob", Amount: 1}}, want: "sender account"},
		{name: "missing receiver", pending: []transaction.Transaction{{Type: transaction.Transfer, From: "alice", To: "missing", Amount: 1}}, want: "receiver account"},
		{name: "same account", pending: []transaction.Transaction{{Type: transaction.Transfer, From: "alice", To: "alice", Amount: 1}}, want: "different accounts"},
		{name: "zero amount", pending: []transaction.Transaction{{Type: transaction.Transfer, From: "alice", To: "bob"}}, want: "amount must be positive"},
		{name: "negative amount", pending: []transaction.Transaction{{Type: transaction.Transfer, From: "alice", To: "bob", Amount: -1}}, want: "amount must be positive"},
		{name: "pending reward", pending: []transaction.Transaction{{Type: transaction.Reward, To: "alice", Amount: 50}}, want: "type must be transfer"},
		{name: "unknown type", pending: []transaction.Transaction{{Type: "unknown", To: "alice", Amount: 1}}, want: "type must be transfer"},
		{name: "new reward cannot fund transfer", miner: "alice", pending: []transaction.Transaction{{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 51}}, want: "50 GOC available"},
		{name: "cumulative overspend", pending: []transaction.Transaction{
			{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 30},
			{Type: transaction.Transfer, From: "alice", To: "carol", Amount: 21},
		}, want: "pending transaction 1"},
		{name: "incoming pending cannot fund transfer", pending: []transaction.Transaction{
			{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 20},
			{Type: transaction.Transfer, From: "bob", To: "carol", Amount: 1},
		}, want: `pending transaction 1: account "bob" has 0 GOC available`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain, accounts := fundedMiningChain(t)
			if tt.corrupt {
				chain.Blocks[1].Hash = "tampered"
			}
			miner := tt.miner
			if miner == "" {
				miner = "carol"
			}
			got, err := MineBlock(t.Context(), chain, tt.pending, miner, accounts, 102)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if !reflect.DeepEqual(got, blockchain.Block{}) {
				t.Errorf("failure returned a block: %+v", got)
			}
		})
	}
	got, err := MineBlock(t.Context(), blockchain.Blockchain{}, nil, "alice", nil, 102)
	if !errors.Is(err, blockchain.ErrEmptyBlockchain) || !reflect.DeepEqual(got, blockchain.Block{}) {
		t.Errorf("empty chain = %+v, %v", got, err)
	}
}

func TestMineBlockCancellation(t *testing.T) {
	for _, alreadyCancelled := range []bool{true, false} {
		name := "during search"
		if alreadyCancelled {
			name = "before mining"
		}
		t.Run(name, func(t *testing.T) {
			chain, accounts := miningChain(t, blockchain.MaxDifficulty)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
			defer cancel()
			wantErr := context.DeadlineExceeded
			if alreadyCancelled {
				cancel()
				wantErr = context.Canceled
			}
			got, err := MineBlock(ctx, chain, nil, "alice", accounts, 101)
			if !errors.Is(err, wantErr) || !reflect.DeepEqual(got, blockchain.Block{}) {
				t.Errorf("cancelled mining = %+v, %v; want zero block and %v", got, err, wantErr)
			}
			if len(chain.Blocks) != 1 || len(chain.Blocks[0].Transactions) != 0 {
				t.Error("cancelled mining changed history")
			}
		})
	}
}

func miningChain(t *testing.T, difficulty int) (blockchain.Blockchain, map[string]struct{}) {
	t.Helper()
	genesis, err := blockchain.NewGenesisBlock(100, difficulty)
	if err != nil {
		t.Fatal(err)
	}
	return blockchain.NewBlockchain(genesis), map[string]struct{}{"alice": {}, "bob": {}, "carol": {}}
}

func fundedMiningChain(t *testing.T) (blockchain.Blockchain, map[string]struct{}) {
	t.Helper()
	chain, accounts := miningChain(t, 1)
	block, err := ProofOfWork(t.Context(), blockchain.Block{
		Height: 1, Timestamp: 101, PreviousHash: chain.Blocks[0].Hash, Difficulty: 1,
		Transactions: []transaction.Transaction{{Type: transaction.Reward, To: "alice", Amount: 50}},
	})
	if err != nil {
		t.Fatal(err)
	}
	chain.Blocks = append(chain.Blocks, block)
	return chain, accounts
}
