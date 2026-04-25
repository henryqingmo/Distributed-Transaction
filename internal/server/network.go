package server

import (
	"bufio"
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
			account := parseAccount(fields[2])
			response = s.handleBalance(txnID, account)

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
