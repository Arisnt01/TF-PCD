package main

import (
	"TF/model"
	"TF/utils"
	"fmt"
	"time"
)

func main() {
	start := time.Now()
	utils.Info("Iniciando limpieza concurrente y generación de matriz...")

	data := utils.LoadDatasetConcurrent("data_25M/movies.csv", 8)
	utils.Info(fmt.Sprintf("Datos cargados concurrentemente: %d registros", len(data)))

	utils.Info("Generando matriz usuario-película...")
	matrix := utils.BuildUserMovieMatrix(data)
	utils.Info("Matriz generada correctamente.")

	utils.Info("Generando estadísticas del dataset...")
	stats := utils.GenerateStats(matrix)
	fmt.Printf("\n📊 Estadísticas del dataset:\n"+
		"Usuarios únicos: %d\nPelículas únicas: %d\nValoraciones totales: %d\n"+
		"Promedio de valoraciones por usuario: %.2f\n",
		stats.Users, stats.Movies, stats.Ratings, stats.AvgRatingsPerUser)

	fmt.Println("Limpieza completa. Iniciando filtrado colaborativo...")
	utils.Info("Ejecutando filtrado colaborativo...")
	recs := model.CollaborativeFiltering(matrix)
	utils.Info("Filtrado colaborativo completado.")

	fmt.Println("Recomendaciones generadas:")
	for user, movies := range recs {
		fmt.Printf("Usuario %d → Películas recomendadas: %v\n", user, movies)
	}

	fmt.Printf("\nTiempo total: %v\n", time.Since(start))
}
