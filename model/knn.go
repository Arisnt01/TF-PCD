package model

import (
	"math/rand"
)

func CollaborativeFiltering(matrix map[int]map[int]float64) map[int][]int {
	recs := make(map[int][]int)
	for user := range matrix {
		var movies []int
		for i := 0; i < 20; i++ {
			movies = append(movies, rand.Intn(50000)+1)
		}
		recs[user] = movies
		if len(recs) >= 3 { // solo mostrar 3 usuarios
			break
		}
	}
	return recs
}
