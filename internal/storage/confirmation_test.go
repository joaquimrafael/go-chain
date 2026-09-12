package storage

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/joaquimrafael/go-chain/internal/blockchain"
	"github.com/joaquimrafael/go-chain/internal/mining"
	"github.com/joaquimrafael/go-chain/internal/transaction"
)

func TestConfirmMinedBlockPersistsAndReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	store, chain, block, pending := confirmationFixture(t, path)
	// This transfer arrived after mining took its snapshot. Even identical
	// contents must remain pending when the new ID was not included.
	later, err := store.AddPending(t.Context(), pending[0].Transaction, 105)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmMinedBlock(t.Context(), block, []int64{pending[0].ID, pending[1].ID}); err != nil {
		t.Fatal(err)
	}
	chain.Blocks = append(chain.Blocks, block)
	assertChain(t, store, chain)
	assertPending(t, store, []PendingTransaction{later})
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openTestStore(t, path)
	assertChain(t, store, chain)
	assertPending(t, store, []PendingTransaction{later})
	loaded, err := store.LoadChain(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := store.LoadAccountNames(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := blockchain.Validate(loaded, accounts); err != nil {
		t.Fatalf("reopened chain is invalid: %v", err)
	}
	balances, err := loaded.Balances()
	if err != nil || !reflect.DeepEqual(balances, map[string]int64{"alice": 35, "bob": 65}) {
		t.Errorf("balances = %v, %v; want alice=35, bob=65", balances, err)
	}
}

func TestConfirmMinedBlockRollsBack(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*blockchain.Block, []int64)
		trigger string
		want    string
	}{
		{name: "missing second ID", change: func(_ *blockchain.Block, ids []int64) { ids[1] = 999 }, want: "pending transaction 999 is missing"},
		{name: "duplicate ID", change: func(_ *blockchain.Block, ids []int64) { ids[1] = ids[0] }, want: "is missing or does not match"},
		{name: "wrong transfer for ID", change: func(_ *blockchain.Block, ids []int64) { ids[0], ids[1] = ids[1], ids[0] }, want: "does not match"},
		{name: "wrong tip hash", change: func(block *blockchain.Block, _ []int64) { block.PreviousHash = "stale" }, want: "stale mining candidate"},
		{name: "wrong height", change: func(block *blockchain.Block, _ []int64) { block.Height++ }, want: "stale mining candidate"},
		{name: "block insertion failure", trigger: `CREATE TRIGGER fail_block BEFORE INSERT ON blocks
			BEGIN SELECT RAISE(ABORT, 'forced block failure'); END`, want: "store mined block"},
		{name: "transaction insertion failure", trigger: `CREATE TRIGGER fail_transfer BEFORE INSERT ON block_transactions
			WHEN NEW.position = 2 BEGIN SELECT RAISE(ABORT, 'forced transaction failure'); END`, want: "store block 2 transaction 2"},
		{name: "second deletion failure", trigger: `CREATE TRIGGER fail_delete BEFORE DELETE ON pending_transactions
			WHEN OLD.amount = 5 BEGIN SELECT RAISE(ABORT, 'forced deletion failure'); END`, want: "delete pending transaction"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "chain.db")
			store, chain, block, pending := confirmationFixture(t, path)
			ids := []int64{pending[0].ID, pending[1].ID}
			if tt.change != nil {
				tt.change(&block, ids)
			}
			if tt.trigger != "" {
				if _, err := store.db.ExecContext(t.Context(), tt.trigger); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.ConfirmMinedBlock(t.Context(), block, ids); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("confirmation error = %v, want %q", err, tt.want)
			}
			assertChain(t, store, chain)
			assertPending(t, store, pending)
			// Also check rows directly: LoadChain would not expose orphan rows.
			var count int
			if err := store.db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM block_transactions").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Errorf("confirmed transaction count = %d, want only the original reward", count)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			store = openTestStore(t, path)
			assertChain(t, store, chain)
			assertPending(t, store, pending)
		})
	}
}

func TestConfirmMinedBlockRejectsStaleSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.db")
	store, chain, block, pending := confirmationFixture(t, path)
	accounts, err := store.LoadAccountNames(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	other, err := mining.MineBlock(t.Context(), chain, nil, "alice", accounts, 106)
	if err != nil {
		t.Fatal(err)
	}
	// A separate connection advances the tip after the original snapshot.
	otherStore := openTestStore(t, path)
	if err := otherStore.ConfirmMinedBlock(t.Context(), other, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmMinedBlock(t.Context(), block, []int64{pending[0].ID, pending[1].ID}); err == nil || !strings.Contains(err.Error(), "stale mining candidate") {
		t.Fatalf("stale confirmation error = %v", err)
	}
	chain.Blocks = append(chain.Blocks, other)
	assertChain(t, store, chain)
	assertPending(t, store, pending)
}

func TestConfirmMinedBlockCancellationAndArguments(t *testing.T) {
	store, chain, block, pending := confirmationFixture(t, filepath.Join(t.TempDir(), "chain.db"))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := store.ConfirmMinedBlock(ctx, block, []int64{pending[0].ID, pending[1].ID}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled confirmation error = %v", err)
	}
	if err := store.ConfirmMinedBlock(t.Context(), block, nil); err == nil || !strings.Contains(err.Error(), "count") {
		t.Fatalf("missing IDs error = %v", err)
	}
	if err := store.ConfirmMinedBlock(t.Context(), blockchain.Block{}, nil); err == nil || !strings.Contains(err.Error(), "reward") {
		t.Fatalf("empty block error = %v", err)
	}
	block.Transactions[1].Type = transaction.Reward
	if err := store.ConfirmMinedBlock(t.Context(), block, []int64{pending[0].ID, pending[1].ID}); err == nil || !strings.Contains(err.Error(), "transfers after") {
		t.Fatalf("extra reward error = %v", err)
	}
	assertChain(t, store, chain)
	assertPending(t, store, pending)
}

func TestConfirmMinedBlockRequiresInitialization(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "chain.db"))
	block := blockchain.Block{Transactions: []transaction.Transaction{{Type: transaction.Reward, To: "alice", Amount: 50}}}
	for _, schemaOnly := range []bool{false, true} {
		if schemaOnly {
			if _, err := store.db.ExecContext(t.Context(), schema); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.ConfirmMinedBlock(t.Context(), block, nil); !errors.Is(err, ErrNotInitialized) {
			t.Fatalf("uninitialized confirmation error = %v", err)
		}
	}
}

func confirmationFixture(t *testing.T, path string) (*Store, blockchain.Blockchain, blockchain.Block, []PendingTransaction) {
	t.Helper()
	store := openTestStore(t, path)
	ctx := t.Context()
	if err := store.Initialize(ctx, 100, 1); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alice", "bob"} {
		if _, err := store.CreateAccount(ctx, name, 100); err != nil {
			t.Fatal(err)
		}
	}
	accounts, err := store.LoadAccountNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := store.LoadChain(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reward, err := mining.MineBlock(ctx, chain, nil, "alice", accounts, 101)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmMinedBlock(ctx, reward, nil); err != nil {
		t.Fatal(err)
	}
	chain.Blocks = append(chain.Blocks, reward)
	assertChain(t, store, chain)
	transfers := []transaction.Transaction{
		{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 10},
		{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 5},
	}
	var pending []PendingTransaction
	for i, transfer := range transfers {
		entry, err := store.AddPending(ctx, transfer, int64(102+i))
		if err != nil {
			t.Fatal(err)
		}
		pending = append(pending, entry)
	}
	block, err := mining.MineBlock(ctx, chain, transfers, "bob", accounts, 104)
	if err != nil {
		t.Fatal(err)
	}
	return store, chain, block, pending
}
