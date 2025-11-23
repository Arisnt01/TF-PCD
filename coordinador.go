package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type Stats struct {
	Users             int     `json:"users"`
	Movies            int     `json:"movies"`
	Ratings           int     `json:"ratings"`
	AvgRatingsPerUser float64 `json:"avg_ratings_per_user"`
	Node              string  `json:"node"`
}

func main() {
	// Define workers and partitions (ajusta si cambias puertos o rutas)
	workers := []struct {
		Addr      string
		Partition string
	}{
		{"localhost:9001", "ml-25m/ratings_part1.csv"},
		{"localhost:9002", "ml-25m/ratings_part2.csv"},
		{"localhost:9003", "ml-25m/ratings_part3.csv"},
		{"localhost:9004", "ml-25m/ratings_part4.csv"},
	}

	coordListen := flag.String("listen", ":9100", "coordinator listen address for results")
	flag.Parse()

	// Start listener to receive worker results
	ln, err := net.Listen("tcp", *coordListen)
	if err != nil {
		fmt.Println("Coordinator listen error:", err)
		return
	}
	defer ln.Close()
	fmt.Println("Coordinator listening for results on", *coordListen)

	var wg sync.WaitGroup
	resultsChan := make(chan Stats, len(workers))

	// Start goroutine to accept incoming results (one per worker)
	wg.Add(1)
	go func() {
		defer wg.Done()
		received := 0
		for received < len(workers) {
			conn, err := ln.Accept()
			if err != nil {
				fmt.Println("Accept error:", err)
				continue
			}
			go func(c net.Conn) {
				defer c.Close()
				var s Stats
				dec := json.NewDecoder(c)
				if err := dec.Decode(&s); err != nil {
					fmt.Println("Decode error:", err)
					return
				}
				fmt.Printf("Received result from worker (node=%s): users=%d movies=%d ratings=%d avg=%.2f\n",
					s.Node, s.Users, s.Movies, s.Ratings, s.AvgRatingsPerUser)
				resultsChan <- s
			}(conn)
			received++
		}
	}()

	// Give the listener a moment
	time.Sleep(300 * time.Millisecond)

	// Send PROCESS commands to each worker
	for _, w := range workers {
		sendProcessCommand(w.Addr, w.Partition, strings.TrimPrefix(*coordListen, ":"))
	}

	// Wait and aggregate results
	agg := Stats{}
	got := 0
	timeout := time.After(60 * time.Second)
	for {
		select {
		case s := <-resultsChan:
			agg.Users += s.Users
			agg.Movies += s.Movies // note: movies may overlap among partitions - we'll dedupe later if we wanted exact
			agg.Ratings += s.Ratings
			agg.AvgRatingsPerUser += s.AvgRatingsPerUser
			got++
			if got == len(workers) {
				// finalize
				agg.AvgRatingsPerUser = agg.AvgRatingsPerUser / float64(len(workers))
				fmt.Println("\n==== AGGREGATED STATS (simple sum/avg of partitions) ====")
				fmt.Printf("Total users (sum of partitions): %d\n", agg.Users)
				fmt.Printf("Total movies (sum of partitions): %d\n", agg.Movies)
				fmt.Printf("Total ratings: %d\n", agg.Ratings)
				fmt.Printf("Avg ratings per user (avg of partitions): %.2f\n", agg.AvgRatingsPerUser)
				// Done
				return
			}
		case <-timeout:
			fmt.Println("Timeout waiting for worker results")
			return
		}
	}
}

func sendProcessCommand(workerAddr, partition, coordPort string) {
	conn, err := net.Dial("tcp", workerAddr)
	if err != nil {
		fmt.Printf("Error connecting to worker %s: %v\n", workerAddr, err)
		return
	}
	defer conn.Close()
	// Command format: PROCESS;COORD=host:port
	cmd := fmt.Sprintf("PROCESS;COORD=localhost:%s\n", coordPort)
	// we could also include partition in the command, but workers here use the partition passed when launching
	fmt.Fprintf(conn, "%s", cmd)
	fmt.Printf("Sent PROCESS to %s (partition=%s)\n", workerAddr, partition)
}
