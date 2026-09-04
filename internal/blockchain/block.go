package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/joaquimrafael/go-chain/internal/transaction"
)

// Block records an ordered group of confirmed transactions in the blockchain.
type Block struct {
	Height       int64
	Timestamp    int64
	PreviousHash string
	Hash         string
	Nonce        uint64
	Difficulty   int
	Transactions []transaction.Transaction
}

// hashPayload contains exactly the block data protected by its hash.
// A struct keeps JSON serialization deterministic and deliberately omits Hash.
type hashPayload struct {
	Height       int64                     `json:"height"`
	Timestamp    int64                     `json:"timestamp"`
	PreviousHash string                    `json:"previous_hash"`
	Nonce        uint64                    `json:"nonce"`
	Difficulty   int                       `json:"difficulty"`
	Transactions []transaction.Transaction `json:"transactions"`
}

// CalculateHash returns the lowercase hexadecimal SHA-256 hash of the block's
// contents. The stored Hash field is excluded so it cannot hash itself.
func (b Block) CalculateHash() string {
	payload := hashPayload{
		Height:       b.Height,
		Timestamp:    b.Timestamp,
		PreviousHash: b.PreviousHash,
		Nonce:        b.Nonce,
		Difficulty:   b.Difficulty,
		Transactions: b.Transactions,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		panic("blockchain: marshal hash payload: " + err.Error())
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
