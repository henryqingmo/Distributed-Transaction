package server

import (
	"cs425_mp3/internal/lock"
	"sync"
)

type Server struct {
	BranchID string
	Config   ClusterConfig

	Accounts        map[string]*Account
	Locks           map[string]*AccountLock
	TentativeWrites map[string]map[string]int
	Transactions    map[string]*Transaction
	Participants    map[string]*ParticipantClient

	mu sync.Mutex
}

type Account struct {
	Name             string
	CommittedBalance int
}

type AccountLock struct {
	Account   string
	State     lock.LockState
	ReadHolds map[string]struct{}
	WriteHold string
	WaitQueue []WaitEntry
}

type WaitEntry struct {
	TxnID string
	Mode  lock.LockState
	Ready chan struct{}
}

type Transaction struct {
	ID            string
	Timestamp     int64
	CoordinatorID string
	Participants  map[string]struct{}
	Aborted       bool
	LockedAccount map[string]struct{}
}

type ClusterConfig map[string]NodeInfo

type NodeInfo struct {
	BranchID string
	Host     string
	Port     string
}

func NewServer(branchID string, cfg ClusterConfig) *Server {
	s := &Server{
		BranchID:        branchID,
		Config:          cfg,
		Accounts:        make(map[string]*Account),
		Locks:           make(map[string]*AccountLock),
		TentativeWrites: make(map[string]map[string]int),
		Transactions:    make(map[string]*Transaction),
		Participants:    make(map[string]*ParticipantClient)}
	return s
}

// tryAcquireLock attempts to acquire a lock for txnID on account.
// Returns: "granted", "wait", or "wound"
func (s *Server) tryAcquireLock(txnID string, account string, mode lock.LockState, ch chan struct{}) string {
	acl, exists := s.Locks[account]
	if !exists {
		acl = &AccountLock{
			Account:   account,
			State:     lock.UNLOCKED,
			ReadHolds: make(map[string]struct{}),
		}
		s.Locks[account] = acl
	}

	requesterTs := s.Transactions[txnID].Timestamp

	if mode == lock.READ {
		if acl.WriteHold == "" {
			// No write hold — grant read immediately
			acl.ReadHolds[txnID] = struct{}{}
			acl.State = lock.READ
			s.Transactions[txnID].LockedAccount[account] = struct{}{}
			return "granted"
		}
		// Write hold exists — wound-wait against holder
		holderTs := s.Transactions[acl.WriteHold].Timestamp
		if requesterTs < holderTs {
			// Requester is older — wound the write holder
			s.Transactions[acl.WriteHold].Aborted = true
			acl.WriteHold = ""
			acl.ReadHolds[txnID] = struct{}{}
			acl.State = lock.READ
			s.Transactions[txnID].LockedAccount[account] = struct{}{}
			return "wound"
		}
		// Requester is younger — wait
		acl.WaitQueue = append(acl.WaitQueue, WaitEntry{TxnID: txnID, Mode: mode, Ready: ch})
		return "wait"
	}

	// mode == WRITE

	if acl.WriteHold != "" {
		holderTs := s.Transactions[acl.WriteHold].Timestamp

		if holderTs < requesterTs {
			//wait
			acl.WaitQueue = append(acl.WaitQueue, WaitEntry{TxnID: txnID, Mode: mode, Ready: ch})
			return "wait"
		}

	}

	for key := range acl.ReadHolds {
		holderTs := s.Transactions[key].Timestamp
		if holderTs < requesterTs {
			acl.WaitQueue = append(acl.WaitQueue, WaitEntry{TxnID: txnID, Mode: mode, Ready: ch})
			return "wait"
		}
	}

	wounded := false

	// wound
	if acl.WriteHold != "" {
		s.Transactions[acl.WriteHold].Aborted = true
		wounded = true
	}
	for key := range acl.ReadHolds {
		s.Transactions[key].Aborted = true
		delete(acl.ReadHolds, key)
		wounded = true
	}

	acl.WriteHold = txnID
	acl.State = lock.WRITE
	s.Transactions[txnID].LockedAccount[account] = struct{}{}

	if wounded {
		return "wound"
	}

	return "granted"

}

func (s *Server) processWaitQueue(acl *AccountLock) {
	if len(acl.WaitQueue) == 0 {
		return
	}

	front := acl.WaitQueue[0]

	if front.Mode == lock.WRITE {
		// Only grant write if no holders remain
		if acl.WriteHold == "" && len(acl.ReadHolds) == 0 {
			acl.WaitQueue = acl.WaitQueue[1:]
			acl.WriteHold = front.TxnID
			acl.State = lock.WRITE
			s.Transactions[front.TxnID].LockedAccount[acl.Account] = struct{}{}
			close(front.Ready)
		}
		return
	}

	// Front wants READ — grant all consecutive READs (no write hold allowed)
	if acl.WriteHold != "" {
		return
	}
	i := 0
	for i < len(acl.WaitQueue) && acl.WaitQueue[i].Mode == lock.READ {
		entry := acl.WaitQueue[i]
		acl.ReadHolds[entry.TxnID] = struct{}{}
		acl.State = lock.READ
		s.Transactions[entry.TxnID].LockedAccount[acl.Account] = struct{}{}
		close(entry.Ready)
		i++
	}
	acl.WaitQueue = acl.WaitQueue[i:]
}
