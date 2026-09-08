package mining

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/joaquimrafael/go-chain/internal/blockchain"
	"github.com/joaquimrafael/go-chain/internal/transaction"
)

func TestProofOfWork(t *testing.T) {
	tests := []struct {
		difficulty int
		nonce      uint64
		hash       string
	}{
		{1, 2, "0caec0d9ef4eceb244f15d0924c6f9bb463b4531ce0ddae9572b7c5513cb9137"},
		{2, 709, "00f1ead894f96b96341fdbe376107c7762176f76b58e0a473d96bbcdda5a5ac2"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("difficulty %d", tt.difficulty), func(t *testing.T) {
			input := candidate(tt.difficulty)
			got, err := ProofOfWork(t.Context(), input)
			if err != nil {
				t.Fatalf("ProofOfWork() error = %v", err)
			}
			if got.Nonce != tt.nonce || got.Hash != tt.hash {
				t.Errorf("nonce/hash = %d/%s, want %d/%s", got.Nonce, got.Hash, tt.nonce, tt.hash)
			}
			if got.Hash != got.CalculateHash() || !strings.HasPrefix(got.Hash, strings.Repeat("0", tt.difficulty)) {
				t.Errorf("mined hash does not match contents or difficulty: %s", got.Hash)
			}
			want := candidate(tt.difficulty)
			want.Nonce, want.Hash = tt.nonce, tt.hash
			if !reflect.DeepEqual(got, want) {
				t.Errorf("mining changed fields other than nonce and hash: %+v", got)
			}
			if !reflect.DeepEqual(input, candidate(tt.difficulty)) {
				t.Errorf("mining changed input: %+v", input)
			}
			// Even a previously mined candidate must start searching at zero.
			again, err := ProofOfWork(t.Context(), got)
			if err != nil || !reflect.DeepEqual(again, got) {
				t.Errorf("repeated mining = %+v, %v; want identical result", again, err)
			}
		})
	}
}

func TestProofOfWorkAcceptsNonceZero(t *testing.T) {
	input := candidate(1)
	input.Timestamp = 1_700_000_051
	got, err := ProofOfWork(t.Context(), input)
	if err != nil {
		t.Fatalf("ProofOfWork() error = %v", err)
	}
	const wantHash = "0eb3816eb19cc7df247e44d7b2e4937210899149a748a3db992120e5dfd0f3c3"
	if got.Nonce != 0 || got.Hash != wantHash {
		t.Errorf("nonce/hash = %d/%s, want 0/%s", got.Nonce, got.Hash, wantHash)
	}
}

func TestProofOfWorkInvalidDifficulty(t *testing.T) {
	for _, difficulty := range []int{-1, 0, 65} {
		t.Run(fmt.Sprint(difficulty), func(t *testing.T) {
			input := candidate(difficulty)
			got, err := ProofOfWork(t.Context(), input)
			if err == nil || !strings.Contains(err.Error(), "difficulty must be between 1 and 64") {
				t.Fatalf("ProofOfWork() error = %v, want difficulty error", err)
			}
			if !reflect.DeepEqual(got, blockchain.Block{}) || !reflect.DeepEqual(input, candidate(difficulty)) {
				t.Errorf("failure returned a candidate or changed input: %+v, %+v", got, input)
			}
		})
	}
}

func TestProofOfWorkAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	input := candidate(1)
	got, err := ProofOfWork(ctx, input)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ProofOfWork() error = %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(got, blockchain.Block{}) || !reflect.DeepEqual(input, candidate(1)) {
		t.Errorf("cancellation returned a candidate or changed input: %+v, %+v", got, input)
	}
}

func TestProofOfWorkDeadlineStopsSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	input := candidate(blockchain.MaxDifficulty)
	type result struct {
		block blockchain.Block
		err   error
	}
	done := make(chan result, 1)
	go func() {
		block, err := ProofOfWork(ctx, input)
		done <- result{block, err}
	}()
	select {
	case got := <-done:
		if !errors.Is(got.err, context.DeadlineExceeded) {
			t.Fatalf("ProofOfWork() error = %v, want context.DeadlineExceeded", got.err)
		}
		if !reflect.DeepEqual(got.block, blockchain.Block{}) || !reflect.DeepEqual(input, candidate(blockchain.MaxDifficulty)) {
			t.Errorf("cancellation returned a candidate or changed input: %+v, %+v", got.block, input)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ProofOfWork did not stop promptly after its deadline")
	}
}

func candidate(difficulty int) blockchain.Block {
	return blockchain.Block{
		Height:       1,
		Timestamp:    1_700_000_000,
		PreviousHash: blockchain.GenesisPreviousHash,
		Nonce:        math.MaxUint64,
		Hash:         "old hash",
		Difficulty:   difficulty,
		Transactions: []transaction.Transaction{
			{Type: transaction.Reward, To: "alice", Amount: 50},
			{Type: transaction.Transfer, From: "alice", To: "bob", Amount: 10},
		},
	}
}
