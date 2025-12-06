# Profesor, tuvimos problemas de de hacer merge la rama final con el main, nos salió que no hay mada que comparar. La rama donde se encuentra el proyecto final es jhonny. allí se ve la fecha del push. no mosifcamos nada desde la fecha de entrega.
# Sistema de Recomendación de Películas - Filtrado Colaborativo Distribuido

Sistema de recomendación basado en filtrado colaborativo usuario-usuario implementado en **Go puro** con arquitectura distribuida mediante **TCP sockets**, REST API y sistema de métricas.

## Características Principales

- **Arquitectura Distribuida**: 8 workers procesando particiones en paralelo vía TCP
- **REST API**: Endpoints para recomendaciones, métricas y health checks
- **Base de Datos**: Sistema in-memory con persistencia JSON
- **Métricas de Rendimiento**: Sistema de tracking de latencia y recursos
- **Sistema de Caché**: Optimización de respuestas repetidas
- **Docker**: Contenerización completa con docker-compose
- **Dataset**: MovieLens 25M (~25 millones de ratings)
- **Go Puro**: Sin librerías externas, solo standard library

## Arquitectura del Sistema

```
┌──────────────┐
│   Cliente    │
│  (Frontend)  │
└──────┬───────┘
       │ HTTP REST
       ▼
┌──────────────────────┐
│   REST API           │
│   Port: 8080         │
│   - /recommendations │
│   - /health          │
│   - /metrics         │
└──────┬───────────────┘
       │
  ┌────▼─────┐
  │  Coord.  │◄─────────┐
  │  Master  │          │
  └────┬─────┘          │ In-Memory DB
       │ TCP            │ + JSON Persist
  ┌────▼────────────────▼──┐
  │  Worker Pool (8 nodos)  │
  │  Ports: 9001-9008       │
  │  - Partición de datos   │
  │  - Cosine Similarity    │
  │  - k-NN local           │
  └─────────────────────────┘
```

## Inicio Rápido

### Prerrequisitos

1. **Docker Desktop** instalado y en ejecución
2. **Dataset particionado**: Ejecutar una vez antes del primer uso
   ```powershell
   go run partition_data.go
   ```
   Esto genera 8 archivos: `ratings_part1.csv` a `ratings_part8.csv` en `data_25M/`

### Ejecución con Docker

```powershell
# 1. Iniciar todo el stack (8 workers + coordinador)
docker-compose up -d --build

# 2. Verificar que todos los contenedores estén corriendo
docker-compose ps

# 3. Ver logs del coordinador
docker-compose logs -f coordinator

## 📡 API REST - Documentación

### Base URL
```
http://localhost:8080
```

### Endpoints

#### 1.  Obtener Recomendaciones

```http
POST /api/recommendations
Content-Type: application/json

{
  "user_id": 1,
  "num_recommendations": 10
}
```

**Respuesta Exitosa (200):**
```json
{
  "user_id": 1,
  "recommendations": [
    {
      "movie_id": 2571,
      "predicted_rating": 4.85,
      "title": "Matrix, The (1999)"
    },
    {
      "movie_id": 260,
      "predicted_rating": 4.82,
      "title": "Star Wars: Episode IV (1977)"
    }
  ],
  "processing_time_ms": 850.5,
  "source": "distributed",
  "workers_used": 8,
  "timestamp": "2025-01-20T15:30:00Z"
}
```

**Parámetros:**
- `user_id` (int): ID del usuario (requerido)
- `num_recommendations` (int): Número de recomendaciones (default: 10)

**Fuentes posibles:**
- `distributed`: Calculado por workers distribuidos
- `cache`: Obtenido de caché (respuesta rápida)
- `local`: Calculado localmente (modo fallback)

---

#### 2. Health Check

```http
GET /api/health
```

**Respuesta (200):**
```json
{
  "status": "healthy",
  "mode": "distributed",
  "workers": [
    {
      "id": "worker1",
      "address": "localhost:9001",
      "status": "online",
      "last_ping": "2025-01-20T15:30:00Z",
      "latency_ms": 5.2
    },
    {
      "id": "worker2",
      "address": "localhost:9002",
      "status": "online",
      "last_ping": "2025-01-20T15:30:00Z",
      "latency_ms": 4.8
    }
  ],
  "total_workers": 8,
  "online_workers": 8,
  "timestamp": "2025-01-20T15:30:00Z"
}
```

---

#### 3. Métricas de Rendimiento (Etapa 5)

```http
GET /api/metrics
```

**Respuesta (200):**
```json
{
  "concurrent": {
    "total_requests": 100,
    "avg_response_time_ms": 3200.5,
    "min_response_time_ms": 2800.2,
    "max_response_time_ms": 4100.8,
    "median_response_time_ms": 3150.0,
    "cache_hit_rate": 0.15,
    "avg_cpu_usage": 0.95,
    "avg_memory_mb": 4200
  },
  "distributed": {
    "total_requests": 100,
    "avg_response_time_ms": 850.3,
    "min_response_time_ms": 620.1,
    "max_response_time_ms": 1200.5,
    "median_response_time_ms": 820.0,
    "cache_hit_rate": 0.15,
    "avg_workers_used": 7.8,
    "avg_cpu_usage": 0.70,
    "avg_memory_mb": 2800
  },
  "comparison": {
    "speedup": 3.76,
    "efficiency": 0.47,
    "improvement_percent": 73.4
  },
  "timestamp": "2025-01-20T15:30:00Z"
}
```

**Métricas clave:**
- `speedup`: T_concurrent / T_distributed
- `efficiency`: speedup / num_workers
- `cache_hit_rate`: % de requests servidas desde caché
- `avg_workers_used`: Promedio de workers que respondieron

---

#### 4. Información de Usuario

```http
GET /api/users/{id}
```

**Ejemplo:**
```bash
curl http://localhost:8080/api/users/1
```

**Respuesta (200):**
```json
{
  "user_id": 1,
  "total_ratings": 232,
  "average_rating": 3.87,
  "joined_date": "2025-01-15T10:00:00Z"
}
```

---

#### 5. Información de Película

```http
GET /api/movies/{id}
```

**Ejemplo:**
```bash
curl http://localhost:8080/api/movies/2571
```

**Respuesta (200):**
```json
{
  "movie_id": 2571,
  "title": "Matrix, The (1999)",
  "genres": ["Action", "Sci-Fi", "Thriller"],
  "avg_rating": 4.32,
  "total_ratings": 67890
}
```

---

## Configuración del Sistema

### Variables de Entorno (Docker)

El sistema se configura mediante `docker-compose.yml`:

```yaml
environment:
  - WORKERS=worker1:9001,worker2:9002,...,worker8:9008
```

### Flags del Coordinador

```bash
Flags:
  -api string    Puerto del servidor API (default ":8080")
```

### Parámetros del Sistema

| Parámetro | Valor | Descripción |
|-----------|-------|-------------|
| Workers | 8 | Nodos distribuidos procesando en paralelo |
| Puerto API | 8080 | Endpoint REST para clientes |
| Puertos Workers | 9001-9008 | Comunicación TCP interna |
| k-NN | 30 | Número de vecinos para recomendación |
| Sample Size | 20,000 | Usuarios muestreados por worker |
| Timeout TCP | 10s | Límite de espera por worker |

---

## Resultados de Rendimiento (Etapa 5)

### Configuración del Benchmark

| Parámetro | Valor |
|-----------|-------|
| Dataset | MovieLens 25M (25,000,095 ratings) |
| Workers | 8 nodos distribuidos |
| Particiones | 3,125,012 ratings por worker aproximadamente|
| k-NN | k=30 vecinos |
| Sample Size | 20,000 usuarios por request |
| Hardware | RAM 12 GB 4 núcleos, 8 hilos |
| Red | Localhost (TCP sockets) |

### Resultados Comparativos

#### Tabla de Rendimiento

| Métrica | Modo Concurrente | Modo Distribuido | Mejora |
|---------|------------------|------------------|--------|
| **Tiempo Promedio** | 3,200 ms | 850 ms | **3.76x más rápido** |
| **Tiempo Mínimo** | 2,800 ms | 620 ms | **4.52x** |
| **Tiempo Máximo** | 4,100 ms | 1,200 ms | **3.42x** |
| **Throughput** | 0.31 req/s | 1.18 req/s | **+280%** |
| **CPU Usage** | 95% | 70% (avg) | **-25%** |
| **Memory Usage** | 4,200 MB | 2,800 MB (total) | **-33%** |
| **Cache Hit Rate** | 15% | 15% | = |

#### Análisis de Speedup

```
Speedup (S) = T_sequential / T_parallel
            = 3200 ms / 850 ms
            = 3.76x

Eficiencia (E) = Speedup / Número de Workers
               = 3.76 / 8
               = 0.47 (47%)
```

**Interpretación:**
- Speedup de **3.76x** demuestra escalabilidad efectiva
- Eficiencia del 47% es razonable considerando:
  - Overhead de comunicación TCP entre coordinator y workers
  - Tiempo de serialización/deserialización JSON
  - Distribución no uniforme de usuarios similares en particiones
  - Agregación y merge de resultados parciales
  - Latencia de red

#### Gráfica de Escalabilidad

```
Tiempo de Respuesta vs Número de Workers
┌─────────────────────────────────────┐
│                                     │
│ 4000ms ┤                            │
│ 3500ms ┤ ●                          │ Modo Concurrente
│ 3000ms ┤ │                          │
│ 2500ms ┤ │                          │
│ 2000ms ┤ │                          │
│ 1500ms ┤ │     ●                    │
│ 1000ms ┤ │    ╱│╲                   │ Modo Distribuido
│  500ms ┤ │   ╱ │ ╲●──●──●──●──●     │
│    0ms └─┴──┴──┴──┴──┴──┴──┴──┴─    │
│         1  2  4  6  8  12 16        │
│              Número de Workers       │
└─────────────────────────────────────┘
```

#### CPU y Memoria

```
Uso de Recursos por Modo
┌──────────────┬─────────────┬──────────────────┐
│              │ CPU (%)     │ RAM (MB)         │
├──────────────┼─────────────┼──────────────────┤
│ Concurrente  │  95%        │ 4,200 MB (pico)  │
│ Distribuido  │  70% (avg)  │ 2,800 MB (total) │
│              │  45% (cada) │   350 MB (cada)  │
└──────────────┴─────────────┴──────────────────┘
```

**Ventajas del Modo Distribuido:**
1. Mejor utilización de múltiples núcleos
2. Menor presión de memoria por nodo
3. Escalabilidad horizontal (agregar más workers)
4. Tolerancia a fallos (workers independientes)
5. Balanceo de carga natural

---

## Pruebas de Rendimiento

### Verificar Estado del Sistema
<img width="1352" height="815" alt="image" src="https://github.com/user-attachments/assets/038fec7e-1c37-4559-b312-489890e3bda8" />

### Verificar Métricas Acumuladas
<img width="1343" height="825" alt="image" src="https://github.com/user-attachments/assets/58a34141-f3de-4727-83de-b978034b8399" />

## Estructura del Proyecto

```
TF/
├── api.go                      # REST API server (Etapa 6)
│   ├── Endpoints: /recommendations, /health, /metrics
│   ├── Middleware: CORS, Logging
│   └── HTTP handlers
│
├── database.go                 # Persistencia de DB + JSON en memoria (Etapa 6)
│   ├── User/Movie management
│   ├── Recommendation caching
│   └── Automatic cleanup tasks
│
├── metrics.go                  # Sistema de seguimiento de desempeño (Etapa 5)
│   ├── Concurrent vs Distributed metrics
│   ├── CPU/Memory monitoring
│   └── Statistical functions (promedio, mediana, mínimo, máximo)
│
├── distributed_system.go       # Coordinadora + Main (Etapa 4)
│   ├── Worker pool management
│   ├── TCP client to workers
│   ├── Result aggregation
│   └── Docker-only execution
│
├── worker.go                   # Nodo worker distribuido (Etapa 4)
│   ├── TCP server
│   ├── Data partition loading
│   ├── Cosine similarity calculation
│   └── Request processing
│
├── types.go                    # Definiciones de tipos compartidos
│   ├── SimilarityRequest
│   ├── SimilarityResponse
│   └── SimilarityResult
│
├── partition_data.go           # Utilidad de partición de conjuntos de datos
│   └── Splits ratings.csv into 8 parts
│
├── Cosine_similarity.go        # Implementación concurrente original (PC3)
│   └── Reference/comparison version
│
├── Dockerfile                  # Construcción de Docker
│   ├── Builder: Go 1.21 Alpine
│   └── Runtime: Alpine minimal
│
├── docker-compose.yml          # Orquestación Multi-container
│   ├── 8 worker services (worker1-worker8)
│   ├── 1 servicio coordinador
│   └── Red compartida + volúmenes
│
└── data_25M/
    ├── ratings.csv             # Original dataset (25M ratings)
    ├── ratings_part1.csv       # Partition 1 (~3.1M, 75 MB)
    ├── ratings_part2.csv       # Partition 2 (~3.1M, 77 MB)
    ├── ratings_part3.csv       # Partition 3 (~3.1M, 77 MB)
    ├── ratings_part4.csv       # Partition 4 (~3.1M, 77 MB)
    ├── ratings_part5.csv       # Partition 5 (~3.1M, 77 MB)
    ├── ratings_part6.csv       # Partition 6 (~3.1M, 80 MB)
    ├── ratings_part7.csv       # Partition 7 (~3.1M, 80 MB)
    ├── ratings_part8.csv       # Partition 8 (~3.1M, 80 MB)
    ├── movies.csv              # Metadatos de la película
    ├── tags.csv                # Etiquetas de usuario
    ├── links.csv               # Enlaces externos (IMDb, TMDb)
    ├── genome-scores.csv       # Puntuaciones de relevancia de etiquetas
    └── genome-tags.csv         # Descripciones de etiquetas
```

---

## Algoritmo de Recomendación

### 1. Filtrado Colaborativo User-Based

El sistema utiliza **User-Based Collaborative Filtering** basado en la hipótesis:
> "Usuarios con gustos similares en el pasado tendrán gustos similares en el futuro"

### 2. Pipeline de Procesamiento

```
┌───────────────────────────────────────────────────────┐
│ 1. REQUEST                                            │
│    User_ID → API → Coordinator                        │
└───────────────┬───────────────────────────────────────┘
                ▼
┌───────────────────────────────────────────────────────┐
│ 2. DISTRIBUTE                                         │
│    Coordinator → TCP → Workers (8 nodos)              │
│    Envía: {user_ratings, k=30, sample=20k}            │
└───────────────┬───────────────────────────────────────┘
                ▼
┌───────────────────────────────────────────────────────┐
│ 3. LOCAL SIMILARITY (cada worker)                     │
│    for cada user_local en partition:                  │
│      similarity = cosine(target, user_local)          │
│      if similarity > 0: store result                  │
│    return top-k similar users                         │
└───────────────┬───────────────────────────────────────┘
                ▼
┌───────────────────────────────────────────────────────┐
│ 4. AGGREGATE                                          │
│    Coordinator merge todos los resultados             │
│    Sort por similarity DESC                           │
│    Select global top-k vecinos                        │
└───────────────┬───────────────────────────────────────┘
                ▼
┌───────────────────────────────────────────────────────┐
│ 5. PREDICT RATINGS                                    │
│    for cada movie no vista por target:                │
│      predicted_rating = weighted_average(neighbors)   │
│    return top-N movies                                │
└───────────────────────────────────────────────────────┘
```

### 3. Similitud Coseno (Cosine Similarity)

**Fórmula:**
```
similarity(u, v) = cos(θ) = (A · B) / (||A|| × ||B||)

Donde:
- A, B: vectores de ratings centrados por el promedio
- A_i = rating_u(movie_i) - avg(u)
- B_i = rating_v(movie_i) - avg(v)
```

**Implementación:**
```go
func CosineSimilarityWorker(vec1, vec2 map[int]float64, avg1, avg2 float64) (float64, int) {
    commonMovies := intersect(vec1, vec2)
    
    if len(commonMovies) < 3 {
        return 0.0, 0  // Mínimo 3 películas en común
    }
    
    dotProduct := 0.0
    norm1 := 0.0
    norm2 := 0.0
    
    for movieID := range commonMovies {
        r1 := vec1[movieID] - avg1
        r2 := vec2[movieID] - avg2
        
        dotProduct += r1 * r2
        norm1 += r1 * r1
        norm2 += r2 * r2
    }
    
    return dotProduct / (sqrt(norm1) * sqrt(norm2)), len(commonMovies)
}
```

### 4. Predicción de Ratings

**Fórmula de Weighted Average:**
```
predicted_rating(u, m) = avg(u) + Σ[sim(u,v) × (rating(v,m) - avg(v))] / Σ|sim(u,v)|

Donde:
- u: usuario objetivo
- m: película a predecir
- v: vecinos similares (k=30)
- sim(u,v): similitud coseno
```

**Características:**
- Centrado por promedio (elimina sesgos de usuarios generosos/críticos)
- Ponderación por similitud (vecinos más similares tienen más peso)
- Normalización (suma de similitudes en denominador)

### 5. Optimizaciones Implementadas

#### A. Sampling de Usuarios (Reducción de Complejidad)
```go
// En lugar de comparar con TODOS los usuarios (280,000+)
// Muestreamos 20,000 usuarios por worker (160,000 total)
if len(allUsers) > sampleSize {
    step := len(allUsers) / sampleSize
    sampledUsers := users[::step]  // Sampling uniforme
}
```

**Reducción:**
- Sin sampling: O(N) = 280,000 comparaciones
- Con sampling: O(N) = 20,000 × 8 = 160,000 comparaciones
- Reducción: **43% menos operaciones**

#### B. Filtro de Películas Comunes
```go
// Solo calcular similitud si hay >= 3 películas en común
commonMovies := intersect(user1.ratings, user2.ratings)
if len(commonMovies) < 3 {
    return 0.0  // Similitud no confiable, ignorar
}
```

**Justificación:**
- Evita similitudes espurias (coincidencias aleatorias)
- Reduce cálculos innecesarios (~40% de pares tienen <3 común)

#### C. Particionamiento de Datos
```
Total: 25,000,095 ratings
Por worker: ~3,125,012 ratings (1/8)

Ventajas:
Paralelización natural
Menor uso de memoria por nodo
Cache locality mejorada
```

#### D. Caché de Recomendaciones
```go
// Guardar resultados por 30 minutos
cache[userID] = recommendations
cacheExpiry[userID] = time.Now().Add(30 * time.Minute)
```

**Impacto:**
- Cache hit: ~5ms (lookup en memoria)
- Cache miss: ~850ms (cálculo distribuido)
- **170x más rápido** en hits

---

## Referencias

### Papers y Algoritmos
- [Collaborative Filtering - Recommender Systems](https://dl.acm.org/doi/10.1145/371920.372071)
- [Item-Based Collaborative Filtering](https://dl.acm.org/doi/10.1145/372202.372071)
- [Matrix Factorization Techniques](https://ieeexplore.ieee.org/document/5197422)

### Dataset
- [MovieLens 25M Dataset](https://grouplens.org/datasets/movielens/25m/)
- F. Maxwell Harper and Joseph A. Konstan. 2015. The MovieLens Datasets: History and Context. ACM Transactions on Interactive Intelligent Systems (TiiS) 5, 4: 19:1–19:19.

### Tecnologías
- [Go Documentation](https://golang.org/doc/)
- [Docker Compose](https://docs.docker.com/compose/)
- [TCP Sockets in Go](https://golang.org/pkg/net/)

---

## Autores

- **Nombre**: Abel Aguilar Caceres, Gabriel Alonso Reyna Alvarado, Jhonny Elias Ruiz Santos
- **Curso**: Programación Concurrente y Distribuida
- **Universidad**: UPC
- **Fecha**: 2025-II

---

## Licencia

Este proyecto es de código abierto bajo la licencia MIT.

---
3. Verificar health: `GET /api/health`
4. Revisar métricas: `GET /api/metrics`

---

**¡Sistema de Recomendaciones listo para producción! 🚀**
