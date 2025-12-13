package main

import (
	"log"
	"net/http"
	"os"
)

type Config struct {
	QdrantURL           string
	CollectionName      string
	DataDir             string
	SimilarityThreshold float64
}

var config = Config{
	QdrantURL:           getEnv("QDRANT_URL", "http://qdrant:6333"),
	CollectionName:      getEnv("COLLECTION_NAME", "documents"),
	DataDir:             getEnv("DATA_DIR", "/files/reports"),
	SimilarityThreshold: 0.85,
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting file analysis service...")

	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	if err := os.MkdirAll("/files/reports", 0755); err != nil {
		log.Fatalf("Failed to create reports directory: %v", err)
	}

	if err := initializeQdrant(); err != nil {
		log.Fatalf("Failed to initialize Qdrant: %v", err)
	}

	http.HandleFunc("/analyze", handleAnalyze)
	http.HandleFunc("/reports/", handleGetReports)
	http.HandleFunc("/health", handleHealthCheck)

	log.Println("File analysis service is running on :8002")
	if err := http.ListenAndServe(":8002", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
