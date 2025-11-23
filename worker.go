package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"
)

// ============================================================================
// WORKER DISTRIBUIDO - ETAPA 4
// ============================================================================

type WorkerRating struct {
	UserID  int
	MovieID int
	Rating  float64
}

type WorkerUserRatings map[int]float64

type WorkerDataSet struct {
	UserRatingsMap map[int]WorkerUserRatings
	UserAvgRatings map[int]float64
	AllUserIDs     []int
	TotalRatings   int
	mu             sync.RWMutex
}

var (
	workerDataset *WorkerDataSet
	workerID      string
)

// Carga de datos de la partición
func LoadWorkerPartition(filepath string) (*WorkerDataSet, error) {
	log.Printf("[%s] Cargando partición: %s", workerID, filepath)

	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	ratings := make([]WorkerRating, 0)
	reader.Read() // Skip header

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

		ratings = append(ratings, WorkerRating{
			UserID:  userID,
			MovieID: movieID,
			Rating:  rating,
		})

		count++
		if count%500000 == 0 {
			log.Printf("[%s] Procesados: %d ratings...", workerID, count)
		}
	}

	log.Printf("[%s] Total ratings cargados: %d", workerID, count)

	// Construir dataset
	ds := &WorkerDataSet{
		UserRatingsMap: make(map[int]WorkerUserRatings),
		UserAvgRatings: make(map[int]float64),
		AllUserIDs:     make([]int, 0),
		TotalRatings:   count,
	}

	for _, r := range ratings {
		if ds.UserRatingsMap[r.UserID] == nil {
			ds.UserRatingsMap[r.UserID] = make(WorkerUserRatings)
		}
		ds.UserRatingsMap[r.UserID][r.MovieID] = r.Rating
	}

	// Calcular promedios
	for userID, userRatings := range ds.UserRatingsMap {
		sum := 0.0
		for _, rating := range userRatings {
			sum += rating
		}
		ds.UserAvgRatings[userID] = sum / float64(len(userRatings))
		ds.AllUserIDs = append(ds.AllUserIDs, userID)
	}

	log.Printf("[%s] Usuarios únicos: %d", workerID, len(ds.UserRatingsMap))
	return ds, nil
}

// Cálculo de similitud coseno
func CosineSimilarityWorker(vec1, vec2 WorkerUserRatings, avg1, avg2 float64) (float64, int) {
	commonMovies := make([]int, 0)
	for movieID := range vec1 {
		if _, exists := vec2[movieID]; exists {
			commonMovies = append(commonMovies, movieID)
		}
	}

	commonCount := len(commonMovies)
	if commonCount < 3 {
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

// Procesar solicitud de similitud
func ProcessSimilarityRequest(req SimilarityRequest) SimilarityResponse {
	startTime := time.Now()

	// Métricas de sistema
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memBefore := memStats.Alloc

	workerDataset.mu.RLock()
	defer workerDataset.mu.RUnlock()

	targetRatings := req.TargetRatings
	targetAvg := req.TargetAvg

	similarities := make([]SimilarityResult, 0)
	usersChecked := 0

	// Sampling de usuarios locales
	candidateUserIDs := workerDataset.AllUserIDs
	if len(candidateUserIDs) > req.SampleSize {
		step := len(candidateUserIDs) / req.SampleSize
		sampled := make([]int, 0, req.SampleSize)
		for i := 0; i < len(candidateUserIDs) && len(sampled) < req.SampleSize; i += step {
			sampled = append(sampled, candidateUserIDs[i])
		}
		candidateUserIDs = sampled
	}

	for _, userID := range candidateUserIDs {
		if userID == req.TargetUserID {
			continue
		}

		userRatings := workerDataset.UserRatingsMap[userID]
		userAvg := workerDataset.UserAvgRatings[userID]

		similarity, commonCount := CosineSimilarityWorker(targetRatings, userRatings, targetAvg, userAvg)
		usersChecked++

		if similarity > 0 && commonCount >= 3 {
			similarities = append(similarities, SimilarityResult{
				UserID:     userID,
				Similarity: similarity,
			})
		}
	}

	// Ordenar por similitud
	sort.Slice(similarities, func(i, j int) bool {
		return similarities[i].Similarity > similarities[j].Similarity
	})

	// Retornar top-k
	if len(similarities) > req.K {
		similarities = similarities[:req.K]
	}

	processTime := time.Since(startTime).Milliseconds()

	// Métricas finales
	runtime.ReadMemStats(&memStats)
	memAfter := memStats.Alloc
	memUsed := (memAfter - memBefore) / 1024 / 1024 // MB

	return SimilarityResponse{
		WorkerID:     workerID,
		Similarities: similarities,
		ProcessTime:  float64(processTime),
		UsersChecked: usersChecked,
		CPUUsage:     0.0,
		MemoryUsage:  memUsed,
	}
}

// Manejador de conexiones TCP
func handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req SimilarityRequest
	if err := decoder.Decode(&req); err != nil {
		log.Printf("[%s] Error decodificando solicitud: %v", workerID, err)
		return
	}

	log.Printf("[%s] Procesando solicitud para usuario %d", workerID, req.TargetUserID)

	response := ProcessSimilarityRequest(req)

	if err := encoder.Encode(response); err != nil {
		log.Printf("[%s] Error enviando respuesta: %v", workerID, err)
		return
	}

	log.Printf("[%s] Solicitud completada en %.2fms", workerID, response.ProcessTime)
}

// Servidor TCP del worker
func startWorkerServer(listenAddr string) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("[%s] Error iniciando servidor: %v", workerID, err)
	}
	defer listener.Close()

	log.Printf("[%s] Worker escuchando en %s", workerID, listenAddr)
	log.Printf("[%s] Dataset cargado: %d usuarios, %d ratings",
		workerID, len(workerDataset.UserRatingsMap), workerDataset.TotalRatings)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[%s] Error aceptando conexión: %v", workerID, err)
			continue
		}

		go handleConnection(conn)
	}
}

func main() {
	listenAddr := flag.String("listen", ":9001", "Dirección de escucha del worker (ej: :9001)")
	partitionFile := flag.String("partition", "", "Archivo de partición de datos")
	workerName := flag.String("name", "", "Nombre del worker")
	flag.Parse()

	if *partitionFile == "" {
		log.Fatal("Debe especificar un archivo de partición con --partition")
	}

	// Determinar ID del worker
	if *workerName != "" {
		workerID = *workerName
	} else {
		workerID = fmt.Sprintf("worker%s", *listenAddr)
	}

	// Cargar partición de datos
	var err error
	workerDataset, err = LoadWorkerPartition(*partitionFile)
	if err != nil {
		log.Fatalf("[%s] Error cargando partición: %v", workerID, err)
	}

	log.Printf("[%s] Inicializado correctamente", workerID)

	// Iniciar servidor TCP
	startWorkerServer(*listenAddr)
}
