package network

type Operation int

const (
	OperationBegin Operation = iota
	OperationDeposit
	OperationWithdraw
	OperationBalance
	OperationCommit
	OperationAbort
)

type ResponseMessage int

const (
	ResponseOK ResponseMessage = iota
	ResponseAborted
	ResponseYES
	ResponseNO
)

type Message struct {
	TxnID      string
	FromBranch string
	ToBranch   string
	Operation  Operation
	Account    string
	Amount     int
}

type Response struct {
	TxnID           string
	FromBranch      string
	ResponseMessage ResponseMessage
	Balance         int
}

type ConfigEntry struct {
	BranchID string
	Host     string
	Port     string
}
