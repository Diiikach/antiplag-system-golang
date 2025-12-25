package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	fileStoringURL := getEnv("FILE_STORING_URL", "http://file_storing:8001")
	fileAnalysisURL := getEnv("FILE_ANALYSIS_URL", "http://file_analysis:8002")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/submit", handleSubmit(fileStoringURL, fileAnalysisURL))
	mux.HandleFunc("/api/works/", handleReports(fileAnalysisURL))
	mux.HandleFunc("/health", handleGatewayHealth)

	server := &http.Server{
		Addr:         ":8000",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		Handler:      corsMiddleware(mux),
	}

	log.Println("gateway running at :8000")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
