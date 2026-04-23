# MP3 Design Document — Distributed Transactions

## Overview

Two-Phase Locking (2PL) for isolation + Two-Phase Commit (2PC) for atomicity.
These solve different problems: 2PL controls concurrent access during a transaction,
2PC ensures all-or-nothing commit across servers.

---

## Server Data Structures

### Per-account lock state (on each branch server)
```
accounts: map[string]Account
  Account:
    committedBalance int
    lockState:
      mode: UNLOCKED | READ | WRITE
      holders: []TransactionID      // for read locks (multiple allowed)
      holder:  TransactionID        // for write lock (exclusive)
      waitQueue: []WaitEntry        // blocked transactions + their requested mode
```

### Per-transaction write buffer (on each branch server)
```
tentativeWrites: map[TransactionID]map[AccountName]int
```
Tentative writes are invisible to other transactions. A transaction reads its own
write buffer first; if no entry, falls back to committedBalance.

### Coordinator state (on the coordinating server, per transaction)
```
Transaction:
  id:           TransactionID
  timestamp:    int64             // assigned at BEGIN, used for wound-wait
  participants: []ServerID        // servers contacted so far (grows dynamically)
  clientConn:   net.Conn
```

---

## Locking Protocol (Strict 2PL)

- **Read lock (BALANCE):** shared — multiple transactions can hold simultaneously
- **Write lock (DEPOSIT/WITHDRAW):** exclusive — one holder at a time
- Locks are held until the transaction commits or aborts (strict 2PL)
- This prevents dirty reads: T2 cannot read uncommitted writes from T1

### Lock acquisition
1. Check account's lock state
2. If compatible (e.g., read + read), grant immediately
3. If incompatible, apply wound-wait (see below)

---

## Deadlock Prevention — Wound-Wait

Each transaction receives a timestamp at `BEGIN`. Lower timestamp = older.

| Scenario | Action |
|---|---|
| Older T1 wants lock held by younger T2 | T1 **wounds** T2 (T2 is aborted) |
| Younger T2 wants lock held by older T1 | T2 **waits** |

This ensures waiting only goes in one direction (younger→older), making cycles impossible.

No timeouts are used (per spec requirement).

### When a transaction is wounded
- The server holding the lock sends `ABORTED` back to the wounded transaction's coordinator
- The wounded transaction's coordinator sends `ABORT` to all servers in its participant list
- Each server clears tentative writes and releases all locks held by that transaction

---

## Coordinator Responsibilities

The client connects to a randomly selected server as coordinator. The client only
ever talks to the coordinator.

The coordinator:
- Routes each command to the correct branch server (by account prefix)
- Maintains the participant list (set of servers contacted, grows per command)
- Handles `COMMIT` and `ABORT` by running 2PC or broadcasting abort
- For accounts on its own branch: calls local function directly (no network hop)

---

## Two-Phase Commit (at COMMIT)

### Phase 1 — Prepare (vote)
Coordinator sends `PREPARE` to all participants.

Each participant:
1. Checks that all accounts written during the transaction have non-negative balances
2. Votes `YES` or `NO`

If any participant votes `NO`, coordinator sends `ABORT` to all and replies `ABORTED` to client.

### Phase 2 — Commit or Abort
If all votes are `YES`:
- Coordinator sends `COMMIT` to all participants
- Each participant applies tentative writes to committed balances, releases all locks
- Coordinator replies `COMMIT OK` to client
- Each server prints balances of all non-zero accounts after committing

If any vote is `NO`:
- Coordinator sends `ABORT` to all participants
- Each participant clears tentative writes, releases all locks
- Coordinator replies `ABORTED` to client

---

## Abort (mid-transaction)

Triggered by: user `ABORT` command, `NOT FOUND` on BALANCE/WITHDRAW, wound-wait, or negative balance at commit.

Steps:
1. Coordinator sends `ABORT` to all servers in participant list
2. Each server: delete tentative write buffer for transaction, release all locks
3. Coordinator replies `ABORTED` to client

---

## Wire Protocol

Coordinator ↔ participant communication over TCP.
Each message is a line of text (newline-delimited) for simplicity.

Request format (coordinator → participant):
```
<txn_id> <command> [args...]
e.g.: tx42 DEPOSIT foo 10
      tx42 BALANCE foo
      tx42 PREPARE
      tx42 COMMIT
      tx42 ABORT
```

Response format (participant → coordinator):
```
OK [value]    — success, optional value for BALANCE
ABORTED       — transaction aborted (wound, not found, etc.)
YES / NO      — prepare vote
```

---

## Implementation Order

1. Server data structures (account store, lock table, write buffers)
2. Lock acquisition/release + wound-wait logic
3. Single-server transactions (coordinator = participant, no forwarding)
4. Cross-server forwarding (coordinator routes to remote participants)
5. 2PC at commit
6. Client (parse stdin, connect to random server, print responses)
