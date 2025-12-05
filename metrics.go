package main

import (
	"runtime"
	"sync"
	"time"
)

// METRICS - ETAPA 5
type SystemMetrics struct {
	// Métricas concurrentes
	ConcurrentRequests  []RequestMetric
	ConcurrentTotalTime time.Duration
	ConcurrentCPU       []float64
	ConcurrentMemory    []uint64

	// Métricas distribuidas
	DistributedRequests  []RequestMetric
	DistributedTotalTime time.Duration
	DistributedCPU       []float64
	DistributedMemory    []uint64

	// Estadísticas generales
	TotalRequests int
	CacheHits     int
	CacheMisses   int
	ActiveNodes   int

	mu        sync.RWMutex
	startTime time.Time
}

type RequestMetric struct {
	Timestamp     time.Time
	DurationMS    float64
	IsDistributed bool
	NodesUsed     int
	CPUPercent    float64
	MemoryMB      uint64
	Success       bool
}

type NodeMetrics struct {
	NodeID          string
	RequestsHandled int
	AverageTimeMS   float64
	CPUUsage        float64
	MemoryUsage     uint64
	LastUpdate      time.Time
}

// Crear nuevo sistema de métricas
func NewSystemMetrics() *SystemMetrics {
	return &SystemMetrics{
		ConcurrentRequests:  make([]RequestMetric, 0),
		DistributedRequests: make([]RequestMetric, 0),
		ConcurrentCPU:       make([]float64, 0),
		ConcurrentMemory:    make([]uint64, 0),
		DistributedCPU:      make([]float64, 0),
		DistributedMemory:   make([]uint64, 0),
		startTime:           time.Now(),
	}
}

// Registrar una petición
func (m *SystemMetrics) RecordRequest(durationMS float64, isDistributed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metric := RequestMetric{
		Timestamp:     time.Now(),
		DurationMS:    durationMS,
		IsDistributed: isDistributed,
		CPUPercent:    m.getCurrentCPUUsage(),
		MemoryMB:      memStats.Alloc / 1024 / 1024,
		Success:       true,
	}

	m.TotalRequests++

	if isDistributed {
		m.DistributedRequests = append(m.DistributedRequests, metric)
		m.DistributedTotalTime += time.Duration(durationMS) * time.Millisecond
		m.DistributedCPU = append(m.DistributedCPU, metric.CPUPercent)
		m.DistributedMemory = append(m.DistributedMemory, metric.MemoryMB)
	} else {
		m.ConcurrentRequests = append(m.ConcurrentRequests, metric)
		m.ConcurrentTotalTime += time.Duration(durationMS) * time.Millisecond
		m.ConcurrentCPU = append(m.ConcurrentCPU, metric.CPUPercent)
		m.ConcurrentMemory = append(m.ConcurrentMemory, metric.MemoryMB)
	}
}

// Registrar cache hit
func (m *SystemMetrics) RecordCacheHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CacheHits++
}

// Registrar cache miss
func (m *SystemMetrics) RecordCacheMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CacheMisses++
}

// Obtener métricas de escenario concurrente
func (m *SystemMetrics) GetConcurrentMetrics() MetricsScenario {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.ConcurrentRequests) == 0 {
		return MetricsScenario{}
	}

	times := make([]float64, len(m.ConcurrentRequests))
	for i, req := range m.ConcurrentRequests {
		times[i] = req.DurationMS
	}

	return MetricsScenario{
		AverageTimeMS:   m.average(times),
		MedianTimeMS:    m.median(times),
		MinTimeMS:       m.min(times),
		MaxTimeMS:       m.max(times),
		CPUUsagePercent: m.average(m.ConcurrentCPU),
		MemoryUsageMB:   m.averageUint64(m.ConcurrentMemory),
		Throughput:      m.calculateThroughput(len(m.ConcurrentRequests)),
	}
}

// Obtener métricas de escenario distribuido
func (m *SystemMetrics) GetDistributedMetrics() MetricsScenario {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.DistributedRequests) == 0 {
		return MetricsScenario{}
	}

	times := make([]float64, len(m.DistributedRequests))
	for i, req := range m.DistributedRequests {
		times[i] = req.DurationMS
	}

	return MetricsScenario{
		AverageTimeMS:   m.average(times),
		MedianTimeMS:    m.median(times),
		MinTimeMS:       m.min(times),
		MaxTimeMS:       m.max(times),
		CPUUsagePercent: m.average(m.DistributedCPU),
		MemoryUsageMB:   m.averageUint64(m.DistributedMemory),
		Throughput:      m.calculateThroughput(len(m.DistributedRequests)),
	}
}

// Obtener CPU actual
func (m *SystemMetrics) GetCurrentCPU() float64 {
	return m.getCurrentCPUUsage()
}

// Obtener speedup actual
func (m *SystemMetrics) GetCurrentSpeedup() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	concurrentMetrics := m.GetConcurrentMetrics()
	distributedMetrics := m.GetDistributedMetrics()

	if distributedMetrics.AverageTimeMS == 0 {
		return 1.0
	}

	return concurrentMetrics.AverageTimeMS / distributedMetrics.AverageTimeMS
}

// Obtener total de requests
func (m *SystemMetrics) GetTotalRequests() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.TotalRequests
}

// Obtener tiempo promedio
func (m *SystemMetrics) GetAverageTime() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalRequests := len(m.ConcurrentRequests) + len(m.DistributedRequests)
	if totalRequests == 0 {
		return 0
	}

	totalTime := m.ConcurrentTotalTime + m.DistributedTotalTime
	return float64(totalTime.Milliseconds()) / float64(totalRequests)
}

// Obtener tasa de cache hit
func (m *SystemMetrics) GetCacheHitRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.CacheHits + m.CacheMisses
	if total == 0 {
		return 0
	}

	return float64(m.CacheHits) / float64(total) * 100
}

// Generar reporte de rendimiento
func (m *SystemMetrics) GeneratePerformanceReport() PerformanceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return PerformanceReport{
		Timestamp:           time.Now(),
		Uptime:              time.Since(m.startTime),
		TotalRequests:       m.TotalRequests,
		ConcurrentRequests:  len(m.ConcurrentRequests),
		DistributedRequests: len(m.DistributedRequests),
		CacheHitRate:        m.GetCacheHitRate(),
		AverageTimeMS:       m.GetAverageTime(),
		ConcurrentMetrics:   m.GetConcurrentMetrics(),
		DistributedMetrics:  m.GetDistributedMetrics(),
	}
}

type PerformanceReport struct {
	Timestamp           time.Time
	Uptime              time.Duration
	TotalRequests       int
	ConcurrentRequests  int
	DistributedRequests int
	CacheHitRate        float64
	AverageTimeMS       float64
	ConcurrentMetrics   MetricsScenario
	DistributedMetrics  MetricsScenario
}

// HELPERS ESTADÍSTICOS
func (m *SystemMetrics) average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func (m *SystemMetrics) averageUint64(values []uint64) uint64 {
	if len(values) == 0 {
		return 0
	}
	sum := uint64(0)
	for _, v := range values {
		sum += v
	}
	return sum / uint64(len(values))
}

func (m *SystemMetrics) median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)

	// Bubble sort simple para mediana
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func (m *SystemMetrics) min(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
	}
	return min
}

func (m *SystemMetrics) max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

func (m *SystemMetrics) calculateThroughput(requests int) float64 {
	elapsed := time.Since(m.startTime).Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(requests) / elapsed
}

// Obtener uso de CPU
func (m *SystemMetrics) getCurrentCPUUsage() float64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Aproximación basada en GC y goroutines
	numGoroutines := float64(runtime.NumGoroutine())
	numCPU := float64(runtime.NumCPU())

	// Estimación simple: más goroutines = más CPU usado
	usage := (numGoroutines / (numCPU * 100)) * 100
	if usage > 100 {
		usage = 100
	}

	return usage
}

// Tarea de monitoreo periódico
func (m *SystemMetrics) StartMonitoring() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			m.mu.Lock()
			if m.TotalRequests > 0 {
				m.ConcurrentCPU = append(m.ConcurrentCPU, m.getCurrentCPUUsage())
				m.ConcurrentMemory = append(m.ConcurrentMemory, memStats.Alloc/1024/1024)
			}
			m.mu.Unlock()
		}
	}()
}
