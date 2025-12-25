package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// Локальный микросервис embeddings генерирует 384-мерные эмбединги
// через sentence-transformers/all-MiniLM-L6-v2
const embeddingsServiceURL = "http://embeddings:8003"

func generateVector(text string) ([]float32, error) {
	url := embeddingsServiceURL + "/embed"

	payload := map[string]interface{}{
		"text": text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}

	var resp *http.Response
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = client.Do(req)
		if err == nil {
			break
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("request to embeddings service failed after %d attempts: %w", maxRetries, lastErr)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings service error [%d]: %s", resp.StatusCode, string(body))
	}

	// Парсим ответ: {"embedding": [0.1, 0.2, ..., 0.3]}
	var result struct {
		Embedding []float64 `json:"embedding"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse embedding response: %w", err)
	}

	if len(result.Embedding) != 384 {
		return nil, fmt.Errorf("unexpected embedding dimension: got %d, expected 384", len(result.Embedding))
	}

	// Конвертируем в float32
	vector := make([]float32, len(result.Embedding))
	for i, v := range result.Embedding {
		vector[i] = float32(v)
	}

	return vector, nil
}
