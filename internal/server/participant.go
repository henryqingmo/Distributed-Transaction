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
