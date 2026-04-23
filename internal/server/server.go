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

	mu sync.RWMutex
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
}

type Transaction struct {
	ID            string
	Timestamp     int64
	CoordinatorID string
	Participants  map[string]struct{}
	Aborted       bool
}

type ClusterConfig map[string]NodeInfo

type NodeInfo struct {
	BranchID string
	Host     string
	Port     string
}

func NewServer(branchID string, cfg ClusterConfig) *Server {
	return &Server{
		BranchID:        branchID,
		Config:          cfg,
		Accounts:        make(map[string]*Account),
		Locks:           make(map[string]*AccountLock),
		TentativeWrites: make(map[string]map[string]int),
		Transactions:    make(map[string]*Transaction),
		Participants:    make(map[string]*ParticipantClient),
	}
}
