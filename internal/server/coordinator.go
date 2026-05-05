package server

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
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

// participantConn bundles a TCP connection with its buffered reader/writer.
type participantConn struct {
	conn    net.Conn
	writer  *bufio.Writer
	scanner *bufio.Scanner
}

func (s *Server) getOrDialBranch(branch string, pconns map[string]*participantConn) (*participantConn, error) {
	if pc, ok := pconns[branch]; ok {
		return pc, nil
	}
	node, ok := s.Config[branch]
	if !ok {
		return nil, fmt.Errorf("unknown branch: %s", branch)
	}
	conn, err := net.Dial("tcp", node.Host+":"+node.Port)
	if err != nil {
		return nil, err
	}
	pc := &participantConn{
		conn:    conn,
		writer:  bufio.NewWriter(conn),
		scanner: bufio.NewScanner(conn),
	}
	pconns[branch] = pc
	return pc, nil
}

func sendRecv(pc *participantConn, msg string) (string, error) {
	fmt.Fprintln(pc.writer, msg)
	if err := pc.writer.Flush(); err != nil {
		return "", err
	}
	if !pc.scanner.Scan() {
		return "", fmt.Errorf("connection closed")
	}
	return pc.scanner.Text(), nil
}

// branchOf extracts the branch ID from a qualified account name like "A.foo".
func branchOf(rawAccount string) string {
	idx := strings.Index(rawAccount, ".")
	if idx == -1 {
		return ""
	}
	return rawAccount[:idx]
}

// routeOperation sends a DEPOSIT/WITHDRAW/BALANCE to the correct branch —
// calling local handlers directly for own branch, or forwarding over TCP.
func (s *Server) routeOperation(txnID, cmd, rawAccount string, amount int, pconns map[string]*participantConn, participants map[string]bool) string {
	branch := branchOf(rawAccount)
	if branch == "" {
		return "ABORTED"
	}
	participants[branch] = true

	if branch == s.BranchID {
		account := parseAccount(rawAccount)
		switch cmd {
		case "DEPOSIT":
			return s.handleDeposit(txnID, account, amount)
		case "WITHDRAW":
			return s.handleWithdraw(txnID, account, amount)
		case "BALANCE":
			return s.handleBalance(txnID, rawAccount)
		}
		return "ABORTED"
	}

	var msg string
	switch cmd {
	case "DEPOSIT", "WITHDRAW":
		msg = fmt.Sprintf("%s %s %s %d", txnID, cmd, rawAccount, amount)
	case "BALANCE":
		msg = fmt.Sprintf("%s %s %s", txnID, cmd, rawAccount)
	default:
		return "ABORTED"
	}

	pc, err := s.getOrDialBranch(branch, pconns)
	if err != nil {
		return "ABORTED"
	}
	resp, err := sendRecv(pc, msg)
	if err != nil {
		return "ABORTED"
	}
	return resp
}

// broadcastAbort sends ABORT to every branch the transaction touched.
func (s *Server) broadcastAbort(txnID string, participants map[string]bool, pconns map[string]*participantConn) {
	for branch := range participants {
		if branch == s.BranchID {
			s.handleAbort(txnID)
		} else {
			pc, err := s.getOrDialBranch(branch, pconns)
			if err != nil {
				continue
			}
			sendRecv(pc, fmt.Sprintf("%s ABORT", txnID))
		}
	}
}

func (s *Server) handleClientSession(conn net.Conn, scanner *bufio.Scanner, writer *bufio.Writer) {
	ts := time.Now().UnixNano()
	txnID := fmt.Sprintf("txn:%d", ts)

	s.mu.Lock()
	s.createTransaction(txnID, ts)
	s.mu.Unlock()

	fmt.Fprintln(writer, "OK")
	writer.Flush()

	pconns := make(map[string]*participantConn)
	defer func() {
		for _, pc := range pconns {
			pc.conn.Close()
		}
	}()
	participants := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		cmd := fields[0]
		var resp string

		switch cmd {
		case "DEPOSIT", "WITHDRAW":
			if len(fields) < 3 {
				continue
			}
			amount, _ := strconv.Atoi(fields[2])
			resp = s.routeOperation(txnID, cmd, fields[1], amount, pconns, participants)
		case "BALANCE":
			if len(fields) < 2 {
				continue
			}
			resp = s.routeOperation(txnID, "BALANCE", fields[1], 0, pconns, participants)
		case "COMMIT":
			resp = s.runTwoPhaseCommit(txnID, participants, pconns)
			fmt.Fprintln(writer, resp)
			writer.Flush()
			return
		case "ABORT":
			s.broadcastAbort(txnID, participants, pconns)
			fmt.Fprintln(writer, "ABORTED")
			writer.Flush()
			return
		default:
			continue
		}

		// Any ABORTED or NOT FOUND response means the transaction is dead —
		// clean up all other participants before replying to the client.
		if resp == "ABORTED" || strings.HasPrefix(resp, "NOT FOUND") {
			s.broadcastAbort(txnID, participants, pconns)
			fmt.Fprintln(writer, resp)
			writer.Flush()
			return
		}

		fmt.Fprintln(writer, resp)
		writer.Flush()
	}
}

func (s *Server) runTwoPhaseCommit(txnID string, participants map[string]bool, pconns map[string]*participantConn) string {
	// Snapshot branch→conn pairs before spawning goroutines to avoid
	// concurrent reads/writes on the pconns map.
	type target struct {
		branch string
		pc     *participantConn // nil means own branch (call locally)
	}
	targets := make([]target, 0, len(participants))
	for branch := range participants {
		if branch == s.BranchID {
			targets = append(targets, target{branch: branch, pc: nil})
		} else {
			targets = append(targets, target{branch: branch, pc: pconns[branch]})
		}
	}

	// Phase 1 — PREPARE in parallel
	type voteResult struct {
		branch string
		yes    bool
	}
	votes := make(chan voteResult, len(targets))
	for _, t := range targets {
		t := t
		go func() {
			if t.pc == nil {
				v := s.handlePrepare(txnID)
				votes <- voteResult{branch: t.branch, yes: v == "YES"}
			} else {
				resp, err := sendRecv(t.pc, fmt.Sprintf("%s PREPARE", txnID))
				votes <- voteResult{branch: t.branch, yes: err == nil && resp == "YES"}
			}
		}()
	}

	allYes := true
	for range targets {
		if v := <-votes; !v.yes {
			allYes = false
		}
	}

	// Phase 2 — COMMIT or ABORT (sequential, participants never block here)
	if allYes {
		for _, t := range targets {
			if t.pc == nil {
				s.handleCommit(txnID)
			} else {
				sendRecv(t.pc, fmt.Sprintf("%s COMMIT", txnID))
			}
		}
		return "COMMIT OK"
	}

	for _, t := range targets {
		if t.pc == nil {
			s.handleAbort(txnID)
		} else {
			sendRecv(t.pc, fmt.Sprintf("%s ABORT", txnID))
		}
	}
	return "ABORTED"
}
