package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// CORS middleware для поддержки запросов с frontend
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleSubmit(fileStoringURL, fileAnalysisURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.ParseMultipartForm(20 << 20)
		file, handler, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Invalid file upload", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Upload file to file_storing service
		buf := &bytes.Buffer{}
		mw := multipart.NewWriter(buf)
		fw, err := mw.CreateFormFile("file", handler.Filename)
		if err != nil {
			http.Error(w, "Failed to create form file", http.StatusInternalServerError)
			return
		}
		io.Copy(fw, file)
		mw.Close()

		req, _ := http.NewRequest("POST", fileStoringURL+"/upload", buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			http.Error(w, "Failed to upload file to storage service", http.StatusBadGateway)
			return
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		// Call file_analysis service
		analysisData := map[string]string{
			"file_name": handler.Filename,
			"sender":    r.FormValue("sender"),
			"work_id":   r.FormValue("work_id"),
		}
		body, _ := json.Marshal(analysisData)
		resp2, err := http.Post(fileAnalysisURL+"/analyze", "application/json", bytes.NewReader(body))
		if err != nil {
			http.Error(w, "Failed to call analysis service", http.StatusBadGateway)
			return
		}

		io.Copy(w, resp2.Body)
		resp2.Body.Close()
	}
}

func handleReports(fileAnalysisURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 {
			http.Error(w, "Invalid request path", http.StatusBadRequest)
			return
		}
		workid := parts[len(parts)-2]

		url := fileAnalysisURL + "/reports/" + workid
		resp, err := http.Get(url)
		if err != nil {
			http.Error(w, "Failed to reach analysis service", http.StatusBadGateway)
			return
		}

		if resp.StatusCode != http.StatusOK {
			http.Error(w, "Report not found", http.StatusNotFound)
			return
		}

		io.Copy(w, resp.Body)
		resp.Body.Close()
	}
}

func handleGatewayHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
