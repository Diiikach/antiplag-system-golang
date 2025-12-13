package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FileName string `json:"file_name"`
		Sender   string `json:"sender"`
		WorkID   string `json:"work_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	content, err := ioutil.ReadFile(filepath.Join("/files", req.FileName))
	if err != nil {
		log.Printf("Error reading file %s: %v", req.FileName, err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	vector, err := generateVector(string(content))
	if err != nil {
		log.Printf("Error generating vector: %v", err)
		http.Error(w, "Failed to process document", http.StatusInternalServerError)
		return
	}

	_, err = storeDocument(req.Sender, req.WorkID, req.FileName, vector)
	if err != nil {
		log.Printf("Error storing document: %v", err)
		http.Error(w, "Failed to store document", http.StatusInternalServerError)
		return
	}

	matches, err := findSimilarDocuments(req.WorkID, req.Sender, vector)
	if err != nil {
		log.Printf("Error finding similar documents: %v", err)
		http.Error(w, "Failed to analyze document", http.StatusInternalServerError)
		return
	}

	var similarity float64
	if len(matches) > 0 {
		similarity = matches[0].Score
	}

	report := Report{
		FileName:    req.FileName,
		Sender:      req.Sender,
		WorkID:      req.WorkID,
		Plagiarized: len(matches) > 0,
		Similarity:  similarity,
		Timestamp:   getCurrentTime(),
	}

	if err := saveReport(report); err != nil {
		log.Printf("Error saving report: %v", err)
	}

	imagePath, err := downloadAndSaveWordCloud(string(content))
	if err != nil {
		log.Printf("Error generating word cloud: %v", err)
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"similarity":  similarity,
			"plagiarized": len(matches) > 0,
			"error":       "Failed to generate word cloud",
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	defer os.Remove(imagePath)

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "attachment; filename=wordcloud.png")

	imageData, err := ioutil.ReadFile(imagePath)
	if err != nil {
		log.Printf("Error reading image file: %v", err)
		http.Error(w, "Failed to read image", http.StatusInternalServerError)
		return
	}

	w.Write(imageData)
}

func handleGetReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workID := strings.TrimPrefix(r.URL.Path, "/reports/")
	if workID == "" {
		http.Error(w, "Work ID is required", http.StatusBadRequest)
		return
	}

	files, err := ioutil.ReadDir(config.DataDir)
	if err != nil {
		http.Error(w, "Failed to read reports", http.StatusInternalServerError)
		return
	}

	var reports []Report
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		data, err := ioutil.ReadFile(filepath.Join(config.DataDir, file.Name()))
		if err != nil {
			log.Printf("Error reading report %s: %v", file.Name(), err)
			continue
		}

		var report Report
		if err := json.Unmarshal(data, &report); err != nil {
			log.Printf("Error parsing report %s: %v", file.Name(), err)
			continue
		}

		if report.WorkID == workID {
			reports = append(reports, report)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if len(reports) == 0 {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]Report{})
		return
	}

	if err := json.NewEncoder(w).Encode(reports); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
