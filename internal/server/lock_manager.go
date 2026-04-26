package server

import "cs425_mp3/internal/lock"

type LockDecision string

const (
	LockGranted LockDecision = "granted"
	LockWait    LockDecision = "wait"
	LockWound   LockDecision = "wound"
)

func (s *Server) lockForAccount(account string) *AccountLock {
	acl, exists := s.Locks[account]
	if exists {
		return acl
	}
	acl = &AccountLock{
		Account:   account,
		State:     lock.UNLOCKED,
		ReadHolds: make(map[string]struct{}),
	}
	s.Locks[account] = acl
	return acl
}

func (s *Server) acquireLock(txnID string, account string, mode lock.LockState, ch chan struct{}) LockDecision {
	acl := s.lockForAccount(account)

	requesterTs, ok := s.txnTimestamp(txnID)
	if !ok {
		return LockGranted
	}

	if mode == lock.READ {
		if acl.WriteHold == txnID {
			s.recordTxnLock(txnID, account)
			return LockGranted
		}
		if acl.WriteHold == "" {
			acl.ReadHolds[txnID] = struct{}{}
			acl.State = lock.READ
			s.recordTxnLock(txnID, account)
			return LockGranted
		}
		holderTs, ok := s.txnTimestamp(acl.WriteHold)
		if !ok || requesterTs < holderTs {
			s.markTxnAborted(acl.WriteHold)
			acl.WriteHold = ""
			acl.ReadHolds[txnID] = struct{}{}
			acl.State = lock.READ
			s.recordTxnLock(txnID, account)
			return LockWound
		}
		acl.WaitQueue = append(acl.WaitQueue, WaitEntry{TxnID: txnID, Mode: mode, Ready: ch})
		return LockWait
	}

	if acl.WriteHold != "" {
		if acl.WriteHold == txnID {
			s.recordTxnLock(txnID, account)
			return LockGranted
		}
		holderTs, ok := s.txnTimestamp(acl.WriteHold)
		if ok && holderTs < requesterTs {
			acl.WaitQueue = append(acl.WaitQueue, WaitEntry{TxnID: txnID, Mode: mode, Ready: ch})
			return LockWait
		}
	}

	for holderTxnID := range acl.ReadHolds {
		if holderTxnID == txnID {
			continue
		}
		holderTs, ok := s.txnTimestamp(holderTxnID)
		if ok && holderTs < requesterTs {
			acl.WaitQueue = append(acl.WaitQueue, WaitEntry{TxnID: txnID, Mode: mode, Ready: ch})
			return LockWait
		}
	}

	wounded := false
	if acl.WriteHold != "" {
		s.markTxnAborted(acl.WriteHold)
		wounded = true
	}
	for holderTxnID := range acl.ReadHolds {
		if holderTxnID == txnID {
			delete(acl.ReadHolds, holderTxnID)
			continue
		}
		s.markTxnAborted(holderTxnID)
		delete(acl.ReadHolds, holderTxnID)
		wounded = true
	}

	acl.WriteHold = txnID
	acl.State = lock.WRITE
	s.recordTxnLock(txnID, account)
	if wounded {
		return LockWound
	}
	return LockGranted
}

func (s *Server) processWaitQueue(account string) {
	acl := s.Locks[account]
	if acl == nil || len(acl.WaitQueue) == 0 {
		return
	}

	// if first in queue is write lock, and
	// both read, write unlocked, pop and grant
	front := acl.WaitQueue[0]
	if front.Mode == lock.WRITE {
		if acl.WriteHold == "" && len(acl.ReadHolds) == 0 {
			acl.WaitQueue = acl.WaitQueue[1:]
			acl.WriteHold = front.TxnID
			acl.State = lock.WRITE
			s.recordTxnLock(front.TxnID, acl.Account)
			close(front.Ready)
		}
		return
	}

	if acl.WriteHold != "" {
		return
	}

	// if write unlocked, read locked, grant and pop all consective read
	i := 0
	for i < len(acl.WaitQueue) && acl.WaitQueue[i].Mode == lock.READ {
		entry := acl.WaitQueue[i]
		acl.ReadHolds[entry.TxnID] = struct{}{}
		acl.State = lock.READ
		s.recordTxnLock(entry.TxnID, acl.Account)
		close(entry.Ready)
		i++
	}
	acl.WaitQueue = acl.WaitQueue[i:]
}

func (s *Server) releaseTransactionLocks(txnID string) []string {
	accounts := s.txnLockedAccounts(txnID)
	for _, account := range accounts {
		acl := s.Locks[account]
		if acl == nil {
			continue
		}
		if acl.WriteHold == txnID {
			acl.WriteHold = ""
		}
		// delete from readholds
		delete(acl.ReadHolds, txnID)
		// delete from wait queue
		newWaitQueue := make([]WaitEntry, 0, len(acl.WaitQueue))
		for _, entry := range acl.WaitQueue {
			if entry.TxnID != txnID {
				newWaitQueue = append(newWaitQueue, entry)
			}
		}
		acl.WaitQueue = newWaitQueue
	}
	return accounts
}
