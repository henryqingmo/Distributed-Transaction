package server

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ParseConfig(filename string) (ClusterConfig, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := make(ClusterConfig)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("bad config line: %q", line)
		}
		cfg[fields[0]] = NodeInfo{BranchID: fields[0], Host: fields[1], Port: fields[2]}
	}
	return cfg, scanner.Err()
}

func NewServer(branchID string, cfg ClusterConfig) *Server {
	s := &Server{
		BranchID:          branchID,
		Config:            cfg,
		Accounts:          make(map[string]*Account),
		Locks:             make(map[string]*AccountLock),
		TentativeWrites:   make(map[string]map[string]int),
		Transactions:      make(map[string]*Transaction),
		Participants:      make(map[string]*ParticipantClient),
		participantSvc:    NewParticipantService(),
		coordinatorSvc:    NewCoordinatorService(),
	}
	return s
}
