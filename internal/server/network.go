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

		txnID := fields[0]
		op := fields[1]

		var response string

		switch op {
		case "DEPOSIT":
			// txnID DEPOSIT branch.account amount
			account, amount, ok := parseAccountAmount(fields)
			if !ok {
				continue
			}
			response = s.handleDeposit(txnID, account, amount)

		case "WITHDRAW":
			// txnID WITHDRAW branch.account amount
			account, amount, ok := parseAccountAmount(fields)
			if !ok {
				continue
			}
			response = s.handleWithdraw(txnID, account, amount)

		case "BALANCE":
			// txnID BALANCE branch.account
			if len(fields) < 3 {
				continue
			}
			response = s.handleBalance(txnID, fields[2])

		case "PREPARE":
			response = s.handlePrepare(txnID)

		case "COMMIT":
			response = s.handleCommit(txnID)

		case "ABORT":
			response = s.handleAbort(txnID)
		}

		fmt.Fprintln(writer, response)
		writer.Flush()
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

func (s *Server) handleDeposit(txnID string, account string, amount int) string {
	s.mu.Lock()
	ch := make(chan struct{})
	result := s.tryAcquireLock(txnID, account, lock.WRITE, ch)
	if result == "wait" {
		s.mu.Unlock()
		<-ch
		s.mu.Lock()
	}
	if s.Transactions[txnID].Aborted {
		s.mu.Unlock()
		return "ABORTED"
	}
	s.execDeposit(txnID, account, amount)
	s.mu.Unlock()
	return "OK"
}

func (s *Server) handleWithdraw(txnID string, account string, amount int) string {
	s.mu.Lock()
	ch := make(chan struct{})
	result := s.tryAcquireLock(txnID, account, lock.WRITE, ch)
	if result == "wait" {
		s.mu.Unlock()
		<-ch
		s.mu.Lock()
	}
	if s.Transactions[txnID].Aborted {
		s.mu.Unlock()
		return "ABORTED"
	}
	ok := s.execWithdraw(txnID, account, amount)
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
	ch := make(chan struct{})
	result := s.tryAcquireLock(txnID, account, lock.READ, ch)
	if result == "wait" {
		s.mu.Unlock()
		<-ch
		s.mu.Lock()
	}
	if s.Transactions[txnID].Aborted {
		s.mu.Unlock()
		return "ABORTED"
	}
	balance, found := s.getBalance(txnID, account)
	s.mu.Unlock()
	if !found {
		s.handleAbort(txnID)
		return "NOT FOUND, ABORTED"
	}
	return fmt.Sprintf("%s = %d", rawAccount, balance)
}

func (s *Server) handlePrepare(txnID string) string {
	s.mu.Lock()
	vote := s.execPrepare(txnID)
	s.mu.Unlock()
	if vote == VoteYes {
		return "YES"
	}
	return "NO"
}

func (s *Server) handleCommit(txnID string) string {
	s.mu.Lock()
	s.execCommit(txnID)
	for account := range s.Transactions[txnID].LockedAccount {
		s.processWaitQueue(s.Locks[account])
	}
	s.mu.Unlock()
	return "OK"
}

func (s *Server) handleAbort(txnID string) string {
	s.mu.Lock()
	s.execAbort(txnID)
	for account := range s.Transactions[txnID].LockedAccount {
		s.processWaitQueue(s.Locks[account])
	}
	s.mu.Unlock()
	return "ABORTED"
}
