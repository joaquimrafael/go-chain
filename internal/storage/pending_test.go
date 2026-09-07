package storage

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/joaquimrafael/go-chain/internal/transaction"
)

func TestPendingPersistsInInsertionOrder(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "chain.db")
	store := openTestStore(t, path)
	if err := store.Initialize(ctx, 1_700_000_000, 1); err != nil {
		t.Fatal(err)
	}
	pending, err := store.LoadPending(ctx)
	if err != nil || pending == nil || len(pending) != 0 {
		t.Fatalf("initial LoadPending() = %v, %v, want non-nil empty slice", pending, err)
	}
	// Timestamps deliberately differ from insertion order. Identical transfers
	// must remain separate entries, identified by different database IDs.
	want := []PendingTransaction{
		{CreatedAt: 1_700_000_003, Transaction: transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 10}},
		{CreatedAt: 1_700_000_001, Transaction: transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "O'Brien", Amount: 5}},
		{CreatedAt: 1_700_000_003, Transaction: transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 10}},
	}
	for i := range want {
		entry, err := store.AddPending(ctx, want[i].Transaction, want[i].CreatedAt)
		if err != nil {
			t.Fatalf("AddPending() error = %v", err)
		}
		if entry.ID <= 0 || (i > 0 && entry.ID <= want[i-1].ID) {
			t.Fatalf("entry %d ID = %d, want increasing positive ID", i, entry.ID)
		}
		want[i].ID = entry.ID
		if entry != want[i] {
			t.Fatalf("AddPending() = %#v, want %#v", entry, want[i])
		}
	}
	assertPending(t, store, want)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openTestStore(t, path)
	assertPending(t, store, want)
	entry, err := store.AddPending(ctx,
		transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 1}, 1_700_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID <= want[len(want)-1].ID {
		t.Fatalf("ID after reopen = %d, want greater than previous ID", entry.ID)
	}
	want = append(want, entry)
	assertPending(t, store, want)
	chain, err := store.LoadChain(ctx)
	if err != nil {
		t.Fatal(err)
	}
	balances, err := chain.Balances()
	if err != nil || len(chain.Blocks) != 1 || len(balances) != 0 {
		t.Fatalf("pending insertion changed confirmed history: blocks = %d, balances = %v, error = %v", len(chain.Blocks), balances, err)
	}
}

func TestPendingRejectsNonTransferTypes(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "chain.db"))
	if err := store.Initialize(ctx, 1_700_000_000, 1); err != nil {
		t.Fatal(err)
	}
	transfer := transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 10}
	existing, err := store.AddPending(ctx, transfer, 1_700_000_001)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []transaction.Type{transaction.Reward, "unknown", ""} {
		transfer.Type = kind
		entry, err := store.AddPending(ctx, transfer, 1_700_000_002)
		if err == nil || !strings.Contains(err.Error(), "pending transaction type must be transfer") || entry != (PendingTransaction{}) {
			t.Errorf("AddPending(%q) = %#v, %v, want zero result and type error", kind, entry, err)
		}
	}
	assertPending(t, store, []PendingTransaction{existing})
}

func TestPendingCancellationPreservesPool(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "chain.db"))
	if err := store.Initialize(ctx, 1_700_000_000, 1); err != nil {
		t.Fatal(err)
	}
	transfer := transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 10}
	existing, err := store.AddPending(ctx, transfer, 1_700_000_001)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if entry, err := store.AddPending(cancelled, transfer, 1_700_000_002); !errors.Is(err, context.Canceled) || entry != (PendingTransaction{}) {
		t.Errorf("cancelled AddPending() = %#v, %v, want context.Canceled", entry, err)
	}
	if pending, err := store.LoadPending(cancelled); !errors.Is(err, context.Canceled) || pending != nil {
		t.Errorf("cancelled LoadPending() = %v, %v, want context.Canceled", pending, err)
	}
	assertPending(t, store, []PendingTransaction{existing})
}

func TestPendingInsertFailurePreservesPool(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "chain.db"))
	if err := store.Initialize(ctx, 1_700_000_000, 1); err != nil {
		t.Fatal(err)
	}
	transfer := transaction.Transaction{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 10}
	existing, err := store.AddPending(ctx, transfer, 1_700_000_001)
	if err != nil {
		t.Fatal(err)
	}
	// Force a SQLite failure to verify that errors do not look like successful inserts.
	if _, err := store.db.ExecContext(ctx, `
		CREATE TRIGGER reject_pending BEFORE INSERT ON pending_transactions
		BEGIN SELECT RAISE(ABORT, 'test insertion failure'); END`); err != nil {
		t.Fatal(err)
	}
	entry, err := store.AddPending(ctx, transfer, 1_700_000_002)
	if err == nil || !strings.Contains(err.Error(), "add pending transaction") || entry != (PendingTransaction{}) {
		t.Errorf("failed AddPending() = %#v, %v, want zero result and insertion error", entry, err)
	}
	assertPending(t, store, []PendingTransaction{existing})
}

func assertPending(t *testing.T, store *Store, want []PendingTransaction) {
	t.Helper()
	got, err := store.LoadPending(context.Background())
	if err != nil {
		t.Fatalf("LoadPending() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadPending() = %#v, want %#v", got, want)
	}
}
