package main

import (
	"cs425_mp3/internal/server"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: server <branch> <config>")
		os.Exit(1)
	}

	branchID := os.Args[1]
	configFile := os.Args[2]

	cfg, err := server.ParseConfig(configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to parse config:", err)
		os.Exit(1)
	}

	node, ok := cfg[branchID]
	if !ok {
		fmt.Fprintln(os.Stderr, "branch not found in config:", branchID)
		os.Exit(1)
	}

	s := server.NewServer(branchID, cfg)
	addr := ":" + node.Port
	if err := s.Listen(addr); err != nil {
		fmt.Fprintln(os.Stderr, "listen error:", err)
		os.Exit(1)
	}
}
