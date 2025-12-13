package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	uploadDir := "/files"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	server := &http.Server{
		Addr:         ":8001",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		Handler:      http.DefaultServeMux,
	}

	http.HandleFunc("/upload", handleUpload(uploadDir))
	http.HandleFunc("/files/", handleDownload(uploadDir))
	http.HandleFunc("/health", handleHealth)

	log.Println("Starting file storage service on :8001")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
