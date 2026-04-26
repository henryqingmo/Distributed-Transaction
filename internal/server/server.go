package server

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
