package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// ============================================
// ESTRUCTURAS Y UTILIDADES
// ============================================

func openCSV(path string) *csv.Reader {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Error abriendo %s: %v", path, err)
	}
	reader := csv.NewReader(bufio.NewReader(f))
	reader.FieldsPerRecord = -1
	return reader
}

func createWriter(path string) *csv.Writer {
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("Error creando %s: %v", path, err)
	}
	return csv.NewWriter(f)
}

// ============================================
// CARGA DE REFERENCIAS
// ============================================

func loadMovies() map[int]bool {
	reader := openCSV("data_25M/movies.csv")
	reader.Read()
	movies := make(map[int]bool)
	for {
		rec, err := reader.Read()
		if err != nil {
			break
		}
		id, err := strconv.Atoi(rec[0])
		if err == nil {
			movies[id] = true
		}
	}
	return movies
}

func loadTags() map[int]bool {
	reader := openCSV("data_25M/genome-tags.csv")
	reader.Read()
	tags := make(map[int]bool)
	for {
		rec, err := reader.Read()
		if err != nil {
			break
		}
		id, err := strconv.Atoi(rec[0])
		if err == nil {
			tags[id] = true
		}
	}
	return tags
}

// ============================================
// FUNCIONES DE LIMPIEZA
// ============================================

func cleanMovies() {
	reader := openCSV("data_25M/movies.csv")
	reader.Read()

	w := createWriter("output/clean/movies_clean.csv")
	defer w.Flush()
	w.Write([]string{"movieId", "title", "genres"})

	for {
		rec, err := reader.Read()
		if err != nil {
			break
		}
		if len(rec) < 3 || rec[0] == "" || rec[1] == "" {
			continue
		}
		w.Write(rec)
	}
	fmt.Println("✅ movies.csv limpiado")
}

func cleanLinks(validMovies map[int]bool) {
	reader := openCSV("data_25M/links.csv")
	reader.Read()

	w := createWriter("output/clean/links_clean.csv")
	defer w.Flush()
	w.Write([]string{"movieId", "imdbId", "tmdbId"})

	for {
		rec, err := reader.Read()
		if err != nil {
			break
		}
		if len(rec) != 3 {
			continue
		}
		id, err := strconv.Atoi(rec[0])
		if err != nil || !validMovies[id] {
			continue
		}
		w.Write(rec)
	}
	fmt.Println("✅ links.csv limpiado")
}

func cleanTags(validMovies map[int]bool) {
	reader := openCSV("data_25M/tags.csv")
	reader.Read()

	w := createWriter("output/clean/tags_clean.csv")
	defer w.Flush()
	w.Write([]string{"userId", "movieId", "tag", "timestamp"})

	for {
		rec, err := reader.Read()
		if err != nil {
			break
		}
		if len(rec) != 4 {
			continue
		}
		movieId, err := strconv.Atoi(rec[1])
		if err != nil || !validMovies[movieId] {
			continue
		}
		if strings.TrimSpace(rec[2]) == "" {
			continue
		}
		w.Write(rec)
	}
	fmt.Println("✅ tags.csv limpiado")
}

// ============================================
// PROCESAMIENTO CONCURRENTE
// ============================================

type job struct {
	record []string
}

func workerRatings(id int, jobs <-chan job, results chan<- []string, validMovies map[int]bool, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs {
		rec := j.record
		if len(rec) != 4 {
			continue
		}
		userId, err1 := strconv.Atoi(rec[0])
		movieId, err2 := strconv.Atoi(rec[1])
		rating, err3 := strconv.ParseFloat(rec[2], 64)
		_, err4 := strconv.ParseInt(rec[3], 10, 64)

		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}
		if rating < 0.5 || rating > 5.0 {
			continue
		}
		if !validMovies[movieId] || userId <= 0 {
			continue
		}
		results <- rec
	}
}

func cleanRatingsConcurrent(validMovies map[int]bool) {
	reader := openCSV("data_25M/ratings.csv")
	reader.Read()

	outPath := "output/clean/ratings_clean.csv"
	w := createWriter(outPath)
	defer w.Flush()
	w.Write([]string{"userId", "movieId", "rating", "timestamp"})

	numWorkers := runtime.NumCPU()
	jobs := make(chan job, numWorkers*4)
	results := make(chan []string, numWorkers*4)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go workerRatings(i, jobs, results, validMovies, &wg)
	}

	// escritor separado
	var writerWG sync.WaitGroup
	writerWG.Add(1)
	go func() {
		defer writerWG.Done()
		count := 0
		for rec := range results {
			w.Write(rec)
			count++
			if count%1000000 == 0 {
				fmt.Printf("[ratings] %d registros válidos\n", count)
			}
		}
	}()

	// lectura
	for {
		rec, err := reader.Read()
		if err != nil {
			break
		}
		jobs <- job{rec}
	}
	close(jobs)

	wg.Wait()
	close(results)
	writerWG.Wait()

	fmt.Println("✅ ratings.csv limpiado (concurrencia activa)")
}

func workerGenomeScores(id int, jobs <-chan job, results chan<- []string, validMovies, validTags map[int]bool, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs {
		rec := j.record
		if len(rec) != 3 {
			continue
		}
		movieId, err1 := strconv.Atoi(rec[0])
		tagId, err2 := strconv.Atoi(rec[1])
		relevance, err3 := strconv.ParseFloat(rec[2], 64)

		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		if !validMovies[movieId] || !validTags[tagId] {
			continue
		}
		if relevance < 0 || relevance > 1 {
			continue
		}
		results <- rec
	}
}

func cleanGenomeScoresConcurrent(validMovies, validTags map[int]bool) {
	reader := openCSV("data_25M/genome-scores.csv")
	reader.Read()

	outPath := "output/clean/genome_scores_clean.csv"
	w := createWriter(outPath)
	defer w.Flush()
	w.Write([]string{"movieId", "tagId", "relevance"})

	numWorkers := runtime.NumCPU()
	jobs := make(chan job, numWorkers*4)
	results := make(chan []string, numWorkers*4)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go workerGenomeScores(i, jobs, results, validMovies, validTags, &wg)
	}

	// escritor separado
	var writerWG sync.WaitGroup
	writerWG.Add(1)
	go func() {
		defer writerWG.Done()
		count := 0
		for rec := range results {
			w.Write(rec)
			count++
			if count%1000000 == 0 {
				fmt.Printf("[genome-scores] %d registros válidos\n", count)
			}
		}
	}()

	// lectura
	for {
		rec, err := reader.Read()
		if err != nil {
			break
		}
		jobs <- job{rec}
	}
	close(jobs)

	wg.Wait()
	close(results)
	writerWG.Wait()

	fmt.Println("✅ genome-scores.csv limpiado (concurrencia activa)")
}

// ============================================
// MAIN
// ============================================

func main() {
	os.MkdirAll(filepath.Join("output", "clean"), os.ModePerm)

	fmt.Println("Cargando referencias...")
	validMovies := loadMovies()
	validTags := loadTags()
	fmt.Printf("Películas válidas: %d | Tags válidos: %d\n\n", len(validMovies), len(validTags))

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		cleanMovies()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		cleanLinks(validMovies)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		cleanTags(validMovies)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		cleanRatingsConcurrent(validMovies)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		cleanGenomeScoresConcurrent(validMovies, validTags)
	}()

	wg.Wait()
	fmt.Println("\n🎉 Limpieza concurrente completada. Archivos en /output/clean/")
}
