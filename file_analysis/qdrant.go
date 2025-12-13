package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

func initializeQdrant() error {
	log.Println("Waiting for Qdrant to be ready...")
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
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

	url := fmt.Sprintf("%s/collections/%s", strings.TrimSuffix(config.QdrantURL, "/"), config.CollectionName)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return createQdrantCollection(url)
	} else if resp.StatusCode == http.StatusOK {
		log.Printf("Collection %s already exists, skipping creation", config.CollectionName)
	} else {
		return fmt.Errorf("unexpected status code while checking collection: %d", resp.StatusCode)
	}

	return nil
}

func createQdrantCollection(url string) error {
	log.Printf("Collection %s not found, creating...", config.CollectionName)

	collectionConfig := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     384,
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
		if strings.Contains(string(body), "already exists") {
			log.Printf("Collection %s already exists, continuing...", config.CollectionName)
			return nil
		}
		return fmt.Errorf("failed to create collection [%d]: %s", resp.StatusCode, string(body))
	}
	log.Printf("Successfully created collection %s", config.CollectionName)
	return nil
}
