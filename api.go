package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type APIServer struct {
	coordinator *DistributedCoordinator
	db          *Database
	metrics     *SystemMetrics
	mu          sync.RWMutex
}

type RecommendationAPIRequest struct {
	UserID int `json:"user_id"`
	TopN   int `json:"top_n"`
}

type RecommendationAPIResponse struct {
	UserID          int                  `json:"user_id"`
	Recommendations []RecommendationItem `json:"recommendations"`
	ProcessTimeMS   float64              `json:"process_time_ms"`
	NodesUsed       int                  `json:"nodes_used"`
	CacheHit        bool                 `json:"cache_hit"`
	Metrics         APIMetrics           `json:"metrics"`
}

type RecommendationItem struct {
	MovieID        int     `json:"movie_id"`
	Title          string  `json:"title"`
	PredictedScore float64 `json:"predicted_score"`
}

type APIMetrics struct {
	TotalCPU    float64 `json:"total_cpu_percent"`
	TotalMemory uint64  `json:"total_memory_mb"`
	Speedup     float64 `json:"speedup"`
}

type HealthResponse struct {
	Status    string              `json:"status"`
	Timestamp string              `json:"timestamp"`
	Workers   []WorkerHealthInfo  `json:"workers"`
	Database  DatabaseHealthInfo  `json:"database"`
	Metrics   SystemHealthMetrics `json:"metrics"`
}

type WorkerHealthInfo struct {
	Address string  `json:"address"`
	Status  string  `json:"status"`
	Latency float64 `json:"latency_ms"`
	Users   int     `json:"users_count"`
	Ratings int     `json:"ratings_count"`
}

type DatabaseHealthInfo struct {
	Status        string `json:"status"`
	TotalUsers    int    `json:"total_users"`
	TotalMovies   int    `json:"total_movies"`
	CachedResults int    `json:"cached_results"`
}

type SystemHealthMetrics struct {
	TotalRequests   int     `json:"total_requests"`
	AverageTimeMS   float64 `json:"average_time_ms"`
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	MemoryUsageMB   uint64  `json:"memory_usage_mb"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
}

type MetricsResponse struct {
	Concurrent  MetricsScenario   `json:"concurrent"`
	Distributed MetricsScenario   `json:"distributed"`
	Comparison  MetricsComparison `json:"comparison"`
}

type MetricsScenario struct {
	AverageTimeMS   float64 `json:"average_time_ms"`
	MedianTimeMS    float64 `json:"median_time_ms"`
	MinTimeMS       float64 `json:"min_time_ms"`
	MaxTimeMS       float64 `json:"max_time_ms"`
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	MemoryUsageMB   uint64  `json:"memory_usage_mb"`
	Throughput      float64 `json:"requests_per_second"`
}

type MetricsComparison struct {
	SpeedupFactor    float64 `json:"speedup_factor"`
	EfficiencyGain   float64 `json:"efficiency_gain_percent"`
	ScalabilityScore float64 `json:"scalability_score"`
}

// Registro de Middleware CORS
func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// Registro de Middleware
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("[API] %s %s", r.Method, r.URL.Path)
		next(w, r)
		log.Printf("[API] Completed in %v", time.Since(start))
	}
}

// Handler: POST /api/recommendations
func (api *APIServer) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RecommendationAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TopN <= 0 {
		req.TopN = 10
	}

	startTime := time.Now()

	// Verificar caché en base de datos
	cacheHit := false
	cachedRecs, err := api.db.GetCachedRecommendations(req.UserID, req.TopN)

	var recommendations []RecommendationItem
	var nodesUsed int

	if err == nil && len(cachedRecs) > 0 {
		// Cache hit
		cacheHit = true
		recommendations = cachedRecs
		nodesUsed = 0
		log.Printf("[API] Cache hit para usuario %d", req.UserID)
	} else {
		// Cache miss - calcular recomendaciones distribuidas
		distRecs, nodes, distErr := api.coordinator.GetDistributedRecommendations(req.UserID, req.TopN)
		if distErr != nil {
			http.Error(w, fmt.Sprintf("Error getting recommendations: %v", distErr), http.StatusInternalServerError)
			return
		}

		recommendations = distRecs
		nodesUsed = nodes

		// Guardar en caché
		go api.db.CacheRecommendations(req.UserID, recommendations)
	}

	processTime := time.Since(startTime).Milliseconds()

	// Obtener métricas actuales
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	response := RecommendationAPIResponse{
		UserID:          req.UserID,
		Recommendations: recommendations,
		ProcessTimeMS:   float64(processTime),
		NodesUsed:       nodesUsed,
		CacheHit:        cacheHit,
		Metrics: APIMetrics{
			TotalCPU:    api.metrics.GetCurrentCPU(),
			TotalMemory: memStats.Alloc / 1024 / 1024,
			Speedup:     api.metrics.GetCurrentSpeedup(),
		},
	}

	// Registrar métricas
	api.metrics.RecordRequest(float64(processTime), nodesUsed > 0)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Handler: GET /api/health
func (api *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verificar salud de workers
	workersHealth := make([]WorkerHealthInfo, 0)
	for _, worker := range api.coordinator.workers {
		health := WorkerHealthInfo{
			Address: worker.Address,
			Status:  "unknown",
			Latency: 0,
		}

		// Ping al worker
		start := time.Now()
		if api.coordinator.PingWorker(worker.Address) {
			health.Status = "healthy"
			health.Latency = float64(time.Since(start).Milliseconds())

			// Obtener estadísticas del worker
			stats := api.coordinator.GetWorkerStats(worker.Address)
			health.Users = stats.Users
			health.Ratings = stats.Ratings
		} else {
			health.Status = "unhealthy"
		}

		workersHealth = append(workersHealth, health)
	}

	// Estado de la base de datos
	dbHealth := DatabaseHealthInfo{
		Status:        "healthy",
		TotalUsers:    api.db.GetUserCount(),
		TotalMovies:   api.db.GetMovieCount(),
		CachedResults: api.db.GetCacheSize(),
	}

	// Métricas del sistema
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	systemMetrics := SystemHealthMetrics{
		TotalRequests:   api.metrics.GetTotalRequests(),
		AverageTimeMS:   api.metrics.GetAverageTime(),
		CPUUsagePercent: api.metrics.GetCurrentCPU(),
		MemoryUsageMB:   memStats.Alloc / 1024 / 1024,
		CacheHitRate:    api.metrics.GetCacheHitRate(),
	}

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
		Workers:   workersHealth,
		Database:  dbHealth,
		Metrics:   systemMetrics,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Handler: GET /api/metrics
func (api *APIServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	concurrentMetrics := api.metrics.GetConcurrentMetrics()
	distributedMetrics := api.metrics.GetDistributedMetrics()

	// Calcular comparación
	speedup := 1.0
	if concurrentMetrics.AverageTimeMS > 0 {
		speedup = concurrentMetrics.AverageTimeMS / distributedMetrics.AverageTimeMS
	}

	efficiencyGain := (speedup - 1.0) * 100
	scalabilityScore := speedup / float64(len(api.coordinator.workers))

	comparison := MetricsComparison{
		SpeedupFactor:    speedup,
		EfficiencyGain:   efficiencyGain,
		ScalabilityScore: scalabilityScore,
	}

	response := MetricsResponse{
		Concurrent:  concurrentMetrics,
		Distributed: distributedMetrics,
		Comparison:  comparison,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Handler: GET /api/users/:id
func (api *APIServer) handleGetUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extraer ID del path
	pathParts := splitPath(r.URL.Path)
	if len(pathParts) < 3 {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(pathParts[2])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := api.db.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// Handler: GET /api/movies/:id
func (api *APIServer) handleGetMovie(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := splitPath(r.URL.Path)
	if len(pathParts) < 3 {
		http.Error(w, "Movie ID required", http.StatusBadRequest)
		return
	}

	movieID, err := strconv.Atoi(pathParts[2])
	if err != nil {
		http.Error(w, "Invalid movie ID", http.StatusBadRequest)
		return
	}

	movie, err := api.db.GetMovie(movieID)
	if err != nil {
		http.Error(w, "Movie not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(movie)
}

// Helper para dividir path
func splitPath(path string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(path, '/') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitString(s string, sep rune) []string {
	var parts []string
	var current string
	for _, c := range s {
		if c == sep {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// Iniciar servidor API
func StartAPIServer(coordinator *DistributedCoordinator, db *Database, metrics *SystemMetrics, port string) {
	api := &APIServer{
		coordinator: coordinator,
		db:          db,
		metrics:     metrics,
	}

	// Configurar rutas
	http.HandleFunc("/api/recommendations", loggingMiddleware(enableCORS(api.handleRecommendations)))
	http.HandleFunc("/api/health", loggingMiddleware(enableCORS(api.handleHealth)))
	http.HandleFunc("/api/metrics", loggingMiddleware(enableCORS(api.handleMetrics)))
	http.HandleFunc("/api/users/", loggingMiddleware(enableCORS(api.handleGetUser)))
	http.HandleFunc("/api/movies/", loggingMiddleware(enableCORS(api.handleGetMovie)))

	// Ruta raíz
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"service": "Movie Recommendation System API",
			"version": "1.0.0",
			"status":  "running",
		})
	})

	log.Printf("[API] Servidor iniciado en %s", port)
	log.Printf("[API] Endpoints disponibles:")
	log.Printf("[API]   POST   /api/recommendations")
	log.Printf("[API]   GET    /api/health")
	log.Printf("[API]   GET    /api/metrics")
	log.Printf("[API]   GET    /api/users/{id}")
	log.Printf("[API]   GET    /api/movies/{id}")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("[API] Error iniciando servidor: %v", err)
	}
}
