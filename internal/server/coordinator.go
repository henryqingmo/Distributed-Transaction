package server

type Coordinator struct {
	Server      *Server
	TxnID       string
	ClientID    string
	Coordinator string
}

type CoordinatorService struct{}

func NewCoordinatorService() *CoordinatorService {
	return &CoordinatorService{}
}

func (c *CoordinatorService) TrackParticipant(s *Server, txnID string, branchID string) {
	s.trackTxnParticipant(txnID, branchID)
}

func (c *CoordinatorService) Participants(s *Server, txnID string) []string {
	return s.txnParticipants(txnID)
}
