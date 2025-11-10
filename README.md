# Sistema de Recomendación de Películas - Filtrado Colaborativo Paralelo

Sistema de recomendación distribuido basado en filtrado colaborativo que procesa 20 millones de reseñas en paralelo utilizando Go, ofreciendo recomendaciones personalizadas con alto rendimiento y escalabilidad.
## Características

- **Filtrado Colaborativo User-Based**: Recomendaciones basadas en usuarios similares
- **Cosine Similarity**: Cálculo de similitud entre usuarios
- **k-NN Algorithm**: Búsqueda de k vecinos más cercanos
- **Procesamiento Paralelo**: Goroutines y Channels para máximo rendimiento
- **Optimizado para Big Data**: Procesa 20M+ ratings eficientemente
- **Go Puro**: Sin dependencias externas

## Requisitos

- Go 1.16 o superior
- Dataset MovieLens 20M (incluido en `data_20M/`)
## Etapas del Sistema

### Etapa 1: Análisis y Limpieza de Datos

- **Carga de datos**: Lee `movies.csv` y `ratings.csv`
- **Validación**: Limpia registros inválidos o incompletos
- **Matriz Usuario-Película**: Genera estructura de datos optimizada
- **Normalización**: Calcula promedios por usuario (mean-centering)

**Características**:
- Procesa 20M+ ratings
- 138,493 usuarios únicos
- 26,744 películas únicas
- Rating promedio global: 3.526

### Etapa 2: Filtrado Colaborativo

**Algoritmo**: User-Based Collaborative Filtering

**Componentes**:

1. **Cosine Similarity**: Calcula similitud entre vectores de usuarios
   ```
   similarity = (A · B) / (||A|| × ||B||)
   ```
   - Con mean-centering para mejor precisión
   - Filtro: mínimo 3 películas en común

2. **k-NN (k Nearest Neighbors)**:
   - k = 30 vecinos más cercanos
   - Sampling de 20,000 usuarios para optimización
   - Selección de usuarios más similares

3. **Generación de Recomendaciones**:
   - Predicción ponderada por similitud
   - Top-N recomendaciones personalizadas

### Etapa 3: Paralelización

**Tecnología**: Goroutines y Channels de Go

**Arquitectura**:
- **Worker Pool Pattern**: Múltiples goroutines procesan trabajos en paralelo
- **Channels**: Comunicación segura entre goroutines
- **WaitGroups**: Sincronización de workers

**Configuraciones de Workers**:
- 1, 2, 4, 8, 16 workers (configurable)
