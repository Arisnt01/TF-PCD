package main

import (
	"fmt"
	"movie_recommender/model"
	"movie_recommender/utils"
	"time"
)

func main() {
	start := time.Now()

	fmt.Println("Cargando dataset y limpiando datos...")
	ratings := model.LoadRatings("data_25M/ratings.csv")

	fmt.Println("Generando matriz usuario–película...")
	userMatrix := model.BuildUserMovieMatrix(ratings)

	fmt.Println("Calculando similitudes (User-based)...")
	sims := model.ComputeSimilaritiesParallel(userMatrix, 8) // 8 goroutines

	fmt.Println("Generando recomendaciones...")
	recs := model.GenerateRecommendations(sims, userMatrix, 5)

	fmt.Println("Proceso completado en:", utils.Elapsed(start))
	fmt.Println("Ejemplo de recomendaciones:", recs[:5])
}
