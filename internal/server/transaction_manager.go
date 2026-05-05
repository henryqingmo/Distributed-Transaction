package server

import (
	"strconv"
	"strings"
)

// txnID format: "txn:<unixnano>" — coordinator embeds timestamp so all
// participants agree on transaction age for wound-wait without extra messages.
func parseTimestampFromTxnID(txnID string) int64 {
	idx := strings.LastIndex(txnID, ":")
	if idx == -1 {
		return 0
	}
	ts, _ := strconv.ParseInt(txnID[idx+1:], 10, 64)
	return ts
}

func (s *Server) createTransaction(txnID string, timestamp int64) {
	s.Transactions[txnID] = &Transaction{
		ID:            txnID,
		Timestamp:     timestamp,
		Participants:  make(map[string]struct{}),
		LockedAccount: make(map[string]struct{}),
	}
}

// ensureTransaction creates a Transaction record the first time a participant
// sees a txnID forwarded from a coordinator. Timestamp is parsed from the ID.
func (s *Server) ensureTransaction(txnID string) {
	if _, ok := s.Transactions[txnID]; !ok {
		s.createTransaction(txnID, parseTimestampFromTxnID(txnID))
	}
}

func (s *Server) getTransaction(txnID string) (*Transaction, bool) {
	txn, ok := s.Transactions[txnID]
	return txn, ok
}

func (s *Server) isTxnAborted(txnID string) bool {
	txn, ok := s.getTransaction(txnID)
	if !ok {
		return true
	}
	return txn.Aborted
}

func (s *Server) markTxnAborted(txnID string) {
	txn, ok := s.getTransaction(txnID)
	if !ok {
		return
	}
	txn.Aborted = true
}

func (s *Server) txnTimestamp(txnID string) (int64, bool) {
	txn, ok := s.getTransaction(txnID)
	if !ok {
		return 0, false
	}
	return txn.Timestamp, true
}

func (s *Server) ensureTxnWriteSet(txnID string) map[string]int {
	if s.TentativeWrites[txnID] == nil {
		s.TentativeWrites[txnID] = make(map[string]int)
	}
	return s.TentativeWrites[txnID]
}

func (s *Server) txnWriteSet(txnID string) map[string]int {
	if s.TentativeWrites[txnID] == nil {
		return nil
	}
	return s.TentativeWrites[txnID]
}

func (s *Server) recordTxnLock(txnID string, account string) {
	txn, ok := s.getTransaction(txnID)
	if !ok {
		return
	}
	if txn.LockedAccount == nil {
		txn.LockedAccount = make(map[string]struct{})
	}
	txn.LockedAccount[account] = struct{}{}
}

func (s *Server) txnLockedAccounts(txnID string) []string {
	txn, ok := s.getTransaction(txnID)
	if !ok {
		return nil
	}
	accounts := make([]string, 0, len(txn.LockedAccount))
	for account := range txn.LockedAccount {
		accounts = append(accounts, account)
	}
	return accounts
}

func (s *Server) trackTxnParticipant(txnID string, branchID string) {
	txn, ok := s.getTransaction(txnID)
	if !ok {
		return
	}
	if txn.Participants == nil {
		txn.Participants = make(map[string]struct{})
	}
	txn.Participants[branchID] = struct{}{}
}

func (s *Server) txnParticipants(txnID string) []string {
	txn, ok := s.getTransaction(txnID)
	if !ok {
		return nil
	}
	participants := make([]string, 0, len(txn.Participants))
	for participant := range txn.Participants {
		participants = append(participants, participant)
	}
	return participants
}
