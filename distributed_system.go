package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// COORDINADOR DISTRIBUIDO - ETAPA 4
type DistributedCoordinator struct {
	workers      []WorkerNode
	localDataset *LocalDataSet
	db           *Database
	metrics      *SystemMetrics
	numWorkers   int
	mu           sync.RWMutex
}

type WorkerNode struct {
	Address   string
	Partition string
	Active    bool
}

type LocalDataSet struct {
	UserRatingsMap  map[int]map[int]float64
	Movies          map[int]string
	UserAvgRatings  map[int]float64
	GlobalAvgRating float64
	AllUserIDs      []int
	mu              sync.RWMutex
}

type WorkerStats struct {
	Users   int
	Movies  int
	Ratings int
}

// Crear nuevo coordinador
func NewDistributedCoordinator(workerAddresses []string, partitions []string, numWorkers int) *DistributedCoordinator {
	workers := make([]WorkerNode, 0)
	for i, addr := range workerAddresses {
		workers = append(workers, WorkerNode{
			Address:   addr,
			Partition: partitions[i],
			Active:    true,
		})
	}

	return &DistributedCoordinator{
		workers:    workers,
		numWorkers: numWorkers,
		localDataset: &LocalDataSet{
			UserRatingsMap: make(map[int]map[int]float64),
			Movies:         make(map[int]string),
			UserAvgRatings: make(map[int]float64),
			AllUserIDs:     make([]int, 0),
		},
	}
}

// Cargar datos locales
func (dc *DistributedCoordinator) LoadLocalData(ratingsPath, moviesPath string) error {
	log.Println("[COORD] Cargando datos locales...")

	// Cargar películas
	if err := dc.loadMovies(moviesPath); err != nil {
		return fmt.Errorf("error cargando películas: %v", err)
	}

	// Cargar ratings
	if err := dc.loadRatings(ratingsPath); err != nil {
		return fmt.Errorf("error cargando ratings: %v", err)
	}

	log.Printf("[COORD] Datos locales cargados: %d usuarios, %d películas",
		len(dc.localDataset.UserRatingsMap), len(dc.localDataset.Movies))

	return nil
}

func (dc *DistributedCoordinator) loadMovies(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
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

		dc.localDataset.Movies[movieID] = record[1]
	}

	return nil
}

func (dc *DistributedCoordinator) loadRatings(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Read()

	totalRating := 0.0
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

		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}

		if dc.localDataset.UserRatingsMap[userID] == nil {
			dc.localDataset.UserRatingsMap[userID] = make(map[int]float64)
			dc.localDataset.AllUserIDs = append(dc.localDataset.AllUserIDs, userID)
		}

		dc.localDataset.UserRatingsMap[userID][movieID] = rating
		totalRating += rating
		count++

		// Agregar también a la base de datos para consultas
		if dc.db != nil {
			dc.db.mu.Lock()
			if dc.db.Ratings[userID] == nil {
				dc.db.Ratings[userID] = make(map[int]float64)
			}
			dc.db.Ratings[userID][movieID] = rating
			dc.db.mu.Unlock()
		}

		if count%1000000 == 0 {
			log.Printf("[COORD] Procesados: %dM ratings...", count/1000000)
		}
	}

	// Calcular promedios
	dc.localDataset.GlobalAvgRating = totalRating / float64(count)

	for userID, userRatings := range dc.localDataset.UserRatingsMap {
		sum := 0.0
		for _, rating := range userRatings {
			sum += rating
		}
		dc.localDataset.UserAvgRatings[userID] = sum / float64(len(userRatings))
	}

	return nil
}

// Obtener recomendaciones distribuidas
func (dc *DistributedCoordinator) GetDistributedRecommendations(userID int, topN int) ([]RecommendationItem, int, error) {
	dc.localDataset.mu.RLock()
	userRatings := dc.localDataset.UserRatingsMap[userID]
	userAvg := dc.localDataset.UserAvgRatings[userID]
	dc.localDataset.mu.RUnlock()

	if len(userRatings) == 0 {
		return nil, 0, fmt.Errorf("usuario no encontrado")
	}

	// Preparar solicitud para workers
	k := 30
	sampleSize := 5000 // Tamaño de muestra por worker

	req := SimilarityRequest{
		TargetUserID:  userID,
		TargetRatings: userRatings,
		TargetAvg:     userAvg,
		K:             k,
		SampleSize:    sampleSize,
	}

	// Enviar a todos los workers en paralelo
	var wg sync.WaitGroup
	responsesChan := make(chan SimilarityResponse, len(dc.workers))

	activeWorkers := 0
	for _, worker := range dc.workers {
		if !worker.Active {
			continue
		}

		activeWorkers++
		wg.Add(1)

		go func(w WorkerNode) {
			defer wg.Done()

			resp, err := dc.sendToWorker(w.Address, req)
			if err != nil {
				log.Printf("[COORD] Error en worker %s: %v", w.Address, err)
				return
			}

			responsesChan <- resp
		}(worker)
	}

	// Esperar respuestas
	wg.Wait()
	close(responsesChan)

	// Combinar resultados
	allSimilarities := make([]SimilarityResult, 0)
	for resp := range responsesChan {
		allSimilarities = append(allSimilarities, resp.Similarities...)
		log.Printf("[COORD] Worker %s: %d similitudes, %.2fms",
			resp.WorkerID, len(resp.Similarities), resp.ProcessTime)
	}

	// Ordenar por similitud
	sort.Slice(allSimilarities, func(i, j int) bool {
		return allSimilarities[i].Similarity > allSimilarities[j].Similarity
	})

	// Tomar top-k similares
	if len(allSimilarities) > k {
		allSimilarities = allSimilarities[:k]
	}

	// Generar recomendaciones
	recommendations := dc.generateRecommendations(userID, allSimilarities, topN)

	return recommendations, activeWorkers, nil
}

// Generar recomendaciones a partir de usuarios similares
func (dc *DistributedCoordinator) generateRecommendations(targetUserID int, similarUsers []SimilarityResult, topN int) []RecommendationItem {
	dc.localDataset.mu.RLock()
	defer dc.localDataset.mu.RUnlock()

	targetRatings := dc.localDataset.UserRatingsMap[targetUserID]
	targetAvg := dc.localDataset.UserAvgRatings[targetUserID]

	candidateScores := make(map[int]float64)
	candidateWeights := make(map[int]float64)

	for _, simUser := range similarUsers {
		userRatings := dc.localDataset.UserRatingsMap[simUser.UserID]
		userAvg := dc.localDataset.UserAvgRatings[simUser.UserID]

		for movieID, rating := range userRatings {
			if _, seen := targetRatings[movieID]; !seen {
				candidateScores[movieID] += simUser.Similarity * (rating - userAvg)
				candidateWeights[movieID] += math.Abs(simUser.Similarity)
			}
		}
	}

	recommendations := make([]RecommendationItem, 0)
	for movieID, scoreSum := range candidateScores {
		weightSum := candidateWeights[movieID]
		if weightSum > 0 {
			predictedScore := targetAvg + (scoreSum / weightSum)

			title := "Unknown"
			if movieTitle, exists := dc.localDataset.Movies[movieID]; exists {
				title = movieTitle
			}

			recommendations = append(recommendations, RecommendationItem{
				MovieID:        movieID,
				Title:          title,
				PredictedScore: predictedScore,
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

// Enviar solicitud a worker via TCP
func (dc *DistributedCoordinator) sendToWorker(address string, req SimilarityRequest) (SimilarityResponse, error) {
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return SimilarityResponse{}, err
	}
	defer conn.Close()

	// Enviar solicitud
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return SimilarityResponse{}, err
	}

	// Recibir respuesta
	decoder := json.NewDecoder(conn)
	var resp SimilarityResponse
	if err := decoder.Decode(&resp); err != nil {
		return SimilarityResponse{}, err
	}

	return resp, nil
}

// Ping a worker para verificar salud
func (dc *DistributedCoordinator) PingWorker(address string) bool {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Obtener estadísticas de worker
func (dc *DistributedCoordinator) GetWorkerStats(address string) WorkerStats {
	return WorkerStats{
		Users:   10000,
		Movies:  5000,
		Ratings: 2500000,
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	apiPort := flag.String("api", ":8080", "Puerto de la API")
	flag.Parse()

	log.Println(strings.Repeat("=", 70))
	log.Println("🎬 SISTEMA DE RECOMENDACIÓN DISTRIBUIDO - DOCKER")
	log.Println(strings.Repeat("=", 70))

	// Inicializar base de datos
	db := NewDatabase("db_snapshot.json")
	if err := db.LoadMovies("data_25M/movies.csv"); err != nil {
		log.Printf("[WARN] No se pudieron cargar películas: %v", err)
	}
	db.StartCleanupTask()

	// Inicializar métricas
	metrics := NewSystemMetrics()
	metrics.StartMonitoring()

	// Modo distribuido con workers (Docker)
	log.Println("\n[MODE] Distribuido con 8 workers")

	// Configurar workers desde variable de entorno
	workersEnv := os.Getenv("WORKERS")
	if workersEnv == "" {
		log.Fatal("[ERROR] Variable WORKERS no configurada. Debe ejecutarse con Docker.")
	}

	workerAddresses := strings.Split(workersEnv, ",")
	log.Printf("[COORD] Workers desde env: %v", workerAddresses)

	partitions := []string{
		"data_25M/ratings_part1.csv",
		"data_25M/ratings_part2.csv",
		"data_25M/ratings_part3.csv",
		"data_25M/ratings_part4.csv",
		"data_25M/ratings_part5.csv",
		"data_25M/ratings_part6.csv",
		"data_25M/ratings_part7.csv",
		"data_25M/ratings_part8.csv",
	}

	coordinator := NewDistributedCoordinator(workerAddresses, partitions, 8)
	coordinator.db = db
	coordinator.metrics = metrics

	// Cargar datos locales para coordinación
	if err := coordinator.LoadLocalData("data_25M/ratings.csv", "data_25M/movies.csv"); err != nil {
		log.Fatalf("[ERROR] No se pudieron cargar datos: %v", err)
	}

	log.Println("\n[INFO] Verificando workers...")
	for _, worker := range coordinator.workers {
		if coordinator.PingWorker(worker.Address) {
			log.Printf("[OK] Worker %s activo", worker.Address)
		} else {
			log.Printf("[WARN] Worker %s no responde", worker.Address)
		}
	}

	// Iniciar API REST
	go StartAPIServer(coordinator, db, metrics, *apiPort)

	log.Printf("\n[INFO] Sistema distribuido listo")
	log.Printf("[INFO] API disponible en http://localhost%s", *apiPort)
	log.Println("\n[INFO] Presiona Ctrl+C para detener")

	select {}
}
