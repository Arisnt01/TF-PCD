package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type UserRatings map[string]float64
type Dataset map[string]UserRatings
type UserPair struct {
	UserA string
	UserB string
}

func jaccardIndex(ratingsA, ratingsB UserRatings) float64 {
	intersection := 0
	union := make(map[string]struct{})

	for item := range ratingsA {
		union[item] = struct{}{}
		if _, ok := ratingsB[item]; ok {
			intersection++
		}
	}
	for item := range ratingsB {
		union[item] = struct{}{}
	}

	if len(union) == 0 {
		return 0.0
	}
	return float64(intersection) / float64(len(union))
}

func loadRatings(filePath string) (Dataset, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data := make(Dataset)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "::")
		if len(parts) < 3 {
			continue
		}

		userID := parts[0]
		movieID := parts[1]
		rating, _ := strconv.ParseFloat(parts[2], 64)

		if _, ok := data[userID]; !ok {
			data[userID] = make(UserRatings)
		}
		data[userID][movieID] = rating
	}

	return data, scanner.Err()
}

func runSequential(data Dataset, userIDs []string) {
	for i := 0; i < len(userIDs); i++ {
		for j := i + 1; j < len(userIDs); j++ {
			_ = jaccardIndex(data[userIDs[i]], data[userIDs[j]])
		}
	}
}

func runConcurrent(data Dataset, userIDs []string, numWorkers int) {
	numJobs := (len(userIDs) * (len(userIDs) - 1)) / 2
	jobs := make(chan UserPair, numJobs)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pair := range jobs {
				_ = jaccardIndex(data[pair.UserA], data[pair.UserB])
			}
		}()
	}

	for i := 0; i < len(userIDs); i++ {
		for j := i + 1; j < len(userIDs); j++ {
			jobs <- UserPair{UserA: userIDs[i], UserB: userIDs[j]}
		}
	}
	close(jobs)
	wg.Wait()
}

func main() {
	filePath := "ratings.dat"

	data, err := loadRatings(filePath)
	if err != nil {
		fmt.Println("Error cargando dataset:", err)
		return
	}

	var userIDs []string
	for id := range data {
		userIDs = append(userIDs, id)
	}
	sort.Strings(userIDs)

	if len(userIDs) > 700 {
		userIDs = userIDs[:700]
	}
	fmt.Printf("Usando %d usuarios\n", len(userIDs))
	fmt.Printf("Total de pares únicos: %d\n\n", (len(userIDs)*(len(userIDs)-1))/2)

	start := time.Now()
	runSequential(data, userIDs)
	seqDuration := time.Since(start)
	fmt.Printf("Tiempo secuencial: %v\n", seqDuration)

	workersList := []int{1, runtime.NumCPU(), runtime.NumCPU() * 2, 32}

	file, _ := os.Create("resultados_jaccard.csv")
	writer := csv.NewWriter(file)
	defer file.Close()
	writer.Write([]string{"Workers", "T_secuencial_ms", "T_concurrente_ms", "Speedup"})

	fmt.Println("\nWorkers | T_sec (ms) | T_con (ms) | Speedup")
	fmt.Println("---------------------------------------------")

	for _, w := range workersList {
		start := time.Now()
		runConcurrent(data, userIDs, w)
		conDuration := time.Since(start)

		speedup := float64(seqDuration) / float64(conDuration)
		fmt.Printf("%7d | %10d | %10d | %.2fx\n",
			w, seqDuration.Milliseconds(), conDuration.Milliseconds(), speedup)

		writer.Write([]string{
			fmt.Sprintf("%d", w),
			fmt.Sprintf("%d", seqDuration.Milliseconds()),
			fmt.Sprintf("%d", conDuration.Milliseconds()),
			fmt.Sprintf("%.2f", speedup),
		})
	}
	writer.Flush()

	fmt.Println("\nResultados guardados en 'resultados_jaccard.csv'.")
}
