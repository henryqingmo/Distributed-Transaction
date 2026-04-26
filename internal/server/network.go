package server

import (
	"bufio"
	"cs425_mp3/internal/lock"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func (s *Server) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	writer := bufio.NewWriter(conn)
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		response, ok := s.routeCommand(fields)
		if !ok {
			continue
		}
		fmt.Fprintln(writer, response)
		writer.Flush()
	}
}

func (s *Server) routeCommand(fields []string) (string, bool) {
	txnID := fields[0]
	op := fields[1]

	switch op {
	case "DEPOSIT":
		account, amount, ok := parseAccountAmount(fields)
		if !ok {
			return "", false
		}
		return s.handleDeposit(txnID, account, amount), true
	case "WITHDRAW":
		account, amount, ok := parseAccountAmount(fields)
		if !ok {
			return "", false
		}
		return s.handleWithdraw(txnID, account, amount), true
	case "BALANCE":
		if len(fields) < 3 {
			return "", false
		}
		return s.handleBalance(txnID, fields[2]), true
	case "PREPARE":
		return s.handlePrepare(txnID), true
	case "COMMIT":
		return s.handleCommit(txnID), true
	case "ABORT":
		return s.handleAbort(txnID), true
	default:
		return "", false
	}
}

// "B.foo" → "foo"
func parseAccount(raw string) string {
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return raw
}

func parseAccountAmount(fields []string) (string, int, bool) {
	if len(fields) < 4 {
		return "", 0, false
	}
	account := parseAccount(fields[2])
	amount, err := strconv.Atoi(fields[3])
	if err != nil {
		return "", 0, false
	}
	return account, amount, true
}

func (s *Server) waitForLockIfNeeded(txnID, account string, mode lock.LockState) bool {
	ch := make(chan struct{})
	result := s.acquireLock(txnID, account, mode, ch)
	if result == LockWait {
		s.mu.Unlock()
		<-ch
		s.mu.Lock()
	}
	return !s.isTxnAborted(txnID)
}

func (s *Server) handleDeposit(txnID string, account string, amount int) string {
	s.mu.Lock()
	s.coordinatorSvc.TrackParticipant(s, txnID, s.BranchID)
	if !s.waitForLockIfNeeded(txnID, account, lock.WRITE) {
		s.mu.Unlock()
		return "ABORTED"
	}
	s.participantSvc.Deposit(s, txnID, account, amount)
	s.mu.Unlock()
	return "OK"
}

func (s *Server) handleWithdraw(txnID string, account string, amount int) string {
	s.mu.Lock()
	s.coordinatorSvc.TrackParticipant(s, txnID, s.BranchID)
	if !s.waitForLockIfNeeded(txnID, account, lock.WRITE) {
		s.mu.Unlock()
		return "ABORTED"
	}
	ok := s.participantSvc.Withdraw(s, txnID, account, amount)
	s.mu.Unlock()
	if !ok {
		s.handleAbort(txnID)
		return "NOT FOUND, ABORTED"
	}
	return "OK"
}

// handleBalance takes the raw "B.foo" account string for response formatting
func (s *Server) handleBalance(txnID string, rawAccount string) string {
	account := parseAccount(rawAccount)
	s.mu.Lock()
	s.coordinatorSvc.TrackParticipant(s, txnID, s.BranchID)
	if !s.waitForLockIfNeeded(txnID, account, lock.READ) {
		s.mu.Unlock()
		return "ABORTED"
	}
	balance, found := s.participantSvc.GetBalance(s, txnID, account)
	s.mu.Unlock()
	if !found {
		s.handleAbort(txnID)
		return "NOT FOUND, ABORTED"
	}
	return fmt.Sprintf("%s = %d", rawAccount, balance)
}

func (s *Server) handlePrepare(txnID string) string {
	s.mu.Lock()
	vote := s.participantSvc.Prepare(s, txnID)
	s.mu.Unlock()
	if vote == VoteYes {
		return "YES"
	}
	return "NO"
}

func (s *Server) handleCommit(txnID string) string {
	s.mu.Lock()
	releasedAccounts := s.participantSvc.Commit(s, txnID)
	for _, account := range releasedAccounts {
		s.processWaitQueue(account)
	}
	s.mu.Unlock()
	return "OK"
}

func (s *Server) handleAbort(txnID string) string {
	s.mu.Lock()
	releasedAccounts := s.participantSvc.Abort(s, txnID)
	for _, account := range releasedAccounts {
		s.processWaitQueue(account)
	}
	s.mu.Unlock()
	return "ABORTED"
}
