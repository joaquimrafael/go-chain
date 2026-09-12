# GoChain Incremental Implementation Plan

## Summary

GoChain will be built in 21 small milestones, following the dependency order in `PRD.md`, `SPEC.md`, and `AGENTS.md`. Each milestone ends with explanation, focused tests, `gofmt` for changed Go files, `go test ./...` when applicable, and a pause for review and commit.

Key defaults:

- Go version remains `1.26.4`.
- SQLite uses `database/sql` with the CGo-free [`modernc.org/sqlite` driver](https://pkg.go.dev/modernc.org/sqlite@v1.56.0), pinned initially to `v1.56.0`.
- The CLI uses the standard library, not a CLI framework.
- The default database is `gochain.db` in the working directory.
- Default difficulty is `3`; accepted difficulty is `1–64` and remains fixed from Genesis.
- Currency amounts use `int64`; nonces use `uint64`.
- Unix timestamps are injected into core constructors for deterministic tests; the CLI supplies the current time.
- Account names are trimmed, must be non-empty, and remain case-sensitive.
- No repository interfaces, service framework, balance table, schema migration framework, or future-feature abstractions will be introduced.

## Core Interfaces and Types

- `transaction.Type` with `transfer` and `reward`, plus `transaction.Transaction`.
- `blockchain.Block` and `blockchain.Blockchain`.
- Deterministic `Block.CalculateHash`.
- `blockchain.ValidateStructure` followed later by complete `blockchain.Validate`.
- Concrete `storage.Store` around `*sql.DB`; no storage interface.
- `storage.PendingTransaction` carries SQLite metadata separately from the blockchain transaction.
- `mining.ProofOfWork` and `mining.MineBlock`, both accepting `context.Context`.
- Public user interface remains the commands defined in `SPEC.md`.

## Milestones

### Milestone 1 — Buildable Project Foundation

- **Status:** ✅ Complete
- **Goal:** Establish the smallest executable Go application without implementing blockchain behavior.
- **What will be implemented:** A `cmd/gochain` entry point, a testable `run` dispatcher, help/usage output, unknown-command handling, and an ignore rule for `gochain.db` and local binaries.
- **Blockchain / Go concepts being learned:** Go module layout, `main` packages, argument handling, error-to-exit-code boundaries, and keeping executable wiring separate from domain code.
- **Expected files or packages involved:** `cmd/gochain`, `.gitignore`, and the existing `go.mod`.
- **Important implementation decisions:** Use only the standard library; do not create empty future packages or placeholder interfaces.
- **Tests to add or run:** Test help and unknown-command behavior; run `go test ./...` and `go build -o /tmp/gochain ./cmd/gochain`.
- **Completion criteria:** The binary builds, prints useful usage, rejects unknown commands clearly, and contains no blockchain logic.
- **What I should be able to explain after completing the milestone:** How a Go CLI starts, how arguments reach application code, and why a small `run` function improves testability.
- **Implementation notes:** Added `cmd/gochain/main.go` and `run.go`. `main` passes arguments and output streams to `run`, then forwards its return value to `os.Exit`. The dispatcher accepts `help`, `-h`, and `--help`; missing or unknown commands print usage to stderr and return exit code 1. Injected `io.Writer` values let tests check output without launching a process. `.gitignore` excludes the root binary and `gochain.db`. This milestone uses only the standard library; blockchain commands remain deferred.

### Milestone 2 — Transaction Values and Block Structure

- **Status:** ✅ Complete
- **Goal:** Define the data that participates in a blockchain block.
- **What will be implemented:** `transaction.Type`, `transaction.Transaction`, and `blockchain.Block` with the exact fields from `SPEC.md`.
- **Blockchain / Go concepts being learned:** Transactions as immutable values, ordered block contents, integer currency, named string types, structs, and package dependencies.
- **Expected files or packages involved:** `internal/transaction` and `internal/blockchain`.
- **Important implementation decisions:** Amounts are `int64`; nonce is `uint64`; transactions have no database ID; reward and transfer validation are deferred.
- **Tests to add or run:** Do not test plain struct field assignments; run compilation and `go test ./...`.
- **Completion criteria:** The exact block and transaction models compile without behavior or speculative fields.
- **What I should be able to explain after completing the milestone:** What every block and transaction field means and why transaction order must be preserved.
- **Implementation notes:** Added the named string type `transaction.Type`, its `Transfer` and `Reward` constants, and `transaction.Transaction` with `Type`, `From`, `To`, and `Amount`. Added `blockchain.Block` with height, timestamp, previous hash, stored hash, nonce, difficulty, and an ordered transaction slice. Amounts and heights use `int64`; nonces use `uint64`. The blockchain package depends on the transaction package. These are plain Go values without database IDs or validation behavior; their exported fields and slices do not enforce immutability.

### Milestone 3 — Deterministic SHA-256 Hashing

- **Status:** ✅ Complete
- **Goal:** Make block contents produce a reproducible cryptographic fingerprint.
- **What will be implemented:** `Block.CalculateHash` using SHA-256, lowercase hexadecimal output, and a dedicated JSON hash-payload struct.
- **Blockchain / Go concepts being learned:** Hash functions, deterministic serialization, byte slices, JSON encoding, and method receivers.
- **Expected files or packages involved:** `internal/blockchain`.
- **Important implementation decisions:** Hash input includes height, timestamp, previous hash, nonce, difficulty, and ordered transactions; stored `Hash` is excluded; maps are forbidden in the payload.
- **Tests to add or run:** Same data gives the same hash; changing nonce, previous hash, transaction data, or transaction order changes it; changing only stored `Hash` does not.
- **Completion criteria:** Hashing is deterministic and every specified field affects the result correctly.
- **What I should be able to explain after completing the milestone:** Why deterministic serialization matters and why a block cannot hash its own stored hash.
- **Implementation notes:** Added the value-receiver method `Block.CalculateHash` and an unexported `hashPayload` struct in `internal/blockchain/block.go`. The method serializes height, timestamp, previous hash, nonce, difficulty, and ordered transactions with `encoding/json`, hashes the bytes with `sha256.Sum256`, and returns lowercase hexadecimal using `hex.EncodeToString`. The stored `Hash` is excluded, and calculating a hash does not update the block. Tests check a deterministic SHA-256 result, changes to every payload field and transaction order, and independence from the stored hash.

### Milestone 4 — Genesis Block

- **Status:** ✅ Complete
- **Goal:** Create the deterministic starting point of every GoChain.
- **What will be implemented:** Genesis constants and `NewGenesisBlock(timestamp, difficulty)`.
- **Blockchain / Go concepts being learned:** Chain roots, protocol constants, constructors, validation of input ranges, and deterministic fixtures.
- **Expected files or packages involved:** `internal/blockchain`.
- **Important implementation decisions:** Height is `0`, previous hash is 64 zeroes, transactions are empty, no currency is created, and Genesis is hashed normally without Proof of Work.
- **Tests to add or run:** Verify all Genesis fields, empty transactions, deterministic hash, and difficulty bounds.
- **Completion criteria:** Given the same timestamp and difficulty, Genesis is reproduced exactly.
- **What I should be able to explain after completing the milestone:** Why Genesis is special and which normal block rules do and do not apply to it.
- **Implementation notes:** Added Genesis and difficulty constants plus `NewGenesisBlock(timestamp, difficulty)` in `internal/blockchain/genesis.go`. The constructor rejects difficulty outside 1–64, uses height and nonce 0, sets the previous hash to 64 zeroes, allocates a non-nil empty transaction slice, and calculates the stored hash without mining. The supplied timestamp makes construction deterministic. Genesis issues no reward. Tests check its fields, a fixed expected hash, repeatability, and both valid and invalid difficulty boundaries.

### Milestone 5 — In-Memory Blockchain Structure

- **Status:** ✅ Complete
- **Goal:** Represent an ordered chain and expose its tip safely.
- **What will be implemented:** `blockchain.Blockchain`, construction from Genesis, tip access, next height, and fixed chain difficulty lookup.
- **Blockchain / Go concepts being learned:** Ordered history, slices, value versus pointer semantics, and explicit error handling for an empty chain.
- **Expected files or packages involved:** `internal/blockchain`.
- **Important implementation decisions:** The complete chain remains in memory during one command; SQLite will later be the persistent source; no caching or generic chain repository is added.
- **Tests to add or run:** New chain contains Genesis; tip and next height are correct; empty-chain access returns a clear error.
- **Completion criteria:** Later hashing, validation, balance, and mining code can consume one simple ordered structure.
- **What I should be able to explain after completing the milestone:** How block order, height, previous hash, and the current tip relate.
- **Implementation notes:** Added `Blockchain` with a `Blocks []Block` field and `NewBlockchain(genesis)` in `internal/blockchain/chain.go`. `Tip` reads the final block, `NextHeight` adds one to the tip's height, and `Difficulty` reads Genesis so the chain's configured difficulty does not depend on later blocks. All three accessors return `ErrEmptyBlockchain` when the slice is empty. Construction and access do not validate the chain or deep-copy transaction slices; callers retain responsibility for validation and mutation. Tests cover Genesis construction, tip and height lookup, Genesis-based difficulty, and empty-chain errors.

### Milestone 6 — Structural Chain Validation

- **Status:** ✅ Complete
- **Goal:** Detect corruption in the cryptographic and linking structure before economic rules exist.
- **What will be implemented:** `ValidateStructure`, replaying from Genesis and returning the first block-specific error.
- **Blockchain / Go concepts being learned:** Invariants, sequential validation, fail-fast errors, hash recalculation, and leading-zero checks.
- **Expected files or packages involved:** `internal/blockchain`.
- **Important implementation decisions:** Check Genesis shape, hash equality, sequential height, previous-hash links, fixed difficulty, and Proof-of-Work prefix for non-Genesis blocks. Economic and reward validation is explicitly deferred to Milestone 13.
- **Tests to add or run:** Valid structure; malformed Genesis; altered height, link, hash, nonce, or difficulty; error identifies the first invalid block and rule.
- **Completion criteria:** Every structural invariant available at this stage is enforced with readable errors.
- **What I should be able to explain after completing the milestone:** How tampering propagates through hashes and links, and why validation starts at Genesis.
- **Implementation notes:** Added `ValidateStructure(chain)` in `internal/blockchain/validation.go`. It walks blocks from Genesis, checking sequential heights, difficulty bounds and agreement with Genesis, the Genesis previous hash and empty transactions, later block links, recalculated hashes, and non-Genesis leading-zero Proof of Work. Errors identify the slice position rather than trusting a possibly corrupted stored height, and validation stops at the first failure. Tests use deterministic difficulty-1 fixtures and cover corruption and error ordering. The fixture nonce search is test setup only; cancellable mining and economic validation remain deferred to Milestones 12 and 13.

### Milestone 7 — SQLite Schema, Initialization, and Chain Reload

- **Status:** ✅ Complete
- **Goal:** Persist Genesis and reconstruct the same chain after process restart.
- **What will be implemented:** Concrete `storage.Store`, schema creation, initialization, Genesis persistence, full chain loading, and connection closing.
- **Blockchain / Go concepts being learned:** `database/sql`, SQLite transactions, relational representation of ordered blockchain data, resource cleanup, and restart persistence.
- **Expected files or packages involved:** `internal/storage`, `go.mod`, and `go.sum`.
- **Important implementation decisions:** Create the four specified tables; preserve transaction order with `position`; initialize schema and Genesis consistently; reject a second initialization; load blocks by height and transactions by position.
- **Tests to add or run:** Fresh initialization, persisted Genesis, second-init rejection, close/reopen behavior, and chain reconstruction from a temporary database.
- **Completion criteria:** A fresh database initializes once and reloads an identical Genesis chain after reopening.
- **What I should be able to explain after completing the milestone:** How blocks and ordered transactions map to relational rows and why SQLite is the persistent source of truth.
- **Implementation notes:** Added `storage.Open(ctx, path)`, `Store.Close`, `Store.Initialize(ctx, timestamp, difficulty)`, and `Store.LoadChain(ctx)`. Schema creation and Genesis insertion commit together; chain loading uses one SQL snapshot and preserves stored hashes, block heights, transaction positions, and empty Genesis transactions. Nonces use decimal `TEXT` to preserve the complete `uint64` range. Pending IDs use `AUTOINCREMENT` so a deleted ID cannot later identify a different pending transfer. No scope deviations; account operations, pending operations, mined-block confirmation, and CLI wiring remain deferred.
- **Verification:** `gofmt -w internal/storage/sqlite.go internal/storage/blocks.go internal/storage/sqlite_test.go`, `go test ./internal/storage`, `go test ./...`, `go vet ./...`, and `git diff --check` passed. `go get modernc.org/sqlite@v1.56.0` and `go mod tidy` resolved the pinned driver. Dependency tidying and full checks required build-cache access beyond the sandbox. Tests cover fresh initialization, all four tables, identical Genesis after reopen, repeated-init rejection, invalid difficulty, cancelled initialization, schema rollback on Genesis failure, ordered multi-block reload, maximum nonce, corrupt-hash preservation, and malformed nonce errors.

### Milestone 8 — Persistent Accounts

- **Status:** ✅ Complete
- **Goal:** Introduce the human-readable identities used by transactions and rewards.
- **What will be implemented:** Account-name normalization, creation, uniqueness enforcement, existence checks, and loading the current account-name set.
- **Blockchain / Go concepts being learned:** Domain validation, unique constraints, parameterized SQL, sentinel errors, and simple set representation in Go.
- **Expected files or packages involved:** `internal/storage`.
- **Important implementation decisions:** Trim surrounding whitespace, reject empty names, preserve case sensitivity, and provide no deletion, keys, passwords, addresses, or balances column.
- **Tests to add or run:** Create/reload accounts; reject blank and duplicate names; confirm `alice` and `Alice` are distinct.
- **Completion criteria:** Accounts survive restart and can be queried reliably by later transaction and validation code.
- **What I should be able to explain after completing the milestone:** Why accounts are identifiers only and why their balances are not stored alongside them.
- **Implementation notes:** Added `NormalizeAccountName`, `Store.CreateAccount(ctx, name, timestamp)`, `Store.AccountExists(ctx, name)`, and `Store.LoadAccountNames(ctx)` in `internal/storage/accounts.go`. Creation returns the normalized name; creation and lookup reject blank names and preserve case and internal whitespace. Parameterized SQL and the existing primary key enforce uniqueness without a separate existence check. Duplicate creation returns `ErrAccountAlreadyExists` without overwriting the original timestamp; blank input returns `ErrInvalidAccountName`. The name set uses `map[string]struct{}`. Store operations require an initialized database. No schema changes, new dependencies, or scope deviations; CLI wiring and balances remain deferred.
- **Verification:** `gofmt -w internal/storage/accounts.go internal/storage/accounts_test.go`, `go test ./internal/storage -run TestAccounts -count=1`, `go test ./...`, `go vet ./...`, and `git diff --check` passed. Full tests and vet required build-cache access beyond the sandbox. Tests cover empty sets, normalization including Unicode whitespace, distinct case-sensitive names, names containing quotes, close/reopen persistence, existence and missing-name checks, duplicate rejection without timestamp replacement, blank-name rejection, and context cancellation without account creation.

### Milestone 9 — Confirmed Balance Reconstruction

- **Status:** ✅ Complete
- **Goal:** Derive balances solely from confirmed blockchain history.
- **What will be implemented:** Ordered transaction replay and per-account confirmed balance lookup.
- **Blockchain / Go concepts being learned:** Event replay, derived state, map accumulation, transaction ordering, and error propagation.
- **Expected files or packages involved:** `internal/blockchain`.
- **Important implementation decisions:** Reward credits the receiver; transfer debits sender and credits receiver; unknown types and negative replay states return errors; zero-balance accounts need no stored map entry.
- **Tests to add or run:** Reward credit, sender debit, receiver credit, multiple blocks, multiple transactions, unknown type, and negative sender balance.
- **Completion criteria:** Synthetic valid chains reproduce the expected balances without a balances table.
- **What I should be able to explain after completing the milestone:** Why blockchain history is authoritative and how replay rebuilds state after restart.
- **Implementation notes:** Added `Blockchain.Balances()` and `Blockchain.Balance(name)` in `internal/blockchain/balance.go`. Replay walks block and transaction slices in order, credits rewards, and debits senders before crediting transfer receivers. Each call builds a fresh `map[string]int64`; no balances are persisted or cached. Names are matched exactly, and absent names read as zero without proving account existence. Empty chains return `ErrEmptyBlockchain`; Genesis alone produces an empty map. Replay errors identify the zero-based block and transaction positions and return no partial balances. Arithmetic safeguards reject nonpositive amounts, insufficient sender funds, and receiver overflow beyond `int64`, alongside unknown transaction types. Hash checks, account existence, reward policy, and other complete validation rules remain separate and deferred; no schema changes or new dependencies were needed.
- **Verification:** `gofmt -w internal/blockchain/balance.go internal/blockchain/balance_test.go`, `go test ./internal/blockchain -run 'TestBalances' -count=1`, `go test ./...`, `go vet ./...`, and `git diff --check` passed. Full tests and vet required build-cache access beyond the sandbox. Tests cover Genesis and empty chains, the specification's Alice/Bob example, exact-name lookup, independent replay maps, multiple blocks and transactions, exact spending, transaction order, unknown types, nonpositive amounts, unfunded senders, overspending, first-error reporting, lookup error propagation, and reward/transfer arithmetic at the `int64` limit. Fixtures deliberately omit mining and complete reward validation to isolate replay behavior.

### Milestone 10 — Transfer and Available-Funds Validation

- **Status:** ✅ Complete
- **Goal:** Decide whether a new transfer may enter the pending pool.
- **What will be implemented:** Transfer validation using account names, confirmed balances, and the existing ordered pending transfers.
- **Blockchain / Go concepts being learned:** Separating confirmed from available state, pure validation functions, table-driven tests, and explicit business rules.
- **Expected files or packages involved:** `internal/transaction`.
- **Important implementation decisions:** Require existing and different accounts, positive amount, and sufficient confirmed balance after subtracting all pending outgoing transfers. Pending incoming transfers and future rewards are not spendable.
- **Tests to add or run:** Missing sender/receiver, same account, zero/negative amount, exact spend, overspend, cumulative pending overspend, and attempted spending of pending incoming funds.
- **Completion criteria:** Every transfer rule in `SPEC.md` is enforced without accessing SQLite directly.
- **What I should be able to explain after completing the milestone:** The difference between confirmed balance and currently available funds.
- **Implementation notes:** Added the pure `transaction.ValidateTransfer(transfer, accounts, confirmed, pending)` function in `internal/transaction/validation.go`. It requires a transfer type, existing and different sender/receiver names, a positive amount, and sufficient available funds. Names are matched exactly and must already be normalized by the caller. Available funds start at the sender's confirmed balance and decrease for each pending outgoing transfer; pending incoming funds and future rewards never increase them. Checking before each subtraction prevents cumulative pending amounts from overflowing `int64`. The function rejects negative sender balances and malformed pending transfer fields, reporting zero-based pending positions; pending funding is checked only for the candidate's sender. It neither mutates inputs nor accesses SQLite. Pending storage, CLI wiring, and complete chain validation remain deferred; no dependencies or schema changes were needed.
- **Verification:** `gofmt -w internal/transaction/validation.go internal/transaction/validation_test.go`, `go test ./internal/transaction -count=1`, `go test ./...`, `go vet ./...`, and `git diff --check` passed. Full tests and vet required build-cache access beyond the sandbox. Table-driven tests cover valid and exact spending, missing accounts, case-sensitive names, same-account transfers, zero/negative amounts, unknown/reward types, unfunded senders, cumulative pending spending, pending incoming funds, unrelated outgoing transfers, already-overspent pending funds, malformed pending entries, negative confirmed balances, `int64` limits, and unchanged inputs.

### Milestone 11 — Persistent Pending Transaction Pool

- **Status:** ✅ Complete
- **Goal:** Make unconfirmed transfers survive separate CLI executions.
- **What will be implemented:** `storage.PendingTransaction`, insertion, ordered loading, and restart persistence.
- **Blockchain / Go concepts being learned:** The mempool concept, persistence metadata, database-generated IDs, and conversion between storage and domain values.
- **Expected files or packages involved:** `internal/storage`.
- **Important implementation decisions:** Pending rows represent transfers only; IDs and `created_at` remain storage metadata and never enter block hashing; loading order is ascending database ID.
- **Tests to add or run:** Insert/load, stable order, close/reopen persistence, and rejection of attempts to store reward transactions as pending.
- **Completion criteria:** Pending transfers reload unchanged and in a deterministic order.
- **What I should be able to explain after completing the milestone:** Why a CLI blockchain needs a persistent mempool and why its database ID is not blockchain data.
- **Implementation notes:** Added `storage.PendingTransaction`, `Store.AddPending(ctx, transfer, timestamp)`, and `Store.LoadPending(ctx)` in `internal/storage/pending.go`. Each entry carries a generated `int64` ID, a Unix `CreatedAt` timestamp, and a separate `transaction.Transaction` value. Parameterized insertion preserves transaction fields and returns the assigned ID; loading uses ascending ID order and restores the transfer type implicit in the pending table. Identical transfers remain distinct rows, and timestamps do not determine queue order. Empty pools return a non-nil empty slice. Insertion rejects rewards and unknown types; callers must use `transaction.ValidateTransfer` for account, amount, and available-funds checks before insertion. Storage metadata stays outside block transaction data and hashing. No schema changes or dependencies were needed; mining, pending removal during atomic confirmation, and CLI wiring remain deferred.
- **Verification:** `gofmt -w internal/storage/pending.go internal/storage/pending_test.go`, `go test ./internal/storage -run TestPending -count=1`, `go test ./...`, `go vet ./...`, and `git diff --check` passed. Full tests and vet ran with build-cache access beyond the sandbox. Tests cover empty pools, insertion results, generated IDs, stable ordering despite out-of-order timestamps, names containing quotes, duplicate transfer values with distinct IDs, close/reopen persistence, further insertion after reopening, unchanged confirmed history, non-transfer rejection, pre-cancelled operations, and forced SQLite insertion failure preserving existing pending rows.

### Milestone 12 — Proof of Work with Context Cancellation

- **Status:** ✅ Complete
- **Goal:** Find a nonce whose block hash satisfies the leading-zero difficulty.
- **What will be implemented:** Difficulty checking and `ProofOfWork(ctx, block)`, updating the candidate nonce and hash only on success.
- **Blockchain / Go concepts being learned:** Brute-force Proof of Work, loops, mutable candidates, `context.Context`, cancellation, and cooperative concurrency.
- **Expected files or packages involved:** `internal/mining`.
- **Important implementation decisions:** Start at nonce `0`, hash sequentially, check cancellation every iteration, use no workers or optimizations, and return without a successful hash when cancelled.
- **Tests to add or run:** Low-difficulty success, stored hash/nonce consistency, leading-zero satisfaction, deterministic start behavior, invalid difficulty, and prompt cancellation at very high difficulty.
- **Completion criteria:** Low-difficulty mining completes quickly and cancellation cannot return a successful candidate.
- **What I should be able to explain after completing the milestone:** Why nonce changes alter the hash and how context cancellation safely stops CPU-bound work.
- **Implementation notes:** Added `mining.ProofOfWork(ctx, block) (blockchain.Block, error)` in `internal/mining/pow.go`. The function checks the existing 1–64 difficulty bounds, searches sequentially from nonce zero using `Block.CalculateHash`, and returns the first hash with the required hexadecimal zero prefix. It operates on a block value, leaves the input unchanged, and returns a zero block on failure. Context errors are checked every iteration and again before returning a successful hash. A nonce-range exhaustion guard prevents unsigned wraparound. Transaction slices are read without copying or mutation; callers must not mutate them concurrently. This step performs only Proof of Work, with full block validation, reward creation, persistence, and CLI coordination remaining in their later milestones. No dependencies or architecture changes were needed.
- **Verification:** `gofmt -w internal/mining/pow.go internal/mining/pow_test.go`, `go test ./internal/mining -count=1`, `go test ./...`, `go vet ./...`, `go test -race ./internal/mining -count=1`, and `git diff --check` passed. The initial sandboxed race build failed with `runtime/race: package testmain: cannot find package`; rerunning with access beyond the sandbox passed. Tests cover fixed first-winning nonces and hashes at difficulty 1 and 2, nonce-zero success, restarting from zero despite a supplied nonce, repeatability, unchanged block contents and input, invalid difficulty, pre-cancellation, and prompt deadline cancellation at maximum difficulty with no returned candidate.

### Milestone 13 — Complete Blockchain Validation

- **Status:** ✅ Complete
- **Goal:** Extend structural validation to all transaction, reward, account, and balance invariants.
- **What will be implemented:** Complete `blockchain.Validate(chain, accountNames)`, composing structural checks with ordered economic replay.
- **Blockchain / Go concepts being learned:** Stateful validation, protocol invariants, composition of checks, and precise diagnostic errors.
- **Expected files or packages involved:** `internal/blockchain` and transaction domain types.
- **Important implementation decisions:** Genesis has no transactions; every non-Genesis block has exactly one first-position reward of 50 GOC from the empty sender; all transaction types and referenced accounts are valid; transfers use different accounts and positive amounts; replay never makes a sender negative.
- **Tests to add or run:** Valid chain; modified transaction; invalid link/nonce/hash; missing, duplicate, misplaced, or wrong reward; unknown account/type; invalid amount; same-account transfer; and historical overspending.
- **Completion criteria:** Validation reports the first invalid block and rule for every invariant in `SPEC.md`.
- **What I should be able to explain after completing the milestone:** How structural validity differs from economic validity and why validation must replay transactions in order.
- **Implementation notes:** Added `blockchain.Validate(chain, accountNames)`, checking structure and economic rules together in chain order. Extracted the existing per-block structural checks so an earlier economic failure takes precedence over a later structural failure; `ValidateStructure` retains its existing behavior. Non-Genesis blocks require a first-position reward, reject additional rewards, and enforce the empty sender, registered receiver, and fixed `transaction.RewardAmount` of 50 GOC. Transfers reuse `transaction.ValidateTransfer` without a pending pool, and each confirmed transaction updates local replay balances through the existing overflow-checked balance function. Replay uses funds confirmed up to the current transaction position; pending admission and future candidate construction still use their separate pre-mining funds rules. Inputs are unchanged, account names are matched exactly against the supplied registry, and errors identify the block and transaction position where applicable. No dependencies, persistence changes, or scope deviations were needed.
- **Verification:** `gofmt -w internal/blockchain/validation.go internal/blockchain/validation_test.go internal/blockchain/validation_economic_test.go internal/transaction/transaction.go`, `go test ./internal/blockchain -run TestValidate -count=1`, `go test ./...`, `go vet ./...`, and `git diff --check` passed. The initial focused test was blocked by sandbox access to the Go build cache; rerunning with cache access passed, as did the full suite and vet. Existing structural corruption cases now exercise both validators. New tests cover empty and Genesis-only chains, reward-only blocks, ordered multi-block replay, unchanged inputs, missing/duplicate/misplaced rewards, reward sender and amount, missing accounts, unknown types, invalid transfers, cumulative and historical overspending, and first-error ordering across structural and economic rules. Economic fixtures are re-mined at difficulty 1 to isolate their rule failures from hash failures. Mining orchestration remains deferred to Milestone 14.

### Milestone 14 — Candidate Blocks, Rewards, and Pure Block Mining

- **Status:** ✅ Complete
- **Goal:** Turn a validated chain and pending snapshot into one valid mined block without persistence.
- **What will be implemented:** Candidate construction and `MineBlock(ctx, chain, pending, miner, accounts, timestamp)`.
- **Blockchain / Go concepts being learned:** Mining orchestration, reward issuance, candidate state, ordered transactions, and pure core logic separated from I/O.
- **Expected files or packages involved:** `internal/mining`.
- **Important implementation decisions:** Validate the existing chain and miner; revalidate pending transfers against pre-mining confirmed funds; place reward first; preserve pending order; derive height, previous hash, and fixed difficulty from the tip; run Proof of Work; validate the appended result before returning it.
- **Tests to add or run:** Reward-only mining, mining with ordered transfers, reward amount/order, unknown miner, invalid pending transfer, fixed difficulty, valid resulting chain, and cancellation.
- **Completion criteria:** The function returns a fully valid block or an error and never touches SQLite.
- **What I should be able to explain after completing the milestone:** Every step from pending snapshot to a valid mined block and why the reward is not pending.
- **Implementation notes:** Added `mining.MineBlock(ctx, chain, pending, miner, accounts, timestamp)` in `internal/mining/mine.go`. It validates the existing chain and exact miner name, replays confirmed balances, and revalidates each pending transfer against the earlier pending prefix. This reserves cumulative outgoing spending without crediting pending incoming transfers or the new reward. The candidate owns a fresh transaction slice containing the 50 GOC reward first and all pending transfers in snapshot order; its height, previous hash, and fixed difficulty follow the validated tip. After Proof of Work, complete validation checks a separately allocated appended chain before the block is returned. Errors, including cancellation, return a zero block and preserve context error identity. Inputs remain unchanged. Repeated replay and prefix validation favor reuse and clarity for the small educational dataset over performance. No dependencies or architecture changes were needed; SQLite confirmation and CLI wiring remain deferred.
- **Verification:** `gofmt -w internal/mining/mine.go internal/mining/mine_test.go`, `go test ./internal/mining -run TestMineBlock -count=1`, `go test ./...`, `go vet ./...`, and `git diff --check` passed. The initial full-suite attempt was blocked by sandbox access to the Go build cache; the full suite and vet passed when rerun with cache access. Tests cover reward-only mining, deterministic results, fixed difficulty and tip linkage, ordered transfers and exact spending, resulting chain validation and balances, unchanged input and backing-array ownership, empty/corrupted history, unknown miner, invalid pending fields and types, cumulative overspending, unspendable rewards and pending incoming funds, pre-cancellation, and deadline cancellation during high-difficulty mining.

### Milestone 15 — Atomic Persistence of Mined Blocks

- **Goal:** Confirm a mined block and remove exactly its included pending rows as one database operation.
- **What will be implemented:** `Store.ConfirmMinedBlock(ctx, block, includedPendingIDs)`.
- **Blockchain / Go concepts being learned:** ACID transactions, optimistic tip checks, commit/rollback, affected-row validation, and the distinction between blockchain transactions and SQL transactions.
- **Expected files or packages involved:** `internal/storage`.
- **Important implementation decisions:** Within one SQLite transaction, recheck expected tip height/hash, insert the block, insert ordered confirmed transactions, delete exactly the captured pending IDs, verify delete counts, then commit. Newer unincluded pending rows remain.
- **Tests to add or run:** Successful confirmation and reload; included rows removed; unincluded rows retained; reward-only block; stale-tip rejection; missing pending ID forces rollback; failed confirmation leaves block and pending state unchanged.
- **Completion criteria:** No observable state can contain half of a mining confirmation.
- **What I should be able to explain after completing the milestone:** Why SQL atomicity is required even though the blockchain itself also contains objects called transactions.

### Milestone 16 — CLI Initialization, Account Creation, and Balance

- **Goal:** Expose the first usable vertical slice through the CLI.
- **What will be implemented:** `init [--difficulty N]`, `create-account <name>`, and `balance <name>`.
- **Blockchain / Go concepts being learned:** Standard-library flag parsing, dependency wiring, filesystem-backed state, and replay through an application boundary.
- **Expected files or packages involved:** `cmd/gochain` and existing storage/blockchain packages.
- **Important implementation decisions:** Main uses `gochain.db`; tests call the runner with an injected temporary path; CLI supplies Unix timestamps; errors remain concise and educational.
- **Tests to add or run:** Initialization/default difficulty, explicit difficulty, repeated init, account creation errors, zero balance, and reward-based balance fixtures.
- **Completion criteria:** A user can initialize a database, create accounts, restart, and query confirmed balances.
- **What I should be able to explain after completing the milestone:** How one CLI command opens state, performs an operation, prints output, and exits.

### Milestone 17 — CLI Send and Pending Commands

- **Goal:** Let users create valid transfers and inspect the persistent pending pool.
- **What will be implemented:** `send --from --to --amount` and `pending`.
- **Blockchain / Go concepts being learned:** Coordinating multiple data sources, parsing integer flags, application-level validation, and readable domain output.
- **Expected files or packages involved:** `cmd/gochain`.
- **Important implementation decisions:** `send` loads accounts, full chain, balances, and pending transfers before validation; only validated transfers are inserted; `pending` preserves storage order; amounts remain whole GOC.
- **Tests to add or run:** Successful send, all invalid transfer rules, cumulative pending overspend, pending incoming not spendable, readable empty/list output, and restart persistence.
- **Completion criteria:** Valid transfers persist as pending and invalid requests leave the database unchanged.
- **What I should be able to explain after completing the milestone:** The full path from a CLI transfer request to a durable pending transaction.

### Milestone 18 — CLI Mining and Safe Cancellation

- **Goal:** Complete the pending-to-confirmed flow without risking partial persistence.
- **What will be implemented:** `mine --miner <name>`, signal-derived context cancellation, one mining goroutine, a small result channel, and post-success atomic confirmation.
- **Blockchain / Go concepts being learned:** Goroutines, channels, OS signals, context propagation, snapshots, and commit-after-computation.
- **Expected files or packages involved:** `cmd/gochain`, reusing mining and storage.
- **Important implementation decisions:** Snapshot pending IDs and transfers before mining; no SQL transaction remains open during Proof of Work; persist only after a successful validated result; wait for the mining goroutine to stop after cancellation; reward exists only inside the committed block.
- **Tests to add or run:** Reward-only mining, mining pending transfers, balance changes, pending removal, invalid miner, cancelled context, unchanged tip/balances/pending state after cancellation, and restart after success.
- **Completion criteria:** Successful mining commits one block atomically; cancellation commits nothing and preserves every pending transfer.
- **What I should be able to explain after completing the milestone:** Why mining happens outside the SQL transaction and why cancellation before confirmation is safe.

### Milestone 19 — CLI Chain Inspection and Validation

- **Goal:** Make blockchain history and integrity visible to the learner.
- **What will be implemented:** `chain` and `validate` with readable block, hash, nonce, difficulty, and transaction output.
- **Blockchain / Go concepts being learned:** Observability through domain-oriented output, validation as replay, and corruption diagnosis.
- **Expected files or packages involved:** `cmd/gochain`.
- **Important implementation decisions:** Output is human-oriented; reward displays as `REWARD`; validation reports success or the first block-specific error; no JSON/API output mode is added.
- **Tests to add or run:** Empty/uninitialized errors, readable multi-block output, valid-chain success, and precise output for hash, link, reward, and transaction corruption.
- **Completion criteria:** Users can inspect the complete chain and clearly understand why a corrupted chain fails.
- **What I should be able to explain after completing the milestone:** How stored blockchain data is independently recalculated and checked rather than trusted.

### Milestone 20 — Full Integration and Restart Scenarios

- **Goal:** Verify the complete product journey across realistic command boundaries.
- **What will be implemented:** Focused end-to-end tests using temporary SQLite databases and repeated application runner invocations.
- **Blockchain / Go concepts being learned:** Integration testing, stateful workflows, restart safety, failure assertions, and testing system invariants rather than individual functions.
- **Expected files or packages involved:** Integration tests near `cmd/gochain` or in a small top-level integration-test package.
- **Important implementation decisions:** Use low test difficulty; never depend on the developer’s real `gochain.db`; directly corrupt a temporary database only for validation tests.
- **Tests to add or run:** Init → create Alice/Bob → mine Alice → send to Bob → inspect pending → restart → mine → verify balances → inspect chain → validate; also atomic failure, cancellation safety, and deliberate corruption detection.
- **Completion criteria:** The entire PRD completion journey passes repeatedly with temporary state and `go test ./...`.
- **What I should be able to explain after completing the milestone:** How all packages cooperate across separate CLI processes and which guarantees are enforced at each boundary.

### Milestone 21 — README and Final Educational Demo

- **Goal:** Make the finished MVP understandable and reproducible without reading the implementation first.
- **What will be implemented:** Complete README covering purpose, architecture, flow, setup, build/run instructions, command reference, full example, hashing, Proof of Work, balances, validation, tests, simplifications, and non-goals.
- **Blockchain / Go concepts being learned:** Technical communication, architecture explanation, reproducible demonstrations, and documenting intentional simplifications.
- **Expected files or packages involved:** `README.md`; `IMPLEMENTATION_PLAN.md` remains the historical implementation guide.
- **Important implementation decisions:** Clearly state that GoChain is educational, single-node, unsigned, account-based, fixed-reward, fixed-difficulty, and not production cryptocurrency software; do not present out-of-scope features as planned work.
- **Tests to add or run:** Run every documented command in a temporary directory, `go test ./...`, and `go build -o /tmp/gochain ./cmd/gochain`; verify README output examples match actual behavior.
- **Completion criteria:** A new reader can reproduce the complete PRD journey and explain the system’s major components and simplifications.
- **What I should be able to explain after completing the milestone:** The complete transaction → pending → mining → block → validation → balance lifecycle in my own words.

## Dependency Sequence

Primary implementation path:

```text
1 Project foundation
  ↓
2 Transaction and block values
  ↓
3 Deterministic hashing
  ↓
4 Genesis
  ↓
5 Blockchain container
  ↓
6 Structural validation
  ↓
7 SQLite initialization and reload
  ↓
8 Accounts
  ↓
9 Balance reconstruction
  ↓
10 Transfer validation
  ↓
11 Pending persistence
  ↓
12 Proof of Work
  ↓
13 Complete validation
  ↓
14 Candidate construction and mining
  ↓
15 Atomic confirmation
  ↓
16 CLI init/accounts/balance
  ↓
17 CLI send/pending
  ↓
18 CLI mining/cancellation
  ↓
19 CLI chain/validate
  ↓
20 Full integration tests
  ↓
21 README and final demo
```

Important cross-dependencies:

- SQLite chain loading depends on the finalized transaction, block, Genesis, and blockchain models.
- Transfer validation depends on balance replay and account lookup.
- Mining depends on hashing, Proof of Work, pending validation, rewards, and complete chain validation.
- Atomic confirmation depends on stable pending IDs and a successfully validated mined block.
- CLI mining depends on both pure mining and atomic storage; it must not combine those phases prematurely.
- Final integration and documentation depend on every user-facing command and persistence guarantee.

## Recommended Next Implementation Task

Implement **Milestone 12 — Proof of Work with Context Cancellation only**.

Before writing its code:

1. Explain Milestone 12’s purpose, Go concepts, and expected file changes.
2. Implement only difficulty checking and sequential Proof of Work with context cancellation; update the candidate nonce and hash only on success.
3. Run `gofmt`, focused Proof-of-Work and cancellation tests, and `go test ./...`.
4. Explain the resulting code and stop for review and commit.
