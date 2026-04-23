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

type ParticipantRequest struct {
	TxnID   string
	Command string
	Account string
	Amount  int
}

type ParticipantResponse struct {
	TxnID   string
	Status  string
	Balance int
}
