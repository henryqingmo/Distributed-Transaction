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
	_ = os.Args[2] 

	_ = server.NewServer(branchID, server.ClusterConfig{})
}
