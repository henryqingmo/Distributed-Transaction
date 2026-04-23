package network

type ClientSession struct {
	ClientID           string
	CoordinatorBranch  string
	CurrentTransaction string
}

type ServerSession struct {
	BranchID string
	ListenOn string
}
