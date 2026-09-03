package transaction

// Type identifies how a transaction changes account balances.
type Type string

const (
	Transfer Type = "transfer"
	Reward   Type = "reward"
)

// Transaction records a transfer of whole GOC between accounts or a mining reward.
type Transaction struct {
	Type   Type
	From   string
	To     string
	Amount int64
}
