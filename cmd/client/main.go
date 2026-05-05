package main

import (
	"bufio"
	"cs425_mp3/internal/server"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: client <id> <config>")
		os.Exit(1)
	}

	cfg, err := server.ParseConfig(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to parse config:", err)
		os.Exit(1)
	}

	nodes := make([]server.NodeInfo, 0, len(cfg))
	for _, n := range cfg {
		nodes = append(nodes, n)
	}

	stdin := bufio.NewScanner(os.Stdin)
	var conn net.Conn
	var srvWriter *bufio.Writer
	var srvScanner *bufio.Scanner

	for stdin.Scan() {
		line := strings.TrimSpace(stdin.Text())
		if line == "" {
			continue
		}
		cmd := strings.Fields(line)[0]

		// Ignore everything until BEGIN
		if conn == nil && cmd != "BEGIN" {
			continue
		}

		if cmd == "BEGIN" {
			node := nodes[rand.Intn(len(nodes))]
			conn, err = net.Dial("tcp", node.Host+":"+node.Port)
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to connect:", err)
				os.Exit(1)
			}
			srvWriter = bufio.NewWriter(conn)
			srvScanner = bufio.NewScanner(conn)
		}

		fmt.Fprintln(srvWriter, line)
		srvWriter.Flush()

		if !srvScanner.Scan() {
			fmt.Fprintln(os.Stderr, "connection closed unexpectedly")
			os.Exit(1)
		}
		resp := srvScanner.Text()
		fmt.Println(resp)

		// Transaction is over — exit
		if resp == "COMMIT OK" || resp == "ABORTED" || resp == "NOT FOUND, ABORTED" {
			conn.Close()
			return
		}
	}
}
