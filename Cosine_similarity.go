package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ESTRUCTURAS DE DATOS
type Rating struct {
	UserID  int
	MovieID int
	Rating  float64
}

type Movie struct {
	MovieID int
	Title   string
}

type UserRatings map[int]float64

type SimilarityResult struct {
	ID         int
	Similarity float64
}

type Recommendation struct {
	MovieID        int
	PredictedScore float64
	Title          string
}

type DataSet struct {
	UserRatingsMap  map[int]UserRatings
	Movies          map[int]Movie
	UserAvgRatings  map[int]float64
	GlobalAvgRating float64
	TotalRatings    int
	AllUserIDs      []int
}

// ETAPA 1: CARGA Y LIMPIEZA DE DATOS
func LoadMovies(filepath string) (map[int]Movie, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	movies := make(map[int]Movie)
	reader.Read()

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			continue
		}

		movieID, err := strconv.Atoi(record[0])
		if err != nil {
			continue
		}

		movies[movieID] = Movie{
			MovieID: movieID,
			Title:   record[1],
		}
	}

	return movies, nil
}

func LoadRatings(filepath string) ([]Rating, error) {
	fmt.Println(" Cargando ratings...")

	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	ratings := make([]Rating, 0, 20000000)
	reader.Read()

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 3 {
			continue
		}

		userID, err1 := strconv.Atoi(record[0])
		movieID, err2 := strconv.Atoi(record[1])
		rating, err3 := strconv.ParseFloat(record[2], 64)

		if err1 != nil || err2 != nil || err3 != nil || rating < 0 || rating > 5 {
			continue
		}

		ratings = append(ratings, Rating{
			UserID:  userID,
			MovieID: movieID,
			Rating:  rating,
		})

		count++
		if count%2000000 == 0 {
			fmt.Printf("  Procesados: %dM ratings...\n", count/1000000)
		}
	}

	fmt.Printf("✓ Total ratings: %d\n", count)
	return ratings, nil
}

func BuildMatrices(ratings []Rating) *DataSet {
	fmt.Println(" Construyendo matrices...")

	ds := &DataSet{
		UserRatingsMap: make(map[int]UserRatings),
		UserAvgRatings: make(map[int]float64),
		AllUserIDs:     make([]int, 0),
	}

	for _, r := range ratings {
		if ds.UserRatingsMap[r.UserID] == nil {
			ds.UserRatingsMap[r.UserID] = make(UserRatings)
		}
		ds.UserRatingsMap[r.UserID][r.MovieID] = r.Rating
	}

	ds.TotalRatings = len(ratings)

	// Calcular promedios y crear lista de IDs
	totalRating := 0.0
	for userID, userRatings := range ds.UserRatingsMap {
		sum := 0.0
		for _, rating := range userRatings {
			sum += rating
			totalRating += rating
		}
		ds.UserAvgRatings[userID] = sum / float64(len(userRatings))
		ds.AllUserIDs = append(ds.AllUserIDs, userID)
	}

	ds.GlobalAvgRating = totalRating / float64(ds.TotalRatings)

	fmt.Printf("✓ Usuarios: %d | Ratings: %d | Promedio: %.3f\n",
		len(ds.UserRatingsMap), ds.TotalRatings, ds.GlobalAvgRating)

	return ds
}

// ETAPA 2: FILTRADO COLABORATIVO OPTIMIZADO
func CosineSimilarity(vec1, vec2 UserRatings, avg1, avg2 float64) (float64, int) {
	commonMovies := make([]int, 0)
	for movieID := range vec1 {
		if _, exists := vec2[movieID]; exists {
			commonMovies = append(commonMovies, movieID)
		}
	}

	commonCount := len(commonMovies)
	if commonCount < 3 { // se busca si tienen al menos 3 peliculas en comun
		return 0.0, commonCount
	}

	dotProduct := 0.0
	norm1 := 0.0
	norm2 := 0.0

	for _, movieID := range commonMovies {
		r1 := vec1[movieID] - avg1
		r2 := vec2[movieID] - avg2

		dotProduct += r1 * r2
		norm1 += r1 * r1
		norm2 += r2 * r2
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0, commonCount
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2)), commonCount
}

func FindSimilarUsers(targetUserID int, ds *DataSet, k int, sampleSize int) []SimilarityResult {
	targetRatings := ds.UserRatingsMap[targetUserID]
	targetAvg := ds.UserAvgRatings[targetUserID]

	// OPTIMIZACIÓN: Sampling de usuarios
	candidateUserIDs := make([]int, 0, sampleSize)
	if len(ds.AllUserIDs) <= sampleSize {
		candidateUserIDs = ds.AllUserIDs
	} else {
		// Sampling aleatorio
		perm := rand.Perm(len(ds.AllUserIDs))
		for i := 0; i < sampleSize && i < len(perm); i++ {
			candidateUserIDs = append(candidateUserIDs, ds.AllUserIDs[perm[i]])
		}
	}

	similarities := make([]SimilarityResult, 0)

	for _, userID := range candidateUserIDs {
		if userID == targetUserID {
			continue
		}

		userRatings := ds.UserRatingsMap[userID]
		userAvg := ds.UserAvgRatings[userID]
		similarity, commonCount := CosineSimilarity(targetRatings, userRatings, targetAvg, userAvg)

		if similarity > 0 && commonCount >= 3 {
			similarities = append(similarities, SimilarityResult{
				ID:         userID,
				Similarity: similarity,
			})
		}
	}

	sort.Slice(similarities, func(i, j int) bool {
		return similarities[i].Similarity > similarities[j].Similarity
	})

	if len(similarities) > k {
		return similarities[:k]
	}
	return similarities
}

func GenerateRecommendations(targetUserID int, similarUsers []SimilarityResult, ds *DataSet, topN int) []Recommendation {
	targetRatings := ds.UserRatingsMap[targetUserID]
	targetAvg := ds.UserAvgRatings[targetUserID]

	candidateScores := make(map[int]float64)
	candidateWeights := make(map[int]float64)

	for _, simUser := range similarUsers {
		userRatings := ds.UserRatingsMap[simUser.ID]
		userAvg := ds.UserAvgRatings[simUser.ID]

		for movieID, rating := range userRatings {
			if _, seen := targetRatings[movieID]; !seen {
				candidateScores[movieID] += simUser.Similarity * (rating - userAvg)
				candidateWeights[movieID] += math.Abs(simUser.Similarity)
			}
		}
	}

	recommendations := make([]Recommendation, 0)
	for movieID, scoreSum := range candidateScores {
		weightSum := candidateWeights[movieID]
		if weightSum > 0 {
			predictedScore := targetAvg + (scoreSum / weightSum)

			title := "Unknown"
			if movie, exists := ds.Movies[movieID]; exists {
				title = movie.Title
			}

			recommendations = append(recommendations, Recommendation{
				MovieID:        movieID,
				PredictedScore: predictedScore,
				Title:          title,
			})
		}
	}

	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].PredictedScore > recommendations[j].PredictedScore
	})

	if len(recommendations) > topN {
		return recommendations[:topN]
	}
	return recommendations
}

// ETAPA 3: PARALELIZACIÓN CON GOROUTINES Y CHANNELS
type ParallelJob struct {
	TargetUserID int
	K            int
	SampleSize   int
}

type ParallelResult struct {
	UserID       int
	SimilarUsers []SimilarityResult
	Duration     time.Duration
}

func SimilarityWorker(id int, jobs <-chan ParallelJob, results chan<- ParallelResult, ds *DataSet, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		start := time.Now()
		similarUsers := FindSimilarUsers(job.TargetUserID, ds, job.K, job.SampleSize)
		duration := time.Since(start)

		results <- ParallelResult{
			UserID:       job.TargetUserID,
			SimilarUsers: similarUsers,
			Duration:     duration,
		}
	}
}

func ParallelRecommendations(userIDs []int, ds *DataSet, k int, topN int, sampleSize int, numWorkers int) (map[int][]Recommendation, []time.Duration) {
	jobs := make(chan ParallelJob, len(userIDs))
	results := make(chan ParallelResult, len(userIDs))

	var wg sync.WaitGroup

	// Iniciar workers (goroutines)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go SimilarityWorker(i, jobs, results, ds, &wg)
	}

	// Enviar trabajos al canal
	for _, userID := range userIDs {
		jobs <- ParallelJob{
			TargetUserID: userID,
			K:            k,
			SampleSize:   sampleSize,
		}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	// Recolectar resultados del canal
	recommendations := make(map[int][]Recommendation)
	durations := make([]time.Duration, 0)

	for result := range results {
		recs := GenerateRecommendations(result.UserID, result.SimilarUsers, ds, topN)
		recommendations[result.UserID] = recs
		durations = append(durations, result.Duration)
	}

	return recommendations, durations
}

func MeasureSpeedup(userIDs []int, ds *DataSet, k int, topN int, sampleSize int) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println(" MEDICIÓN DE SPEEDUP Y ESCALABILIDAD")
	fmt.Println(strings.Repeat("=", 70))

	workerCounts := []int{1, 2, 4, 8, 16}
	results := make(map[int]time.Duration)

	for _, numWorkers := range workerCounts {
		fmt.Printf("\n Workers: %d\n", numWorkers)
		start := time.Now()
		_, _ = ParallelRecommendations(userIDs, ds, k, topN, sampleSize, numWorkers)
		duration := time.Since(start)
		results[numWorkers] = duration
		fmt.Printf("   Tiempo: %v\n", duration)
	}

	fmt.Println("\n RESULTADOS DE SPEEDUP:")
	fmt.Println(strings.Repeat("-", 70))
	baseTime := results[1].Seconds()

	for _, numWorkers := range workerCounts {
		duration := results[numWorkers]
		speedup := baseTime / duration.Seconds()
		efficiency := speedup / float64(numWorkers) * 100

		fmt.Printf("Workers: %2d | Tiempo: %7.2fs | Speedup: %.2fx | Eficiencia: %.1f%%\n",
			numWorkers, duration.Seconds(), speedup, efficiency)
	}

	fmt.Println("\n GRÁFICO DE SPEEDUP:")
	PrintSpeedupChart(results, workerCounts)
}

func PrintSpeedupChart(results map[int]time.Duration, workerCounts []int) {
	baseTime := results[1].Seconds()
	maxSpeedup := 0.0

	speedups := make(map[int]float64)
	for _, w := range workerCounts {
		speedup := baseTime / results[w].Seconds()
		speedups[w] = speedup
		if speedup > maxSpeedup {
			maxSpeedup = speedup
		}
	}

	fmt.Println(strings.Repeat("-", 70))

	for _, w := range workerCounts {
		speedup := speedups[w]
		barLength := int(speedup / maxSpeedup * 50)
		bar := strings.Repeat("█", barLength)
		fmt.Printf("%2d workers | %-50s | %.2fx\n", w, bar, speedup)
	}

	fmt.Println(strings.Repeat("-", 70))
}

func PrintRecommendations(userID int, recs []Recommendation, userRatings UserRatings, ds *DataSet) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf(" RECOMENDACIONES PARA USUARIO %d\n", userID)
	fmt.Println(strings.Repeat("=", 70))

	type UserMovie struct {
		MovieID int
		Rating  float64
	}

	userMovies := make([]UserMovie, 0)
	for movieID, rating := range userRatings {
		userMovies = append(userMovies, UserMovie{movieID, rating})
	}

	sort.Slice(userMovies, func(i, j int) bool {
		return userMovies[i].Rating > userMovies[j].Rating
	})

	shown := 0
	for _, um := range userMovies {
		if shown >= 5 {
			break
		}
		if movie, exists := ds.Movies[um.MovieID]; exists {
			fmt.Printf("    %.1f - %s\n", um.Rating, movie.Title)
			shown++
		}
	}

	fmt.Println("\n Top 10 Recomendaciones:")
	fmt.Println(strings.Repeat("-", 70))

	for i, rec := range recs {
		if i >= 10 {
			break
		}
		fmt.Printf("%2d. [Score: %.2f] %s\n", i+1, rec.PredictedScore, rec.Title)
	}
}

// FUNCIÓN PRINCIPAL
func main() {
	rand.Seed(time.Now().UnixNano())

	startTime := time.Now()

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("ETAPA 1: CARGA Y ANÁLISIS DE DATOS")
	fmt.Println(strings.Repeat("=", 70))

	movies, err := LoadMovies("data_20M/movies.csv")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf(" Películas cargadas: %d\n", len(movies))

	ratings, err := LoadRatings("data_20M/ratings.csv")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	ds := BuildMatrices(ratings)
	ds.Movies = movies

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("ETAPA 2: FILTRADO COLABORATIVO (USER-BASED + k-NN)")
	fmt.Println(strings.Repeat("=", 70))

	// Seleccionar usuarios para pruebas
	testUserIDs := make([]int, 0)
	for userID, userRatings := range ds.UserRatingsMap {
		if len(userRatings) >= 30 && len(userRatings) <= 300 {
			testUserIDs = append(testUserIDs, userID)
			if len(testUserIDs) >= 2000 {
				break
			}
		}
	}

	fmt.Printf("\n Usuarios de prueba: %d\n", len(testUserIDs))

	exampleUserID := testUserIDs[0]
	k := 30
	topN := 10
	sampleSize := 20000

	fmt.Printf("\n Calculando similitudes para usuario %d\n", exampleUserID)
	similarUsers := FindSimilarUsers(exampleUserID, ds, k, sampleSize)

	fmt.Printf("\n Top 5 usuarios más similares:\n")
	for i := 0; i < 5 && i < len(similarUsers); i++ {
		fmt.Printf("   %d. Usuario %d - Similitud: %.4f\n", i+1, similarUsers[i].ID, similarUsers[i].Similarity)
	}

	recommendations := GenerateRecommendations(exampleUserID, similarUsers, ds, topN)
	userRatings := ds.UserRatingsMap[exampleUserID]
	PrintRecommendations(exampleUserID, recommendations, userRatings, ds)

	// ETAPA 3: PARALELIZACIÓN
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("ETAPA 3: PARALELIZACIÓN CON GOROUTINES Y CHANNELS")
	fmt.Println(strings.Repeat("=", 70))

	MeasureSpeedup(testUserIDs, ds, k, topN, sampleSize)

	totalDuration := time.Since(startTime)

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("Tiempo total: %v\n", totalDuration)
	fmt.Println(strings.Repeat("=", 70))
}
