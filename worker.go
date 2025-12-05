package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Stats struct {
	Users             int     `json:"users"`
	Movies            int     `json:"movies"`
	Ratings           int     `json:"ratings"`
	AvgRatingsPerUser float64 `json:"avg_ratings_per_user"`
	Node              string  `json:"node"`
}

func main() {
	listenAddr := flag.String("listen", ":9001", "worker listen address (e.g. :9001)")
	partition := flag.String("partition", "ml-25m/ratings_part1.csv", "path to partition CSV")
	flag.Parse()

	fmt.Printf("Worker listening on %s, partition=%s\n", *listenAddr, *partition)

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		fmt.Println("Listen error:", err)
		return
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}
		go handleConn(conn, *partition)
	}
}

func handleConn(conn net.Conn, partitionPath string) {
	defer conn.Close()
	// Read command line
	msg, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Println("Read command error:", err)
		return
	}
	msg = strings.TrimSpace(msg)
	// Expected format: PROCESS;COORD=host:port
	// Example: PROCESS;COORD=localhost:9100
	parts := strings.Split(msg, ";")
	if len(parts) < 1 || parts[0] != "PROCESS" {
		fmt.Println("Unknown command:", msg)
		return
	}
	coordAddr := "localhost:9100" // default
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "COORD=") {
			coordAddr = strings.TrimPrefix(p, "COORD=")
		}
	}

	fmt.Printf("Received PROCESS command. Will process %s and report to %s\n", partitionPath, coordAddr)

	stats, err := processPartition(partitionPath)
	if err != nil {
		fmt.Println("Processing error:", err)
		return
	}
	// add node identification (local listening addr)
	stats.Node = conn.LocalAddr().String()

	// send result to coordinator
	sendResult(coordAddr, stats)
	fmt.Printf("Finished processing %s, sent result to %s\n", partitionPath, coordAddr)
}

func processPartition(path string) (Stats, error) {
	file, err := os.Open(path)
	if err != nil {
		return Stats{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	users := make(map[int]struct{})
	movies := make(map[int]struct{})
	totalRatings := 0

	// Read header
	if scanner.Scan() {
		// skip header
	}
	for scanner.Scan() {
		line := scanner.Text()
		cols := strings.Split(line, ",")
		if len(cols) < 3 {
			continue
		}
		user, err1 := strconv.Atoi(strings.TrimSpace(cols[0]))
		movie, err2 := strconv.Atoi(strings.TrimSpace(cols[1]))
		_, err3 := strconv.ParseFloat(strings.TrimSpace(cols[2]), 64)
		if err1 != nil || err2 != nil || err3 != nil {
			// possible header or malformed line, skip
			continue
		}
		users[user] = struct{}{}
		movies[movie] = struct{}{}
		totalRatings++
	}

	avg := 0.0
	if len(users) > 0 {
		avg = float64(totalRatings) / float64(len(users))
	}

	return Stats{
		Users:             len(users),
		Movies:            len(movies),
		Ratings:           totalRatings,
		AvgRatingsPerUser: avg,
	}, nil
}

func sendResult(coordAddr string, stats Stats) {
	conn, err := net.Dial("tcp", coordAddr)
	if err != nil {
		fmt.Println("Dial coordinator error:", err)
		return
	}
	defer conn.Close()
	enc := json.NewEncoder(conn)
	if err := enc.Encode(stats); err != nil {
		fmt.Println("Encode/send error:", err)
	}
}
