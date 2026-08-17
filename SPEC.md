# GoChain — Technical Specification

## 1. Purpose

This document describes the technical design of the GoChain MVP.

GoChain is a single-node, local, educational blockchain implemented in Go.

The design intentionally favors:

- small components;
- explicit data flow;
- straightforward Go code;
- easy debugging;
- easy testing;
- educational value.

It does not attempt to model a production blockchain protocol.

---

## 2. High-Level Architecture

The application consists of four main areas:

```text
CLI
 │
 ▼
Blockchain / Transactions
 │
 ▼
Mining / Proof of Work
 │
 ▼
SQLite Storage
```

Responsibilities:

### CLI

Receives commands from the user and coordinates application operations.

Examples:

```text
init
create-account
send
pending
mine
balance
chain
validate
```

### Blockchain

Contains the core blockchain rules:

- blocks;
- hashes;
- chain linking;
- transaction replay;
- balances;
- chain validation.

### Mining

Creates candidate blocks and performs Proof of Work.

### Storage

Persists:

- accounts;
- blocks;
- confirmed transactions;
- pending transactions.

SQLite is the only persistence mechanism.

---

## 3. Suggested Project Structure

A small package structure is preferred.

```text
cmd/
    gochain/
        main.go
        commands.go

internal/
    blockchain/
        block.go
        chain.go
        balance.go
        validation.go

    transaction/
        transaction.go
        validation.go

    mining/
        pow.go
        miner.go

    storage/
        sqlite.go
        accounts.go
        blocks.go
        pending.go

README.md
PRD.md
SPEC.md
go.mod
```

This structure is a guideline, not a strict requirement.

Files should be combined when splitting them would make the project harder to navigate.

Interfaces should not be introduced unless an actual implementation boundary requires one.

---

## 4. Currency Model

GoChain uses:

```text
GoCoin
GOC
```

Amounts are represented as integers.

Example:

```text
10 = 10 GOC
50 = 50 GOC
```

The MVP does not support fractional GOC.

Go should use an integer type such as:

```text
int64
```

for amounts.

Floating-point numbers must not be used for currency.

The fixed mining reward is:

```text
50 GOC
```

---

## 5. Account Model

An account is identified by a unique human-readable name.

Examples:

```text
alice
bob
miner1
```

Conceptually:

```go
Account {
    Name string
}
```

Accounts have no:

- private keys;
- public keys;
- passwords;
- signatures;
- cryptographic addresses.

The account exists only to make transactions and balances easy to understand.

Accounts are persisted in SQLite.

---

## 6. Transaction Model

There are two transaction types:

```text
transfer
reward
```

A conceptual transaction structure is:

```go
Transaction {
    Type   TransactionType
    From   string
    To     string
    Amount int64
}
```

### Transfer Transaction

Example:

```text
alice -> bob : 10 GOC
```

Conceptually:

```text
Type   = transfer
From   = alice
To     = bob
Amount = 10
```

### Reward Transaction

Mining rewards are represented as special transactions.

Example:

```text
SYSTEM -> alice : 50 GOC
```

Conceptually:

```text
Type   = reward
From   = ""
To     = alice
Amount = 50
```

The empty sender represents newly created currency.

Reward transactions never enter the pending transaction pool.

They are created directly during block mining.

---

## 7. Transaction Rules

A normal transfer is valid when:

- sender exists;
- receiver exists;
- sender and receiver are different;
- amount is greater than zero;
- sender has enough available funds.

Confirmed balance is calculated only from the blockchain.

However, when creating another pending transaction, already-pending outgoing transactions must also be considered.

Example:

```text
Alice confirmed balance: 50 GOC

Pending:
Alice -> Bob   30 GOC
```

Alice's confirmed balance remains:

```text
50 GOC
```

but her currently available amount for creating new transactions is:

```text
20 GOC
```

This prevents:

```text
Alice -> Bob     30
Alice -> Charlie 30
```

from creating 60 GOC of pending spending from a 50 GOC balance.

Pending incoming transactions are not spendable until mined.

---

## 8. Block Model

A block conceptually contains:

```go
Block {
    Height       int64
    Timestamp    int64
    PreviousHash string
    Hash         string
    Nonce        uint64
    Difficulty   int
    Transactions []Transaction
}
```

### Height

Sequential position in the blockchain.

```text
Genesis = 0
Next    = 1
Next    = 2
```

### Timestamp

Unix timestamp indicating block creation time.

It is informational and also participates in the block hash.

### PreviousHash

Hash of the previous block.

This creates the chain:

```text
Block 1
PreviousHash -> Block 0 Hash

Block 2
PreviousHash -> Block 1 Hash
```

### Hash

SHA-256 hash calculated from the block content.

The `Hash` field itself is not included when calculating the hash.

### Nonce

Integer changed by Proof of Work until a valid hash is found.

### Difficulty

Number of leading hexadecimal zeroes required in the block hash.

Example:

```text
difficulty = 3

valid:
000a91f...

invalid:
001a91f...
```

### Transactions

Ordered list of transactions confirmed by the block.

Transaction order must remain stable because it affects the block hash.

---

## 9. Block Hashing

GoChain uses SHA-256.

The block hash must depend on at least:

```text
Height
Timestamp
PreviousHash
Nonce
Difficulty
Transactions
```

Conceptually:

```text
block data
    ↓
deterministic serialization
    ↓
SHA-256
    ↓
hexadecimal hash
```

The implementation should use deterministic serialization.

A straightforward solution is to serialize a dedicated hash payload structure using Go's standard JSON support.

Maps should not be used in the hash payload.

The stored `Hash` field must never be part of its own hash calculation.

---

## 10. Genesis Block

The first block is the Genesis Block.

Properties:

```text
Height       = 0
PreviousHash = 64 zero characters
Transactions = []
```

The Genesis Block does not create currency.

Its hash is calculated normally from its contents.

The MVP does not require Proof of Work for the Genesis Block.

Blockchain validation treats it as the special starting point of the chain.

---

## 11. Blockchain Model

At runtime, the blockchain may be represented conceptually as:

```go
Blockchain {
    Blocks []Block
}
```

Because GoChain is intentionally small, the complete blockchain can be loaded from SQLite when a CLI command starts.

This avoids:

- caching;
- complex repositories;
- partial chain loading;
- state synchronization.

For the expected educational dataset, replaying the complete chain is acceptable.

The blockchain in SQLite remains the persistent source of truth.

---

## 12. Balance Calculation

Balances are reconstructed by replaying confirmed transactions in blockchain order.

Initial state:

```text
every account = 0 GOC
```

For each block, transactions are processed in their stored order.

### Reward

```text
SYSTEM -> Alice : 50
```

Produces:

```text
Alice += 50
```

### Transfer

```text
Alice -> Bob : 10
```

Produces:

```text
Alice -= 10
Bob   += 10
```

Example:

```text
Block 1
SYSTEM -> Alice : 50

Block 2
SYSTEM -> Alice : 50
Alice -> Bob : 10
```

Result:

```text
Alice = 90 GOC
Bob   = 10 GOC
```

No authoritative `balances` table exists.

---

## 13. Pending Transaction Pool

The logical mempool is persisted in SQLite.

Conceptually:

```text
Pending Transactions

1. Alice -> Bob     10
2. Alice -> Charlie 5
```

Persistence is required because every CLI command runs as a separate process.

Example:

```text
gochain send
    ↓
process ends

gochain pending
    ↓
new process
```

An in-memory-only mempool would lose transactions between these commands.

SQLite therefore acts as persistent storage for the educational mempool.

---

## 14. Mining Flow

Mining follows this flow:

```text
Load blockchain
        ↓
Load pending transactions
        ↓
Validate pending transactions
        ↓
Create reward transaction
        ↓
Create candidate block
        ↓
Run Proof of Work
        ↓
Validate candidate
        ↓
Persist block
        ↓
Remove included pending transactions
```

The reward transaction should appear first in the block transaction list.

A block can be mined with no pending transfers.

In that case:

```text
Block Transactions
└── Reward -> Miner : 50 GOC
```

---

## 15. Proof of Work

Proof of Work uses a simple leading-zero rule.

Example:

```text
difficulty = 4
```

A valid block hash must begin with:

```text
0000
```

Mining algorithm:

```text
nonce = 0

loop:
    calculate block hash

    if hash satisfies difficulty:
        success

    nonce++
```

The implementation should prioritize readability over mining performance.

No:

- worker pool;
- GPU mining;
- parallel nonce ranges;
- adaptive difficulty;
- optimized binary target representation.

---

## 16. Mining Cancellation and Concurrency

Mining should accept:

```go
context.Context
```

The Proof of Work loop periodically checks whether the context has been cancelled.

Conceptually:

```text
CLI
 │
 ├── mining goroutine
 │       ↓
 │    Proof of Work
 │
 └── Ctrl+C
         ↓
    cancel context
         ↓
    miner stops
```

The CLI may run mining in a single goroutine and receive the result through a small result channel.

This is intentionally the only meaningful concurrency required by the MVP.

The goal is to practice:

- goroutines;
- `context.Context`;
- cancellation;
- channels;
- clean shutdown.

It is not intended to improve mining performance.

If mining is cancelled:

- no block is persisted;
- no mining reward is created;
- pending transactions remain pending.

---

## 17. Blockchain Validation

Validation should replay the blockchain from the Genesis Block to the latest block.

### Genesis Checks

Validate:

- height is `0`;
- previous hash is the expected zero value;
- stored hash matches recalculated hash;
- no normal transactions exist.

### Block Checks

For every non-genesis block:

1. height follows the previous block;
2. `PreviousHash` equals the previous block's `Hash`;
3. stored hash equals the recalculated hash;
4. hash satisfies the block difficulty;
5. exactly one mining reward exists;
6. reward amount equals `50 GOC`;
7. reward transaction is the first transaction;
8. referenced accounts exist;
9. normal transaction amounts are positive;
10. transaction replay never produces an invalid negative sender balance.

Validation should return an understandable error identifying the first invalid block or invariant.

Example:

```text
block 3: previous hash does not match block 2
```

or:

```text
block 4: stored hash does not match calculated hash
```

This makes blockchain corruption easy to demonstrate.

---

## 18. SQLite Persistence

SQLite is accessed through Go's:

```text
database/sql
```

plus one SQLite driver.

A pure-Go SQLite driver is preferred to reduce local environment setup, but the exact driver can be selected during implementation.

The schema should remain explicit and small.

### Accounts

Conceptually:

```text
accounts

name
created_at
```

`name` is unique.

### Blocks

Conceptually:

```text
blocks

height
timestamp
previous_hash
hash
nonce
difficulty
```

`height` is unique.

### Confirmed Transactions

Conceptually:

```text
block_transactions

id
block_height
position
type
sender
receiver
amount
```

`position` preserves transaction order inside a block.

### Pending Transactions

Conceptually:

```text
pending_transactions

id
sender
receiver
amount
created_at
```

The database ID exists for persistence purposes and does not need to be part of the blockchain transaction model.

---

## 19. Mining Persistence Transaction

After Proof of Work succeeds, confirmation should occur inside one SQLite transaction.

Conceptually:

```text
BEGIN

insert block

insert block transactions

delete included pending transactions

COMMIT
```

If any operation fails:

```text
ROLLBACK
```

This guarantees that the application cannot end in a state such as:

```text
block persisted
but
pending transactions still present
```

or:

```text
pending transactions deleted
but
block not persisted
```

This SQLite transaction is unrelated to a blockchain transaction.

The distinction should be clearly explained during implementation because it is educationally useful.

---

## 20. Initialization

Running:

```text
gochain init
```

should:

1. create/open the SQLite database;
2. create required tables;
3. ensure the blockchain has not already been initialized;
4. create the Genesis Block;
5. persist it.

Difficulty may be configurable during initialization.

Conceptually:

```text
gochain init --difficulty 3
```

If omitted, a low development default should be used.

The chain difficulty remains fixed for the MVP.

---

## 21. CLI Behavior

The CLI should remain small and may use Go's standard library for command parsing rather than introducing a CLI framework unless one becomes clearly beneficial.

Expected commands:

```text
gochain init
gochain create-account <name>
gochain balance <name>
gochain send --from <name> --to <name> --amount <amount>
gochain pending
gochain mine --miner <name>
gochain chain
gochain validate
```

Output should favor readability.

Example:

```text
Block #2
Hash:          000a4f...
Previous Hash: 000c92...
Nonce:         4812
Transactions:
  REWARD -> alice : 50 GOC
  alice  -> bob   : 10 GOC
```

The CLI is an educational interface, not a machine-oriented API.

---

## 22. Main Application Flows

### Create Account

```text
CLI
 ↓
validate account name
 ↓
SQLite
 ↓
persist account
```

### Send Transaction

```text
CLI
 ↓
load accounts
 ↓
calculate confirmed balance
 ↓
load pending outgoing transactions
 ↓
calculate available balance
 ↓
validate transfer
 ↓
persist pending transaction
```

### Check Balance

```text
CLI
 ↓
load blockchain
 ↓
replay transactions
 ↓
return confirmed balance
```

### Mine

```text
CLI
 ↓
load blockchain
 ↓
load pending transactions
 ↓
build candidate block
 ↓
Proof of Work
 ↓
validate block
 ↓
SQLite transaction
 ├── persist block
 ├── persist confirmed transactions
 └── delete pending transactions
```

### Validate

```text
CLI
 ↓
load full blockchain
 ↓
recalculate hashes
 ↓
verify links
 ↓
verify Proof of Work
 ↓
replay transactions
 ↓
report result
```

---

## 23. Important Invariants

The implementation should protect the following invariants:

### Chain

```text
block[n].PreviousHash == block[n-1].Hash
```

### Height

```text
block[n].Height == block[n-1].Height + 1
```

### Hash

```text
block.Hash == SHA256(block hash payload)
```

### Proof of Work

Every non-genesis block hash satisfies its difficulty.

### Reward

Every non-genesis block contains exactly one valid reward transaction.

### Balances

Confirmed transaction replay must never allow a sender to spend more confirmed funds than available at that point in history.

### Pending Spending

A user cannot create pending outgoing transfers whose combined amount exceeds the user's confirmed balance.

### Persistence

A mined block and removal of its included pending transactions are committed atomically.

### Cancellation

Cancelled mining does not change persisted blockchain state.

---

## 24. Testing Strategy

Tests should focus on behavior rather than maximizing coverage percentages.

### Unit Tests

Recommended cases:

#### Hashing

- same block data produces the same hash;
- changing nonce changes hash;
- changing transaction data changes hash;
- changing previous hash changes hash.

#### Proof of Work

- miner eventually produces a hash satisfying low difficulty;
- returned nonce produces the stored hash;
- cancelled context stops mining.

#### Balances

- rewards increase miner balance;
- transfers decrease sender balance;
- transfers increase receiver balance;
- multiple blocks replay correctly.

#### Transaction Validation

- nonexistent accounts are rejected;
- zero amount is rejected;
- negative amount is rejected;
- overspending is rejected;
- pending outgoing transactions reduce available spending capacity.

#### Blockchain Validation

- valid chain succeeds;
- modified transaction is detected;
- modified previous hash is detected;
- modified nonce/hash is detected;
- invalid reward is detected.

### Integration Tests

Recommended cases:

- initialize SQLite database;
- persist and reload blocks;
- persist and reload pending transactions;
- application state survives reopening the database;
- successful mining removes included pending transactions;
- cancelled mining leaves pending transactions untouched.

Tests should remain easy to read.

---

## 25. README Expectations

The final README should explain the project as an engineering and educational exercise.

Suggested sections:

```text
GoChain
Why this project exists
What it implements
Architecture
Blockchain flow
Getting started
CLI commands
Complete example
How hashing works
How Proof of Work works
How balances work
How validation detects tampering
Project structure
Intentional simplifications
Tests
Possible future experiments
```

A complete CLI example should demonstrate:

```text
create Alice
create Bob
mine for Alice
send GOC to Bob
inspect pending transaction
mine transaction
check balances
inspect blockchain
validate blockchain
```

A useful educational demonstration is to manually modify persisted blockchain data and show that:

```text
gochain validate
```

detects the corruption.

---

## 26. Implementation Principles for AI Agents

When implementing GoChain, the coding agent must work incrementally.

Before each implementation step, explain:

1. what is being built;
2. why it exists;
3. what blockchain concept it represents;
4. what files will change.

After implementation, explain:

5. how the implementation works;
6. how to run or test it;
7. which code deserves careful study.

Prefer small stages that can be executed and understood independently.

Avoid introducing future abstractions early.

Do not implement infrastructure merely because a production blockchain might require it.

---

## 27. Suggested Implementation Boundaries

The exact implementation plan should be produced separately by the coding agent.

However, the natural conceptual progression is:

```text
Basic project
    ↓
Block + hashing
    ↓
Blockchain + Genesis
    ↓
Chain validation
    ↓
SQLite persistence
    ↓
Accounts
    ↓
Transactions
    ↓
Balance replay
    ↓
Pending transaction pool
    ↓
Proof of Work
    ↓
Mining + rewards
    ↓
Mining cancellation
    ↓
CLI integration
    ↓
Integration tests
    ↓
README / final demo
```

These are conceptual boundaries, not a mandatory implementation plan.

The coding agent should create implementation plans from this specification rather than treating this list as fixed tasks.

---

## 28. Final Design Principle

Whenever implementation choices conflict, prefer the option that makes this flow easiest to understand:

```text
Transaction
    ↓
Pending Pool
    ↓
Block
    ↓
Hash
    ↓
Proof of Work
    ↓
Blockchain
    ↓
Validation
    ↓
Balance
```

GoChain succeeds when the developer can explain why each component exists and how the complete flow works.
