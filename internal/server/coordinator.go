package server

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

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

func (s *Server) handleClientSession(conn net.Conn, scanner *bufio.Scanner, writer *bufio.Writer) {
	ts := time.Now().UnixNano()
	txnID := fmt.Sprintf("txn:%d", ts)

	s.mu.Lock()
	s.createTransaction(txnID, ts)
	s.mu.Unlock()

	fmt.Fprintln(writer, "OK")
	writer.Flush()

	branchConns := make(map[string]net.Conn)
	defer func() {
		for _, c := range branchConns {
			c.Close()
		}
	}()

	for scanner.Scan() {
		line := scanner.Text()
		// routing and 2PC handled in next sections
		_ = line
	}
}
