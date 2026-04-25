package server

type ParticipantClient struct {
	BranchID string
	Address  string
}

type PrepareResult struct {
	TxnID string
	Vote  Vote
}

type Vote int

const (
	VoteYes Vote = iota
	VoteNo
)

func (s *Server) getBalance(txnID, account string) (int, bool) {
	if tentative, exists := s.TentativeWrites[txnID][account]; exists {
		return tentative, true
	}
	acct, found := s.Accounts[account]
	if !found {
		return 0, false
	}
	return acct.CommittedBalance, true
}

func (s *Server) execDeposit(txnID, account string, amount int) {
	if s.TentativeWrites[txnID] == nil {
		s.TentativeWrites[txnID] = make(map[string]int)
	}
	curr, _ := s.getBalance(txnID, account)
	s.TentativeWrites[txnID][account] = curr + amount
}

// Returns false if account not found (triggers abort)
func (s *Server) execWithdraw(txnID, account string, amount int) bool {
	curr, found := s.getBalance(txnID, account)
	if !found {
		return false
	}
	if s.TentativeWrites[txnID] == nil {
		s.TentativeWrites[txnID] = make(map[string]int)
	}
	s.TentativeWrites[txnID][account] = curr - amount
	return true
}

func (s *Server) execPrepare(txnID string) Vote {
	if s.Transactions[txnID].Aborted {
		return VoteNo
	}
	for _, balance := range s.TentativeWrites[txnID] {
		if balance < 0 {
			return VoteNo
		}
	}
	return VoteYes
}

func (s *Server) execCommit(txnID string) bool {
	for account, balance := range s.TentativeWrites[txnID] {

		if s.Accounts[account] == nil {
			s.Accounts[account] = &Account{Name: account}
		}

		s.Accounts[account].CommittedBalance = balance

	}

	delete(s.TentativeWrites, txnID)

	for account := range s.Transactions[txnID].LockedAccount {
		actl := s.Locks[account]
		if actl.WriteHold == txnID {
			actl.WriteHold = ""
		}
		delete(actl.ReadHolds, txnID)

		// delete from waitQueue
		newWaitQueue := make([]WaitEntry, 0)
		for _, entry := range actl.WaitQueue {
			if entry.TxnID != txnID {
				newWaitQueue = append(newWaitQueue, entry)
			}
		}
		actl.WaitQueue = newWaitQueue
	}

	return true
}

func (s *Server) execAbort(txnID string) bool {
	// release all locks
	// remove from queue
	// remove tentative write
	for account := range s.Transactions[txnID].LockedAccount {
		actl := s.Locks[account]
		if actl.WriteHold == txnID {
			actl.WriteHold = ""
		}
		delete(actl.ReadHolds, txnID)

		newWaitQueue := make([]WaitEntry, 0)
		for _, entry := range actl.WaitQueue {
			if entry.TxnID != txnID {
				newWaitQueue = append(newWaitQueue, entry)
			}
		}
		actl.WaitQueue = newWaitQueue
	}
	delete(s.TentativeWrites, txnID)
	return true
}
