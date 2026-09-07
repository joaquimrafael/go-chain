package storage

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/joaquimrafael/go-chain/internal/blockchain"
	"github.com/joaquimrafael/go-chain/internal/transaction"
)

func openTestStore(t *testing.T, path string) *Store {
	t.Helper()
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return store
}

func TestInitializeAndReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "chain ?#%.db")
	store := openTestStore(t, path)
	if _, err := store.LoadChain(ctx); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("LoadChain() before initialization error = %v, want %v", err, ErrNotInitialized)
	}
	if err := store.Initialize(ctx, 1_700_000_000, 3); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	for _, table := range []string{"accounts", "blocks", "block_transactions", "pending_transactions"} {
		var exists bool
		err := store.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = ?)", table).Scan(&exists)
		if err != nil || !exists {
			t.Fatalf("schema table %q exists = %v, error = %v", table, exists, err)
		}
	}
	genesis, err := blockchain.NewGenesisBlock(1_700_000_000, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := blockchain.NewBlockchain(genesis)
	assertChain(t, store, want)
	if err := store.Close(); err != nil {
		t.Fatalf("Close() before reopen error = %v", err)
	}
	store = openTestStore(t, path)
	assertChain(t, store, want)
	if err := store.Initialize(ctx, 1_800_000_000, 1); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second Initialize() error = %v, want %v", err, ErrAlreadyInitialized)
	}
	assertChain(t, store, want)
}

func TestInitializeInvalidDifficultyAndCancellation(t *testing.T) {
	for _, difficulty := range []int{0, 65} {
		t.Run(strconv.Itoa(difficulty), func(t *testing.T) {
			store := openTestStore(t, filepath.Join(t.TempDir(), "chain.db"))
			ctx := context.Background()
			if err := store.Initialize(ctx, 1_700_000_000, difficulty); err == nil {
				t.Fatal("Initialize() accepted invalid difficulty")
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if err := store.Initialize(cancelled, 1_700_000_000, 1); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled Initialize() error = %v, want context.Canceled", err)
			}
			if _, err := store.LoadChain(ctx); !errors.Is(err, ErrNotInitialized) {
				t.Fatalf("LoadChain() error = %v, want %v", err, ErrNotInitialized)
			}
			if err := store.Initialize(ctx, 1_700_000_000, 1); err != nil {
				t.Fatalf("Initialize() retry error = %v", err)
			}
		})
	}
}

func TestInitializeRollsBackSchemaOnGenesisFailure(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "chain.db"))
	ctx := context.Background()
	// Keep a pre-existing blocks table and force the Genesis insert to fail.
	if _, err := store.db.ExecContext(ctx, `
		CREATE TABLE blocks (
		    height INTEGER PRIMARY KEY CHECK (height < 0), timestamp INTEGER,
		    previous_hash TEXT, hash TEXT, nonce TEXT, difficulty INTEGER
		)`); err != nil {
		t.Fatal(err)
	}
	if err := store.Initialize(ctx, 1_700_000_000, 1); err == nil || !strings.Contains(err.Error(), "store Genesis") {
		t.Fatalf("Initialize() error = %v, want Genesis insertion failure", err)
	}
	var tables int
	if err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 1 {
		t.Errorf("tables after rollback = %d, want only the original blocks table", tables)
	}
	if _, err := store.LoadChain(ctx); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("LoadChain() after rollback error = %v, want %v", err, ErrNotInitialized)
	}
}

func TestLoadChainPreservesBlockAndTransactionOrder(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "chain.db")
	store := openTestStore(t, path)
	if err := store.Initialize(ctx, 1_700_000_000, 1); err != nil {
		t.Fatal(err)
	}
	chain, err := store.LoadChain(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for height := int64(1); height <= 2; height++ {
		block := blockchain.Block{
			Height: height, Timestamp: 1_700_000_000 + height,
			PreviousHash: chain.Blocks[len(chain.Blocks)-1].Hash,
			Nonce:        math.MaxUint64, Difficulty: 1,
			Transactions: []transaction.Transaction{
				{Type: transaction.Reward, To: "alice", Amount: 50},
				{Type: transaction.Transfer, From: "alice", To: "bob", Amount: height},
				{Type: transaction.Transfer, From: "bob", To: "alice", Amount: height},
			},
		}
		block.Hash = block.CalculateHash()
		chain.Blocks = append(chain.Blocks, block)
	}
	// Direct SQL is fixture setup only; mined-block persistence comes later.
	// Reverse insertion order proves loading uses height and position, not IDs.
	for i := len(chain.Blocks) - 1; i > 0; i-- {
		block := chain.Blocks[i]
		if _, err := store.db.ExecContext(ctx, `
			INSERT INTO blocks (height, timestamp, previous_hash, hash, nonce, difficulty)
			VALUES (?, ?, ?, ?, ?, ?)`, block.Height, block.Timestamp, block.PreviousHash,
			block.Hash, strconv.FormatUint(block.Nonce, 10), block.Difficulty); err != nil {
			t.Fatal(err)
		}
		for position := len(block.Transactions) - 1; position >= 0; position-- {
			transfer := block.Transactions[position]
			if _, err := store.db.ExecContext(ctx, `
				INSERT INTO block_transactions (block_height, position, type, sender, receiver, amount)
				VALUES (?, ?, ?, ?, ?, ?)`, block.Height, position, transfer.Type,
				transfer.From, transfer.To, transfer.Amount); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openTestStore(t, path)
	assertChain(t, store, chain)

	// Loading must preserve corruption so independent validation can detect it.
	if _, err := store.db.ExecContext(ctx, "UPDATE blocks SET hash = 'corrupt' WHERE height = 0"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadChain(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Blocks[0].Hash != "corrupt" {
		t.Fatal("LoadChain() replaced the stored hash")
	}
	if err := blockchain.ValidateStructure(loaded); err == nil || !strings.Contains(err.Error(), "block 0: stored hash") {
		t.Fatalf("ValidateStructure() error = %v, want corrupt Genesis hash", err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE blocks SET nonce = '18446744073709551616' WHERE height = 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadChain(ctx); err == nil || !strings.Contains(err.Error(), "block 1: invalid nonce") {
		t.Fatalf("LoadChain() error = %v, want invalid nonce in block 1", err)
	}
}

func assertChain(t *testing.T, store *Store, want blockchain.Blockchain) {
	t.Helper()
	got, err := store.LoadChain(context.Background())
	if err != nil {
		t.Fatalf("LoadChain() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadChain() = %#v, want %#v", got, want)
	}
	for _, block := range got.Blocks {
		if hash := block.CalculateHash(); hash != block.Hash {
			t.Errorf("block %d reloaded hash = %q, want %q", block.Height, hash, block.Hash)
		}
	}
}
