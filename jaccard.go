package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

type UserMovies map[string]bool
type Dataset map[string]UserMovies
type MovieMap map[string]string

func jaccardIndex(setA, setB map[string]bool) float64 {
	intersection := 0
	union := make(map[string]struct{})

	for item := range setA {
		union[item] = struct{}{}
		if setB[item] {
			intersection++
		}
	}
	for item := range setB {
		union[item] = struct{}{}
	}

	if len(union) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(union))
}

func loadRatings(filePath string) (Dataset, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data := make(Dataset)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "::")
		if len(parts) < 3 {
			continue
		}
		userID := parts[0]
		movieID := parts[1]

		if _, ok := data[userID]; !ok {
			data[userID] = make(UserMovies)
		}
		data[userID][movieID] = true
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return data, nil
}

func loadMovies(filePath string) (MovieMap, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	movies := make(MovieMap)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "::")
		if len(parts) < 2 {
			continue
		}
		movieID := parts[0]
		title := parts[1]
		movies[movieID] = title
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return movies, nil
}

func recommendMovies(data Dataset, movies MovieMap, target string, topN int) {
	if _, ok := data[target]; !ok {
		fmt.Println("El usuario no existe en el dataset.")
		return
	}

	similarities := make(map[string]float64)

	for user, movieSet := range data {
		if user == target {
			continue
		}
		score := jaccardIndex(data[target], movieSet)
		if score > 0 {
			similarities[user] = score
		}
	}

	type pair struct {
		User  string
		Score float64
	}
	var sorted []pair
	for user, score := range similarities {
		sorted = append(sorted, pair{user, score})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })

	if len(sorted) == 0 {
		fmt.Println("No se encontraron usuarios similares.")
		return
	}

	fmt.Println("\n Usuarios con más similitudes:")
	for i := 0; i < topN && i < len(sorted); i++ {
		fmt.Printf("  %s (%.3f)\n", sorted[i].User, sorted[i].Score)
	}
	recommendations := make(map[string]int)
	for i := 0; i < topN && i < len(sorted); i++ {
		neighbor := sorted[i].User
		for movie := range data[neighbor] {
			if !data[target][movie] {
				recommendations[movie]++
			}
		}
	}

	if len(recommendations) == 0 {
		fmt.Println("\nNo hay nuevas películas para recomendar.")
		return
	}

	type rec struct {
		MovieID string
		Count   int
	}
	var recList []rec
	for movie, count := range recommendations {
		recList = append(recList, rec{movie, count})
	}
	sort.Slice(recList, func(i, j int) bool { return recList[i].Count > recList[j].Count })

	fmt.Println("\nRecomendaciones para el usuario", target, ":")
	for i := 0; i < 10 && i < len(recList); i++ {
		title := movies[recList[i].MovieID]
		fmt.Printf("  %d. %s ( %d usuarios similares)\n", i+1, title, recList[i].Count)
	}
}

func main() {

	ratingsPath := "ratings.dat"
	moviesPath := "movies.dat"

	data, err := loadRatings(ratingsPath)
	if err != nil {
		fmt.Println("Error al leer ratings:", err)
		return
	}
	movies, err := loadMovies(moviesPath)
	if err != nil {
		fmt.Println("Error al leer movies:", err)
		return
	}

	fmt.Printf("Dataset cargado: %d usuarios\n", len(data))
	targetUser := "1"
	recommendMovies(data, movies, targetUser, 5)
}
