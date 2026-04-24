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
