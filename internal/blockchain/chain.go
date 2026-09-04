package blockchain

import "errors"

// ErrEmptyBlockchain is returned when an operation needs at least Genesis.
var ErrEmptyBlockchain = errors.New("blockchain is empty")

// Blockchain keeps blocks in height order while a command is running.
type Blockchain struct {
	Blocks []Block
}

// NewBlockchain starts an in-memory chain with its Genesis block.
func NewBlockchain(genesis Block) Blockchain {
	return Blockchain{Blocks: []Block{genesis}}
}

// Tip returns the latest block in the chain.
func (bc Blockchain) Tip() (Block, error) {
	if len(bc.Blocks) == 0 {
		return Block{}, ErrEmptyBlockchain
	}

	return bc.Blocks[len(bc.Blocks)-1], nil
}

// NextHeight returns the height immediately after the current tip.
func (bc Blockchain) NextHeight() (int64, error) {
	tip, err := bc.Tip()
	if err != nil {
		return 0, err
	}

	return tip.Height + 1, nil
}

// Difficulty returns the fixed chain difficulty established by Genesis.
func (bc Blockchain) Difficulty() (int, error) {
	if len(bc.Blocks) == 0 {
		return 0, ErrEmptyBlockchain
	}

	return bc.Blocks[0].Difficulty, nil
}
