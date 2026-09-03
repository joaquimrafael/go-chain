package blockchain

import "github.com/joaquimrafael/go-chain/internal/transaction"

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
