# AGENTS.md

## Project purpose and authority

GoChain is a small, single-node educational blockchain written in Go. It exists to teach blockchain fundamentals and idiomatic Go; it is not a production blockchain or cryptocurrency.

- Treat `PRD.md` as the product scope.
- Treat `SPEC.md` as the technical source of truth.
- Read the relevant sections of both documents before planning or changing code.
- If a request conflicts with either document, identify the conflict and explain the proposed scope or architecture change before coding.
- Do not add features outside `PRD.md` and `SPEC.md` unless the user explicitly requests them.

## Design principles

- Prefer simplicity, readability, and educational value over production realism, performance, or extensibility.
- Use idiomatic, understandable Go and the standard library when reasonable.
- Avoid unnecessary abstractions, interfaces, layers, frameworks, packages, indirection, and speculative extension points.
- Keep data flow explicit. Combine files or responsibilities when further separation would make the project harder to understand.
- Make every implementation step small, independently understandable, and easy to review.
- Do not silently generate or change large amounts of code. If a milestone is too large, split it and agree on the smaller boundary first.
- Act as a pair programmer: explain concepts and decisions while implementing, and point the developer to code worth studying.
- Do not change the agreed architecture or scope without first explaining why the change is needed and what it affects.

## Required milestone workflow

Before writing code for every implementation milestone, explain:

1. what will be implemented;
2. why it exists;
3. which blockchain or Go concept it teaches;
4. which files are expected to change.

Then implement only that milestone. Do not automatically continue into the next conceptual stage.

After implementation:

1. explain how the implementation works;
2. highlight the most important code to study;
3. run the relevant tests;
4. run `gofmt` on every changed Go file;
5. report the commands run and their results;
6. mention important trade-offs, simplifications, or intentionally deferred work;
7. stop and wait for the next requested milestone.

Prefer focused tests while developing. Run `go test ./...` when the repository has a Go module and the milestone is integrated enough for the full suite. Tests should emphasize important behavior and readable examples, not a coverage percentage.

## GoChain model and invariants

- Accounts use unique, human-readable names. The MVP has no keys, signatures, addresses, or passwords.
- Currency amounts are whole GOC values represented with an integer type such as `int64`; never use floating point.
- Balances are derived by replaying confirmed transactions in blockchain order. Do not create an authoritative balances table.
- Pending incoming transactions are not spendable. Pending outgoing transfers reduce the amount available for further pending spending.
- Pending transactions are persisted in SQLite so they survive separate CLI executions.
- SQLite is the only persistence mechanism. Persist a mined block, its confirmed transactions, and removal of included pending transactions atomically.
- Block hashing must be deterministic, use SHA-256, preserve transaction order, and exclude the stored hash from its own input.
- Proof of Work is intentionally a readable, low-difficulty leading-zero nonce loop, not a performance exercise.
- Mining cancellation must use `context.Context`. Cancellation or failure must not persist a block or reward and must leave pending transactions intact.
- Blockchain validation should replay from Genesis, enforce the invariants in `SPEC.md`, and return clear errors identifying the first invalid block or rule.

## Scope boundaries

Unless explicitly requested, do not add:

- P2P networking, multiple nodes, distributed consensus, forks, or chain synchronization;
- wallets, public/private keys, signatures, cryptographic addresses, or UTXO accounting;
- Merkle Trees, smart contracts, scripting, or Proof of Stake;
- automatic difficulty adjustment, fees, halving, supply caps, or parallel mining workers;
- Docker, Kubernetes, production observability, gRPC, enterprise architecture, or similar infrastructure.

Do not introduce these ideas indirectly through premature interfaces or architecture intended for hypothetical future features.

## Change and testing discipline

- Preserve deterministic behavior and the invariants documented in `SPEC.md`.
- Prefer behavior-focused unit tests for hashing, Proof of Work, cancellation, validation, balances, and transaction rules.
- Prefer focused integration tests for SQLite persistence, restart behavior, atomic mining confirmation, and cancellation safety.
- Keep tests easy to read and use low Proof-of-Work difficulty so they remain fast.
- Do not alter unrelated files or broaden a milestone while fixing an issue.
- Documentation should clearly call out intentional simplifications so readers do not mistake them for production blockchain practices.
