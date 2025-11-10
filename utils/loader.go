package utils

import (
	"encoding/csv"
	"os"
	"strconv"
	"sync"
)

type Rating struct {
	UserID  int
	MovieID int
	Score   float64
}

// Carga concurrente del dataset en varios hilos
func LoadDatasetConcurrent(path string, workers int) []Rating {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, _ := reader.ReadAll()

	var wg sync.WaitGroup
	chunk := len(records) / workers
	results := make([][]Rating, workers)

	for i := 0; i < workers; i++ {
		start := i * chunk
		end := start + chunk
		if i == workers-1 {
			end = len(records)
		}

		wg.Add(1)
		go func(i, start, end int) {
			defer wg.Done()
			local := make([]Rating, 0, end-start)
			for _, r := range records[start:end] {
				user, _ := strconv.Atoi(r[0])
				movie, _ := strconv.Atoi(r[1])
				score, _ := strconv.ParseFloat(r[2], 64)
				local = append(local, Rating{user, movie, score})
			}
			results[i] = local
		}(i, start, end)
	}

	wg.Wait()

	// Combinar resultados
	var all []Rating
	for _, chunk := range results {
		all = append(all, chunk...)
	}
	return all
}
