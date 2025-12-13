package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type Match struct {
	ID        string  `json:"id"`
	FileName  string  `json:"file_name"`
	Sender    string  `json:"sender"`
	Score     float64 `json:"score"`
	Timestamp string  `json:"timestamp"`
}

func storeDocument(sender, workID, fileName string, vector []float32) (string, error) {
	h := sha1.Sum([]byte(fmt.Sprintf("%s_%s_%s", sender, workID, fileName)))
	docID := int64(binary.BigEndian.Uint64(h[:8]) & 0x7FFFFFFFFFFFFFFF)
	docIDStr := fmt.Sprintf("%d", docID)

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

	client := &http.Client{Timeout: 15 * time.Second}

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

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var matches []Match
	for _, item := range result.Result {
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
