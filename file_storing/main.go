package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// Create upload directory if it doesn't exist
	uploadDir := "/files"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	// Set up HTTP server with timeouts
	server := &http.Server{
		Addr:         ":8001",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		Handler:      http.DefaultServeMux,
	}

	// File upload endpoint
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse multipart form with a reasonable max memory (32MB)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Get the file from the request
		file, handler, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Invalid file upload: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Create the file on the server
		fpath := filepath.Join(uploadDir, handler.Filename)
		out, err := os.Create(fpath)
		if err != nil {
			log.Printf("Failed to create file %s: %v", fpath, err)
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		// Copy the file content
		if _, err := io.Copy(out, file); err != nil {
			log.Printf("Failed to save file content %s: %v", fpath, err)
			http.Error(w, "Failed to save file content", http.StatusInternalServerError)
			// Try to remove the partially written file
			os.Remove(fpath)
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","filename":"` + handler.Filename + `"}`))
		log.Printf("Successfully uploaded file: %s", handler.Filename)
	})

	// File download endpoint
	http.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fname := filepath.Base(r.URL.Path)
		if fname == "" || fname == "." || fname == "/" {
			http.Error(w, "Invalid filename", http.StatusBadRequest)
			return
		}

		fpath := filepath.Join(uploadDir, fname)
		fileInfo, err := os.Stat(fpath)
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("Error accessing file %s: %v", fpath, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Set headers for file download
		w.Header().Set("Content-Disposition", "attachment; filename="+fname)
		http.ServeFile(w, r, fpath)
		log.Printf("Served file: %s (size: %d bytes)", fname, fileInfo.Size())
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Start the server
	log.Println("Starting file storage service on :8001")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
