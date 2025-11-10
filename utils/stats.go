package utils

type DatasetStats struct {
	Users             int
	Movies            int
	Ratings           int
	AvgRatingsPerUser float64
}

func BuildUserMovieMatrix(data []Rating) map[int]map[int]float64 {
	matrix := make(map[int]map[int]float64)
	for _, d := range data {
		if matrix[d.UserID] == nil {
			matrix[d.UserID] = make(map[int]float64)
		}
		matrix[d.UserID][d.MovieID] = d.Score
	}
	return matrix
}

func GenerateStats(matrix map[int]map[int]float64) DatasetStats {
	users := len(matrix)
	moviesSet := make(map[int]struct{})
	total := 0
	for _, m := range matrix {
		total += len(m)
		for movie := range m {
			moviesSet[movie] = struct{}{}
		}
	}
	return DatasetStats{
		Users:             users,
		Movies:            len(moviesSet),
		Ratings:           total,
		AvgRatingsPerUser: float64(total) / float64(users),
	}
}
