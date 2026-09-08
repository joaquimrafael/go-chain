package transaction

// Type identifies how a transaction changes account balances.
type Type string

const (
	Transfer Type = "transfer"
	Reward   Type = "reward"
)

// RewardAmount is the fixed number of whole GOC issued by each non-Genesis block.
const RewardAmount int64 = 50

// Transaction records a transfer of whole GOC between accounts or a mining reward.
type Transaction struct {
	Type   Type
	From   string
	To     string
	Amount int64
}
