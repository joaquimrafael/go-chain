# GoChain — Product Requirements Document

## 1. Overview

**GoChain** is a small educational blockchain implemented in Go.

The project is designed to teach blockchain fundamentals through implementation while also providing practical experience with Go, SQLite, concurrency, testing, and structured AI-assisted development.

GoChain is intentionally **not** a production-ready blockchain or cryptocurrency.

The guiding principle is:

> Prefer simplicity and educational value over production realism and complexity.

---

## 2. Goals

The project should help the developer understand, by implementing them:

- transactions;
- blocks;
- cryptographic hashing;
- block linking;
- Proof of Work;
- blockchain validation;
- mining;
- pending transactions;
- mining rewards;
- account balances derived from blockchain history;
- simple persistence with SQLite;
- basic concurrency and cancellation in Go;
- meaningful automated testing.

The project should also provide a manageable, well-documented codebase for practicing idiomatic Go and presenting the project as a backend engineering portfolio piece.

---

## 3. Core User Journey

A user should be able to run a local GoChain instance and perform a flow similar to:

```bash
gochain init

gochain create-account alice
gochain create-account bob

gochain balance alice

gochain mine --miner alice

gochain balance alice

gochain send --from alice --to bob --amount 10

gochain pending

gochain mine --miner alice

gochain balance alice
gochain balance bob

gochain chain

gochain validate
```

Conceptually, this should demonstrate:

```text
Alice mines a block
        ↓
Alice receives GOC
        ↓
Alice creates a transaction to Bob
        ↓
Transaction becomes pending
        ↓
Miner creates a new block
        ↓
Proof of Work is executed
        ↓
Block is added to the chain
        ↓
Transaction becomes confirmed
        ↓
Balances reflect the new blockchain state
```

---

## 4. Functional Scope

### Blockchain

GoChain must support:

- creation of a blockchain;
- a Genesis Block;
- blocks linked through the previous block hash;
- SHA-256 hashing;
- sequential block heights;
- blockchain integrity validation.

### Accounts

Users can create accounts identified by simple names.

Examples:

```text
alice
bob
miner1
```

Accounts do not use public/private key cryptography in the MVP.

### Transactions

Users can create transactions between accounts.

Example:

```text
alice -> bob : 10 GOC
```

A transaction must contain enough information to identify:

- sender;
- receiver;
- amount.

Transactions that have not yet been mined are considered pending.

### Pending Transactions

Pending transactions must survive separate CLI executions.

They will therefore be persisted locally using SQLite.

Once included in a valid mined block, they are no longer pending.

### Mining

A user can mine a new block.

Mining must:

1. collect pending transactions;
2. create a candidate block;
3. include a mining reward;
4. execute Proof of Work;
5. add the valid block to the blockchain;
6. mark included transactions as confirmed.

Mining should also be possible when no user transactions are pending, allowing the miner to receive the block reward.

### Proof of Work

GoChain must implement a simple Proof of Work mechanism based on:

- a nonce;
- a configurable difficulty;
- repeated block hashing until a valid hash is found.

Difficulty should remain low enough for local development.

No automatic difficulty adjustment is required.

### Currency

The native educational currency is:

```text
GoCoin
GOC
```

Mining a block creates a fixed reward.

Initial reward:

```text
50 GOC
```

No halving or maximum supply is required.

### Balances

Account balances must be derived from confirmed blockchain transactions.

The blockchain is the source of truth.

Balances should not be maintained as an independent authoritative state.

Pending transactions must not affect confirmed balances.

### Persistence

SQLite must be used to persist the blockchain, accounts, and pending transactions.

The application state must survive process termination and restart.

The persistence model should prioritize readability and simplicity over performance.

### CLI

The project must expose a simple command-line interface for interacting with the blockchain.

Expected capabilities include:

- initialize blockchain;
- create accounts;
- create transactions;
- inspect pending transactions;
- mine blocks;
- inspect account balances;
- inspect the blockchain;
- validate the blockchain.

Exact command names may evolve during implementation.

---

## 5. Validation Requirements

The application must reject clearly invalid operations such as:

- sending coins from a nonexistent account;
- sending coins to a nonexistent account;
- sending zero or negative amounts;
- sending more available GOC than the sender owns;
- adding an invalid block to the blockchain;
- accepting a blockchain whose hashes or links are inconsistent.

Pending outgoing transactions should be considered when determining whether an account can create another transaction, preventing multiple pending transactions from collectively spending more than the confirmed balance.

Validation rules should remain intentionally small and understandable.

---

## 6. Persistence Requirements

After restarting the application, the following must still be available:

- accounts;
- mined blocks;
- transactions contained in blocks;
- pending transactions.

Balances must be reproducible from persisted blockchain history.

When mining succeeds, storing the new block and removing its pending transactions should behave as one consistent persistence operation.

If mining is cancelled or fails, pending transactions must remain available.

---

## 7. Engineering Quality

The MVP should include a focused automated test suite covering the most important behavior, including:

- block hashing;
- Proof of Work;
- blockchain validation;
- balance reconstruction;
- transaction validation;
- persistence and restart behavior.

Mining should support cancellation through Go's `context.Context`.

The CLI should handle interruption cleanly, allowing a user to cancel an ongoing mining operation without corrupting persisted state.

The repository should also contain a clear README explaining:

- the educational purpose of the project;
- architecture;
- blockchain flow;
- how to build and run the CLI;
- a complete example session;
- how Proof of Work and validation work;
- intentional simplifications and non-goals.

---

## 8. Non-Goals

The MVP will not implement:

- peer-to-peer networking;
- multiple blockchain nodes;
- distributed consensus;
- fork resolution;
- chain synchronization;
- peer discovery;
- gossip protocols;
- TCP networking;
- gRPC;
- cryptographic wallets;
- digital transaction signatures;
- UTXO transactions;
- Merkle Trees;
- smart contracts;
- scripting;
- Proof of Stake;
- automatic difficulty adjustment;
- transaction fees;
- block size economics;
- maximum currency supply;
- reward halving;
- parallel mining workers;
- Docker;
- Kubernetes;
- production observability;
- enterprise architecture patterns;
- unnecessary abstraction layers.

These features may be explored only after the core MVP is complete and understood.

---

## 9. AI-Assisted Development Requirement

GoChain will be implemented incrementally with an AI coding agent acting as a pair programmer.

For every implementation stage, the agent should explain before or alongside the code:

1. what is being implemented;
2. why the component exists;
3. which blockchain concept it demonstrates;
4. which files are being changed;
5. how the implementation works;
6. how to test the implementation;
7. which parts of the code deserve particular attention.

The agent should avoid generating large amounts of unexplained code.

Implementation plans should favor small, understandable milestones.

---

## 10. Completion Criteria

The MVP is considered complete when a user can:

1. initialize a fresh GoChain database;
2. create two accounts;
3. mine GoCoin for one account;
4. observe the mining reward in its confirmed balance;
5. create a transaction to another account;
6. observe that transaction in the pending transaction pool;
7. mine another block containing the transaction;
8. observe updated balances for both accounts;
9. inspect the resulting blockchain;
10. restart the application without losing blockchain or pending state;
11. validate the blockchain successfully;
12. detect blockchain corruption through validation;
13. cancel an ongoing mining operation safely;
14. run a focused automated test suite successfully;
15. follow the README to reproduce the complete blockchain flow;
16. understand the major components of the implementation and how they interact.

At that point, future features can be evaluated individually based on whether they add meaningful educational value.
