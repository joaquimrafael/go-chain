package transaction

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestValidateTransferRules(t *testing.T) {
	accounts := map[string]struct{}{"alice": {}, "bob": {}, "Alice": {}}
	confirmed := map[string]int64{"alice": 50}
	for _, tt := range []struct {
		name     string
		transfer Transaction
		want     string
	}{
		{name: "valid", transfer: Transaction{Type: Transfer, From: "alice", To: "bob", Amount: 10}},
		{name: "exact spend", transfer: Transaction{Type: Transfer, From: "alice", To: "bob", Amount: 50}},
		{name: "case-sensitive distinct accounts", transfer: Transaction{Type: Transfer, From: "alice", To: "Alice", Amount: 10}},
		{name: "missing sender", transfer: Transaction{Type: Transfer, From: "charlie", To: "bob", Amount: 10}, want: `sender account "charlie" does not exist`},
		{name: "missing receiver", transfer: Transaction{Type: Transfer, From: "alice", To: "charlie", Amount: 10}, want: `receiver account "charlie" does not exist`},
		{name: "names matched exactly", transfer: Transaction{Type: Transfer, From: " alice ", To: "bob", Amount: 10}, want: "sender account"},
		{name: "same account", transfer: Transaction{Type: Transfer, From: "alice", To: "alice", Amount: 10}, want: "sender and receiver must be different"},
		{name: "zero amount", transfer: Transaction{Type: Transfer, From: "alice", To: "bob"}, want: "amount must be positive"},
		{name: "negative amount", transfer: Transaction{Type: Transfer, From: "alice", To: "bob", Amount: -1}, want: "amount must be positive"},
		{name: "overspend", transfer: Transaction{Type: Transfer, From: "alice", To: "bob", Amount: 51}, want: "50 GOC available, cannot spend 51 GOC"},
		{name: "no confirmed history", transfer: Transaction{Type: Transfer, From: "bob", To: "alice", Amount: 1}, want: "0 GOC available"},
		{name: "reward", transfer: Transaction{Type: Reward, To: "alice", Amount: 50}, want: "transaction type must be transfer"},
		{name: "unknown type", transfer: Transaction{Type: "unknown", From: "alice", To: "bob", Amount: 10}, want: "transaction type must be transfer"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransfer(tt.transfer, accounts, confirmed, nil)
			checkValidationError(t, err, tt.want)
		})
	}
}

func TestValidateTransferPendingFunds(t *testing.T) {
	accounts := map[string]struct{}{"alice": {}, "bob": {}, "charlie": {}}
	for _, tt := range []struct {
		name      string
		confirmed int64
		amount    int64
		pending   []Transaction
		want      string
	}{
		{name: "exact available spend", confirmed: 50, amount: 20, pending: []Transaction{{Type: Transfer, From: "alice", To: "bob", Amount: 30}}},
		{name: "cumulative pending overspend", confirmed: 50, amount: 21, pending: []Transaction{
			{Type: Transfer, From: "alice", To: "bob", Amount: 10},
			{Type: Transfer, From: "alice", To: "charlie", Amount: 20},
		}, want: "20 GOC available, cannot spend 21 GOC"},
		{name: "all funds reserved", confirmed: 50, amount: 1, pending: []Transaction{{Type: Transfer, From: "alice", To: "bob", Amount: 50}}, want: "0 GOC available"},
		{name: "incoming not spendable", confirmed: 0, amount: 1, pending: []Transaction{{Type: Transfer, From: "bob", To: "alice", Amount: 50}}, want: "0 GOC available"},
		{name: "incoming does not offset outgoing", confirmed: 50, amount: 21, pending: []Transaction{
			{Type: Transfer, From: "bob", To: "alice", Amount: 50},
			{Type: Transfer, From: "alice", To: "charlie", Amount: 30},
		}, want: "20 GOC available"},
		{name: "unrelated outgoing ignored", confirmed: 50, amount: 50, pending: []Transaction{{Type: Transfer, From: "bob", To: "charlie", Amount: 30}}},
		{name: "pending already overspent", confirmed: 50, amount: 1, pending: []Transaction{
			{Type: Transfer, From: "alice", To: "bob", Amount: 30},
			{Type: Transfer, From: "alice", To: "charlie", Amount: 30},
		}, want: "pending transaction 1:"},
		{name: "maximum exact spend", confirmed: math.MaxInt64, amount: math.MaxInt64},
		{name: "maximum pending reservation", confirmed: math.MaxInt64, amount: 1, pending: []Transaction{{Type: Transfer, From: "alice", To: "bob", Amount: math.MaxInt64}}, want: "0 GOC available"},
		{name: "pending sum would overflow", confirmed: math.MaxInt64, amount: 1, pending: []Transaction{
			{Type: Transfer, From: "alice", To: "bob", Amount: math.MaxInt64},
			{Type: Transfer, From: "alice", To: "charlie", Amount: math.MaxInt64},
		}, want: "pending transaction 1:"},
		{name: "negative confirmed balance", confirmed: -1, amount: 1, want: "negative confirmed balance"},
		{name: "pending reward rejected", confirmed: 0, amount: 1, pending: []Transaction{{Type: Reward, To: "alice", Amount: 50}}, want: "pending transaction 0: transaction type must be transfer"},
		{name: "negative pending amount rejected", confirmed: 50, amount: 51, pending: []Transaction{{Type: Transfer, From: "alice", To: "bob", Amount: -1}}, want: "pending transaction 0: amount must be positive"},
		{name: "malformed pending accounts", confirmed: 50, amount: 1, pending: []Transaction{{Type: Transfer, From: "alice", To: "missing", Amount: 1}}, want: "pending transaction 0: receiver account"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			confirmed := map[string]int64{"alice": tt.confirmed, "bob": 50}
			beforeConfirmed := map[string]int64{"alice": tt.confirmed, "bob": 50}
			beforePending := append([]Transaction(nil), tt.pending...)
			candidate := Transaction{Type: Transfer, From: "alice", To: "bob", Amount: tt.amount}
			err := ValidateTransfer(candidate, accounts, confirmed, tt.pending)
			checkValidationError(t, err, tt.want)
			if !reflect.DeepEqual(confirmed, beforeConfirmed) || !reflect.DeepEqual(tt.pending, beforePending) {
				t.Fatal("ValidateTransfer() changed confirmed balances or pending transactions")
			}
		})
	}
	wantAccounts := map[string]struct{}{"alice": {}, "bob": {}, "charlie": {}}
	if !reflect.DeepEqual(accounts, wantAccounts) {
		t.Error("ValidateTransfer() changed the account set")
	}
}

func checkValidationError(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("ValidateTransfer() error = %v, want nil", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("ValidateTransfer() error = %v, want containing %q", err, want)
	}
}
