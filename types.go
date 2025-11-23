package main

// TIPOS COMPARTIDOS - Sistema Distribuido
// ============================================================================
// Solicitud que el coordinador envía a los workers
type SimilarityRequest struct {
	TargetUserID  int             `json:"target_user_id"`
	TargetRatings map[int]float64 `json:"target_ratings"`
	TargetAvg     float64         `json:"target_avg"`
	K             int             `json:"k"`
	SampleSize    int             `json:"sample_size"`
}

// Respuesta que los workers envían al coordinador
type SimilarityResponse struct {
	WorkerID     string             `json:"worker_id"`
	Similarities []SimilarityResult `json:"similarities"`
	ProcessTime  float64            `json:"process_time_ms"`
	UsersChecked int                `json:"users_checked"`
	CPUUsage     float64            `json:"cpu_usage"`
	MemoryUsage  uint64             `json:"memory_mb"`
}

// Representa la similitud entre dos usuarios
type SimilarityResult struct {
	UserID     int     `json:"user_id"`
	Similarity float64 `json:"similarity"`
}
