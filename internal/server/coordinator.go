package server

type Coordinator struct {
	Server      *Server
	TxnID       string
	ClientID    string
	Coordinator string
}

type CoordinatorRequest struct {
	ClientID string
	Command  string
	Account  string
	Amount   int
}

type CoordinatorResponse struct {
	Status  string
	Account string
	Balance int
}
