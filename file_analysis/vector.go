package main

import (
	"crypto/sha1"
	"math"
)

func generateVector(text string) ([]float32, error) {
	h := sha1.Sum([]byte(text))
	vector := make([]float32, 384)

	for i := 0; i < 384; i++ {
		hashIndex := i % len(h)
		vector[i] = float32(h[hashIndex]) / 255.0

		if i%2 == 0 {
			vector[i] = (vector[i] + float32(i%10)/10.0) / 2.0
		}
	}

	var sumSq float64
	for _, v := range vector {
		sumSq += float64(v * v)
	}

	if sumSq > 0 {
		norm := float32(math.Sqrt(sumSq))
		for i := range vector {
			vector[i] /= norm
		}
	}

	return vector, nil
}
