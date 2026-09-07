package transaction

import "fmt"

// ValidateTransfer checks whether a transfer can be added to the pending pool.
// Names must already be normalized and are matched exactly. Confirmed balances
// come from chain replay; absent entries mean zero. Pending entries must have
// valid transfer fields. Their funding is checked only for this transfer's sender.
// Inputs are never changed, and pending incoming funds are never credited.
func ValidateTransfer(transfer Transaction, accounts map[string]struct{}, confirmed map[string]int64, pending []Transaction) error {
	if err := validateTransferFields(transfer, accounts); err != nil {
		return err
	}
	available := confirmed[transfer.From]
	if available < 0 {
		return fmt.Errorf("account %q has negative confirmed balance %d GOC", transfer.From, available)
	}
	for i, queued := range pending {
		if err := validateTransferFields(queued, accounts); err != nil {
			return fmt.Errorf("pending transaction %d: %w", i, err)
		}
		if queued.From != transfer.From {
			continue
		}
		// Subtract only after checking the remaining funds. Summing arbitrary
		// pending amounts first could overflow int64 and hide an overspend.
		if queued.Amount > available {
			return fmt.Errorf("pending transaction %d: account %q has %d GOC available, cannot reserve %d GOC", i, transfer.From, available, queued.Amount)
		}
		available -= queued.Amount
	}
	if transfer.Amount > available {
		return fmt.Errorf("account %q has %d GOC available, cannot spend %d GOC", transfer.From, available, transfer.Amount)
	}
	return nil
}

func validateTransferFields(transfer Transaction, accounts map[string]struct{}) error {
	if transfer.Type != Transfer {
		return fmt.Errorf("transaction type must be transfer, got %q", transfer.Type)
	}
	if _, exists := accounts[transfer.From]; !exists {
		return fmt.Errorf("sender account %q does not exist", transfer.From)
	}
	if _, exists := accounts[transfer.To]; !exists {
		return fmt.Errorf("receiver account %q does not exist", transfer.To)
	}
	if transfer.From == transfer.To {
		return fmt.Errorf("sender and receiver must be different accounts")
	}
	if transfer.Amount <= 0 {
		return fmt.Errorf("amount must be positive, got %d", transfer.Amount)
	}
	return nil
}
