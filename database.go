package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// DATABASE - ETAPA 6
// Base de datos en memoria con persistencia JSON
type Database struct {
	Users               map[int]*User
	Movies              map[int]*Movie
	Ratings             map[int]map[int]float64
	MovieLinks          map[int]MovieLink // userID -> movieID -> rating -> movieLink
	RecommendationCache map[int][]RecommendationItem
	mu                  sync.RWMutex
	persistPath         string
}

type User struct {
	UserID        int       `json:"user_id"`
	RatingsCount  int       `json:"ratings_count"`
	AverageRating float64   `json:"average_rating"`
	TopGenres     []string  `json:"top_genres"`
	LastAccessed  time.Time `json:"last_accessed"`
}

type Movie struct {
	MovieID       int      `json:"movie_id"`
	Title         string   `json:"title"`
	Genres        []string `json:"genres"`
	RatingsCount  int      `json:"ratings_count"`
	AverageRating float64  `json:"average_rating"`
}

type DatabaseSnapshot struct {
	Users   map[int]*User                `json:"users"`
	Movies  map[int]*Movie               `json:"movies"`
	Cache   map[int][]RecommendationItem `json:"cache"`
	Updated time.Time                    `json:"updated"`
}

// Crear nueva base de datos
func NewDatabase(persistPath string) *Database {
	db := &Database{
		Users:               make(map[int]*User),
		Movies:              make(map[int]*Movie),
		Ratings:             make(map[int]map[int]float64),
		MovieLinks:          make(map[int]MovieLink),
		RecommendationCache: make(map[int][]RecommendationItem),
		persistPath:         persistPath,
	}

	// Cargar datos si existen
	if err := db.Load(); err != nil {
		log.Printf("[DB] No se pudo cargar datos previos: %v", err)
	}

	return db
}

// Cargar películas del CSV
func (db *Database) LoadMovies(filepath string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Parsear CSV simple
	scanner := newLineScanner(file)
	scanner.Scan()

	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		parts := parseCSVLine(line)
		if len(parts) < 2 {
			continue
		}

		movieID := parseInt(parts[0])
		if movieID == 0 {
			continue
		}

		title := parts[1]
		genres := make([]string, 0)
		if len(parts) >= 3 {
			genres = splitString(parts[2], '|')
		}

		db.Movies[movieID] = &Movie{
			MovieID: movieID,
			Title:   title,
			Genres:  genres,
		}
		count++
	}

	log.Printf("[DB] Películas cargadas: %d", count)
	return nil
}

// Agregar o actualizar usuario
func (db *Database) UpsertUser(userID int, ratingsCount int, avgRating float64) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if user, exists := db.Users[userID]; exists {
		user.RatingsCount = ratingsCount
		user.AverageRating = avgRating
		user.LastAccessed = time.Now()
	} else {
		db.Users[userID] = &User{
			UserID:        userID,
			RatingsCount:  ratingsCount,
			AverageRating: avgRating,
			LastAccessed:  time.Now(),
		}
	}
}

// Obtener usuario
func (db *Database) GetUser(userID int) (*User, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Si el usuario ya existe en caché, devolverlo
	user, exists := db.Users[userID]
	if exists {
		user.LastAccessed = time.Now()
		return user, nil
	}

	// Si no existe, buscar en Ratings y crearlo
	userRatings, hasRatings := db.Ratings[userID]
	if !hasRatings || len(userRatings) == 0 {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	// Crear usuario con estadísticas calculadas
	sum := 0.0
	count := 0
	for _, rating := range userRatings {
		sum += rating
		count++
	}

	newUser := &User{
		UserID:        userID,
		RatingsCount:  count,
		AverageRating: sum / float64(count),
		TopGenres:     []string{},
		LastAccessed:  time.Now(),
	}

	db.Users[userID] = newUser
	return newUser, nil
}

// Obtener película
func (db *Database) GetMovie(movieID int) (*Movie, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	movie, exists := db.Movies[movieID]
	if !exists {
		return nil, fmt.Errorf("película no encontrada")
	}

	// Calcular estadísticas de ratings si aún no están calculadas
	if movie.RatingsCount == 0 {
		sum := 0.0
		count := 0

		// Recorrer todos los usuarios para encontrar ratings de esta película
		for _, userRatings := range db.Ratings {
			if rating, exists := userRatings[movieID]; exists {
				sum += rating
				count++
			}
		}

		if count > 0 {
			movie.RatingsCount = count
			movie.AverageRating = sum / float64(count)
		}
	}

	return movie, nil
}

// Cachear recomendaciones
func (db *Database) CacheRecommendations(userID int, recommendations []RecommendationItem) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.RecommendationCache[userID] = recommendations
	log.Printf("[DB] Recomendaciones cacheadas para usuario %d", userID)

	// Persistir asíncronamente cada 100 inserciones
	if len(db.RecommendationCache)%100 == 0 {
		go db.Save()
	}
}

// Obtener recomendaciones cacheadas
func (db *Database) GetCachedRecommendations(userID int, topN int) ([]RecommendationItem, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	recs, exists := db.RecommendationCache[userID]
	if !exists {
		return nil, fmt.Errorf("no cache found")
	}

	// Retornar solo topN
	if len(recs) > topN {
		return recs[:topN], nil
	}

	return recs, nil
}

// Limpiar caché antiguo (más de 1 hora)
func (db *Database) CleanOldCache() {
	db.mu.Lock()
	defer db.mu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	cleaned := 0

	for userID, user := range db.Users {
		if user.LastAccessed.Before(cutoff) {
			delete(db.RecommendationCache, userID)
			cleaned++
		}
	}

	if cleaned > 0 {
		log.Printf("[DB] Cache limpiado: %d entradas eliminadas", cleaned)
	}
}

// Obtener estadísticas
func (db *Database) GetUserCount() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.Users)
}

func (db *Database) GetMovieCount() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.Movies)
}

func (db *Database) GetCacheSize() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.RecommendationCache)
}

// Persistir a disco
func (db *Database) Save() error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	snapshot := DatabaseSnapshot{
		Users:   db.Users,
		Movies:  db.Movies,
		Cache:   db.RecommendationCache,
		Updated: time.Now(),
	}

	file, err := os.Create(db.persistPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		return err
	}

	log.Printf("[DB] Base de datos guardada en %s", db.persistPath)
	return nil
}

// Cargar desde disco
func (db *Database) Load() error {
	file, err := os.Open(db.persistPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var snapshot DatabaseSnapshot
	if err := json.NewDecoder(file).Decode(&snapshot); err != nil {
		return err
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	db.Users = snapshot.Users
	db.Movies = snapshot.Movies
	db.RecommendationCache = snapshot.Cache

	log.Printf("[DB] Base de datos cargada desde %s", db.persistPath)
	log.Printf("[DB] Usuarios: %d, Películas: %d, Cache: %d",
		len(db.Users), len(db.Movies), len(db.RecommendationCache))

	return nil
}

// Tarea de limpieza periódica
func (db *Database) StartCleanupTask() {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			db.CleanOldCache()
			db.Save()
		}
	}()
}

// Obtener las películas vistas por un usuario
func (db *Database) GetUserWatchedMovies(userID int) ([]WatchedMovie, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// Verificamos si el usuario existe en Ratings
	userRatings, exists := db.Ratings[userID]
	if !exists || len(userRatings) == 0 {
		return nil, fmt.Errorf("no hay películas para este usuario")
	}

	watched := []WatchedMovie{}

	// Recorrer todas las películas que este usuario ha puntuado
	for movieID, rating := range userRatings {

		// Ver si tenemos datos de la película
		movie, ok := db.Movies[movieID]
		if !ok {
			continue // Película sin info cargada — saltamos
		}

		watched = append(watched, WatchedMovie{
			MovieID: movieID,
			Title:   movie.Title,
			Rating:  rating,
		})
	}

	return watched, nil
}

func (db *Database) LoadMovieLinks(filepath string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := newLineScanner(file)
	scanner.Scan() // saltar encabezado

	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		parts := parseCSVLine(line)
		if len(parts) < 3 {
			continue
		}

		movieID := parseInt(parts[0])
		imdb := parts[1]
		tmdb := parseInt(parts[2])

		db.MovieLinks[movieID] = MovieLink{
			MovieID: movieID,
			ImdbID:  imdb,
			TmdbID:  tmdb,
		}

		count++
	}

	fmt.Printf("[DB] Movie links cargados: %d\n", count)
	return nil
}

func (db *Database) GetMovieLink(movieID int) (MovieLink, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	link, exists := db.MovieLinks[movieID]
	if !exists {
		return MovieLink{}, fmt.Errorf("movie link no encontrado")
	}

	return link, nil
}

// HELPERS
type lineScanner struct {
	file *os.File
	buf  []byte
	pos  int
	line string
}

func newLineScanner(file *os.File) *lineScanner {
	return &lineScanner{
		file: file,
		buf:  make([]byte, 4096),
	}
}

func (s *lineScanner) Scan() bool {
	s.line = ""
	for {
		if s.pos >= len(s.buf) {
			n, err := s.file.Read(s.buf)
			if err != nil || n == 0 {
				return s.line != ""
			}
			s.buf = s.buf[:n]
			s.pos = 0
		}

		for s.pos < len(s.buf) {
			if s.buf[s.pos] == '\n' {
				s.pos++
				return true
			}
			s.line += string(s.buf[s.pos])
			s.pos++
		}
	}
}

func (s *lineScanner) Text() string {
	return s.line
}

func parseCSVLine(line string) []string {
	parts := make([]string, 0)
	current := ""
	inQuotes := false

	for _, c := range line {
		if c == '"' {
			inQuotes = !inQuotes
		} else if c == ',' && !inQuotes {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)

	return parts
}

func parseInt(s string) int {
	result := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}
