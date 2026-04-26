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

type ParticipantService struct{}

func NewParticipantService() *ParticipantService {
	return &ParticipantService{}
}

func (p *ParticipantService) GetBalance(s *Server, txnID, account string) (int, bool) {
	if tentative, exists := s.txnWriteSet(txnID)[account]; exists {
		return tentative, true
	}
	acct, found := s.Accounts[account]
	if !found {
		return 0, false
	}
	return acct.CommittedBalance, true
}

func (p *ParticipantService) Deposit(s *Server, txnID, account string, amount int) {
	writeSet := s.ensureTxnWriteSet(txnID)
	curr, _ := p.GetBalance(s, txnID, account)
	writeSet[account] = curr + amount
}

// Returns false if account not found (triggers abort)
func (p *ParticipantService) Withdraw(s *Server, txnID, account string, amount int) bool {
	curr, found := p.GetBalance(s, txnID, account)
	if !found {
		return false
	}
	writeSet := s.ensureTxnWriteSet(txnID)
	writeSet[account] = curr - amount
	return true
}

func (p *ParticipantService) Prepare(s *Server, txnID string) Vote {
	if s.isTxnAborted(txnID) {
		return VoteNo
	}
	for _, balance := range s.txnWriteSet(txnID) {
		if balance < 0 {
			return VoteNo
		}
	}
	return VoteYes
}

func (p *ParticipantService) Commit(s *Server, txnID string) []string {
	for account, balance := range s.txnWriteSet(txnID) {

		if s.Accounts[account] == nil {
			s.Accounts[account] = &Account{Name: account}
		}

		s.Accounts[account].CommittedBalance = balance

	}

	delete(s.TentativeWrites, txnID)
	// returns all the accounts under txnID, aswell as unlocking them
	return s.releaseTransactionLocks(txnID)
}

func (p *ParticipantService) Abort(s *Server, txnID string) []string {
	s.markTxnAborted(txnID)
	delete(s.TentativeWrites, txnID)
	return s.releaseTransactionLocks(txnID)
}
