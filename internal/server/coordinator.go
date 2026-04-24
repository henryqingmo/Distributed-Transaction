package server

type Coordinator struct {
	Server      *Server
	TxnID       string
	ClientID    string
	Coordinator string
}
