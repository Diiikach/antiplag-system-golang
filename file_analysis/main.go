package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config holds the service configuration
type Config struct {
	QdrantURL           string
	CollectionName      string
	DataDir             string
	SimilarityThreshold float64
}

// Report represents an analysis report
type Report struct {
	FileName    string    `json:"file_name"`
	Sender      string    `json:"sender"`
	WorkID      string    `json:"work_id"`
	Plagiarized bool      `json:"plagiarized"`
	Similarity  float64   `json:"similarity,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Error       string    `json:"error,omitempty"`
}

// Initialize configuration
var config = Config{
	QdrantURL:           getEnv("QDRANT_URL", "http://qdrant:6333"),
	CollectionName:      getEnv("COLLECTION_NAME", "documents"),
	DataDir:             getEnv("DATA_DIR", "/files/reports"),
	SimilarityThreshold: 0.85,
}

func main() {
	// Initialize logger
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting file analysis service...")

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Also ensure /files directory exists for reports
	if err := os.MkdirAll("/files/reports", 0755); err != nil {
		log.Fatalf("Failed to create reports directory: %v", err)
	}

	// Initialize Qdrant collection
	if err := initializeQdrant(); err != nil {
		log.Fatalf("Failed to initialize Qdrant: %v", err)
	}

	// Set up HTTP server
	http.HandleFunc("/analyze", handleAnalyze)
	http.HandleFunc("/reports/", handleGetReports)
	http.HandleFunc("/health", handleHealthCheck)

	log.Println("File analysis service is running on :8002")
	if err := http.ListenAndServe(":8002", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

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

	// Parse request
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

	// Read file content
	content, err := ioutil.ReadFile(filepath.Join("/files", req.FileName))
	if err != nil {
		log.Printf("Error reading file %s: %v", req.FileName, err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Generate document vector
	vector, err := generateVector(string(content))
	if err != nil {
		log.Printf("Error generating vector: %v", err)
		http.Error(w, "Failed to process document", http.StatusInternalServerError)
		return
	}

	// Store document in Qdrant
	if err != nil {
		log.Printf("Error storing document: %v", err)
		http.Error(w, "Failed to store document", http.StatusInternalServerError)
		return
	}

	// Find similar documents
	matches, err := findSimilarDocuments(req.WorkID, req.Sender, vector)
	if err != nil {
		log.Printf("Error finding similar documents: %v", err)
		http.Error(w, "Failed to analyze document", http.StatusInternalServerError)
		return
	}

	// Calculate similarity score
	var similarity float64
	if len(matches) > 0 {
		similarity = matches[0].Score
	}

	// Save report
	report := Report{
		FileName:    req.FileName,
		Sender:      req.Sender,
		WorkID:      req.WorkID,
		Plagiarized: len(matches) > 0,
		Similarity:  similarity,
		Timestamp:   time.Now(),
	}

	if err := saveReport(report); err != nil {
		log.Printf("Error saving report: %v", err)
		// Continue even if report save fails
	}

	// Return only similarity value
	w.Header().Set("Content-Type", "application/json")
	response := map[string]float64{
		"similarity": similarity,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// Helper functions...

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func generateVector(text string) ([]float32, error) {
	h := sha1.Sum([]byte(text))
	vector := make([]float32, 384) // Qdrant expects 384-dimensional vectors

	// Fill the vector with deterministic values based on the hash
	for i := 0; i < 384; i++ {
		// Use different parts of the hash to fill the vector
		hashIndex := i % len(h)
		vector[i] = float32(h[hashIndex]) / 255.0

		// Add some variation based on position
		if i%2 == 0 {
			vector[i] = (vector[i] + float32(i%10)/10.0) / 2.0
		}
	}

	// Ensure the vector is normalized (unit length)
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

func initializeQdrant() error {
	// Wait for Qdrant to be ready
	log.Println("Waiting for Qdrant to be ready...")
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		// Qdrant doesn't have /health endpoint, use /collections instead
		resp, err := http.Get(config.QdrantURL + "/collections")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				log.Println("Qdrant is ready")
				break
			}
		}
		if i == maxRetries-1 {
			return fmt.Errorf("qdrant is not available after %d attempts", maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	// Check if collection exists
	url := fmt.Sprintf("%s/collections/%s", strings.TrimSuffix(config.QdrantURL, "/"), config.CollectionName)

	// First, try to get collection info (use GET instead of HEAD)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}
	resp.Body.Close()

	// If collection doesn't exist, create it
	if resp.StatusCode == http.StatusNotFound {
		log.Printf("Collection %s not found, creating...", config.CollectionName)

		collectionConfig := map[string]interface{}{
			"vectors": map[string]interface{}{
				"size":     384, // Match the vector dimension
				"distance": "Cosine",
			},
		}

		jsonData, err := json.Marshal(collectionConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal collection config: %w", err)
		}

		req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			// If collection already exists (race condition), it's not an error
			if strings.Contains(string(body), "already exists") {
				log.Printf("Collection %s already exists, continuing...", config.CollectionName)
				return nil
			}
			return fmt.Errorf("failed to create collection [%d]: %s", resp.StatusCode, string(body))
		}
		log.Printf("Successfully created collection %s", config.CollectionName)
	} else if resp.StatusCode == http.StatusOK {
		log.Printf("Collection %s already exists, skipping creation", config.CollectionName)
	} else {
		return fmt.Errorf("unexpected status code while checking collection: %d", resp.StatusCode)
	}

	return nil
}

// ... [Previous code remains the same until the storeDocument function]

func storeDocument(sender, workID, fileName string, vector []float32) (string, error) {
	// Генерируем числовой ID на основе хеша от sender и workID
	h := sha1.Sum([]byte(fmt.Sprintf("%s_%s_%s", sender, workID, fileName)))
	// Берем первые 8 байт хеша и делаем их положительными
	docID := int64(binary.BigEndian.Uint64(h[:8]) & 0x7FFFFFFFFFFFFFFF)
	docIDStr := fmt.Sprintf("%d", docID) // Конвертируем в строку для возврата

	// Create the point payload according to Qdrant's API
	point := map[string]interface{}{
		"id":     docID,
		"vector": vector,
		"payload": map[string]interface{}{
			"sender":    sender,
			"work_id":   workID,
			"file_name": fileName,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}

	// Create the upsert operation payload
	payload := map[string]interface{}{
		"points": []map[string]interface{}{point},
	}

	url := fmt.Sprintf("%s/collections/%s/points?wait=true",
		strings.TrimSuffix(config.QdrantURL, "/"),
		config.CollectionName)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add timeout and retry logic
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	var resp *http.Response
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = client.Do(req)
		if err == nil {
			break
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}

	if err != nil {
		return "", fmt.Errorf("request to Qdrant failed after %d attempts: %w", maxRetries, lastErr)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Qdrant API error [%d]: %s - %s",
			resp.StatusCode,
			http.StatusText(resp.StatusCode),
			strings.TrimSpace(string(body)))
	}

	return docIDStr, nil
}

type Match struct {
	ID        string  `json:"id"`
	FileName  string  `json:"file_name"`
	Sender    string  `json:"sender"`
	Score     float64 `json:"score"`
	Timestamp string  `json:"timestamp"`
}

func findSimilarDocuments(workID, excludeSender string, vector []float32) ([]Match, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search",
		strings.TrimSuffix(config.QdrantURL, "/"),
		config.CollectionName)

	payload := map[string]interface{}{
		"vector":          vector,
		"limit":           5,
		"with_payload":    true,
		"with_vectors":    false,
		"score_threshold": 0.7,
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{
				{
					"key": "work_id",
					"match": map[string]interface{}{
						"value": workID,
					},
				},
			},
			"must_not": []map[string]interface{}{
				{
					"key": "sender",
					"match": map[string]interface{}{
						"value": excludeSender,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	type searchResultItem struct {
		ID      uint64      `json:"id"`
		Version int         `json:"version"`
		Score   float64     `json:"score"`
		Payload interface{} `json:"payload"`
	}

	var result struct {
		Result []searchResultItem `json:"result"`
		Status string             `json:"status"`
		Time   float64            `json:"time"`
	}

	// Parse the JSON response using already read body
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var matches []Match
	for _, item := range result.Result {
		// Safely extract values from the payload
		payload, ok := item.Payload.(map[string]interface{})
		if !ok {
			continue
		}

		fileName, _ := payload["file_name"].(string)
		sender, _ := payload["sender"].(string)
		timestamp, _ := payload["timestamp"].(string)

		matches = append(matches, Match{
			ID:        fmt.Sprintf("%d", item.ID),
			FileName:  fileName,
			Sender:    sender,
			Score:     item.Score,
			Timestamp: timestamp,
		})
	}

	return matches, nil
}

func saveReport(report Report) error {
	reportPath := filepath.Join(config.DataDir, fmt.Sprintf("report_%s_%s_%d.json",
		report.Sender,
		report.WorkID,
		time.Now().UnixNano()))

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := ioutil.WriteFile(reportPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
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

	// List all report files for the work
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

	if len(reports) == 0 {
		http.Error(w, "No reports found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(reports); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
