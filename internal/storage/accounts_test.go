package storage

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAccountsPersistAfterReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "chain.db")
	store := openTestStore(t, path)
	if err := store.Initialize(ctx, 1_700_000_000, 1); err != nil {
		t.Fatal(err)
	}
	names, err := store.LoadAccountNames(ctx)
	if err != nil || names == nil || len(names) != 0 {
		t.Fatalf("initial LoadAccountNames() = %v, %v, want non-nil empty set", names, err)
	}
	const timestamp int64 = 1_700_000_001
	for _, tt := range []struct{ input, want string }{
		{input: " \talice\n", want: "alice"},
		{input: "Alice", want: "Alice"},
		{input: "\u2003bob\u00a0", want: "bob"},
		{input: "miner one", want: "miner one"},
		{input: "O'Brien", want: "O'Brien"},
	} {
		name, err := store.CreateAccount(ctx, tt.input, timestamp)
		if err != nil || name != tt.want {
			t.Fatalf("CreateAccount(%q) = %q, %v, want %q", tt.input, name, err, tt.want)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openTestStore(t, path)
	for _, tt := range []struct {
		name string
		want bool
	}{
		{name: "alice", want: true},
		{name: " Alice ", want: true},
		{name: "ALICE", want: false},
		{name: "bob", want: true},
		{name: "miner one", want: true},
		{name: "O'Brien", want: true},
		{name: "charlie", want: false},
		{name: "' OR 1=1 --", want: false},
	} {
		exists, err := store.AccountExists(ctx, tt.name)
		if err != nil || exists != tt.want {
			t.Errorf("AccountExists(%q) = %v, %v, want %v", tt.name, exists, err, tt.want)
		}
	}
	for _, name := range []string{"alice", " alice ", "Alice"} {
		created, err := store.CreateAccount(ctx, name, timestamp+100)
		if !errors.Is(err, ErrAccountAlreadyExists) || created != "" {
			t.Errorf("duplicate CreateAccount(%q) = %q, %v, want ErrAccountAlreadyExists", name, created, err)
		}
	}
	// A duplicate must not replace the original creation timestamp.
	var createdAt int64
	if err := store.db.QueryRowContext(ctx, "SELECT created_at FROM accounts WHERE name = ?", "alice").Scan(&createdAt); err != nil {
		t.Fatal(err)
	}
	if createdAt != timestamp {
		t.Errorf("created_at = %d, want %d", createdAt, timestamp)
	}
	names, err = store.LoadAccountNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{}{"alice": {}, "Alice": {}, "bob": {}, "miner one": {}, "O'Brien": {}}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("LoadAccountNames() = %v, want %v", names, want)
	}
}

func TestAccountsRejectBlankNames(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "chain.db"))
	if err := store.Initialize(ctx, 1_700_000_000, 1); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", " ", "\t\n\r", "\u2003\u00a0"} {
		if created, err := store.CreateAccount(ctx, name, 1_700_000_001); !errors.Is(err, ErrInvalidAccountName) || created != "" {
			t.Errorf("CreateAccount(%q) = %q, %v, want ErrInvalidAccountName", name, created, err)
		}
		if exists, err := store.AccountExists(ctx, name); !errors.Is(err, ErrInvalidAccountName) || exists {
			t.Errorf("AccountExists(%q) = %v, %v, want ErrInvalidAccountName", name, exists, err)
		}
	}
	names, err := store.LoadAccountNames(ctx)
	if err != nil || len(names) != 0 {
		t.Fatalf("LoadAccountNames() = %v, %v, want no accounts", names, err)
	}
}

func TestAccountsPropagateCancellation(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "chain.db"))
	if err := store.Initialize(ctx, 1_700_000_000, 1); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if name, err := store.CreateAccount(cancelled, "alice", 1_700_000_001); !errors.Is(err, context.Canceled) || name != "" {
		t.Errorf("cancelled CreateAccount() = %q, %v, want context.Canceled", name, err)
	}
	if exists, err := store.AccountExists(cancelled, "alice"); !errors.Is(err, context.Canceled) || exists {
		t.Errorf("cancelled AccountExists() = %v, %v, want context.Canceled", exists, err)
	}
	if names, err := store.LoadAccountNames(cancelled); !errors.Is(err, context.Canceled) || names != nil {
		t.Errorf("cancelled LoadAccountNames() = %v, %v, want context.Canceled", names, err)
	}
	if exists, err := store.AccountExists(ctx, "alice"); err != nil || exists {
		t.Errorf("AccountExists() after cancelled creation = %v, %v, want false", exists, err)
	}
}
