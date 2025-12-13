package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func extractKeywords(text string) map[string]int {
	text = strings.ToLower(text)
	reg := regexp.MustCompile(`[^a-zа-я0-9\s]`)
	text = reg.ReplaceAllString(text, " ")

	words := strings.Fields(text)

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
		"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
		"will": true, "would": true, "could": true, "should": true, "may": true, "might": true,
		"can": true, "this": true, "that": true, "these": true, "those": true,
		"i": true, "you": true, "he": true, "she": true, "it": true, "we": true, "they": true,
		"что": true, "это": true, "как": true, "то": true, "все": true, "если": true,
		"он": true, "она": true, "они": true, "мы": true, "вы": true, "я": true,
		"и": true, "или": true, "но": true, "не": true, "да": true, "нет": true,
		"в": true, "на": true, "за": true, "по": true, "от": true, "с": true,
	}

	wordFreq := make(map[string]int)
	for _, word := range words {
		if len(word) > 2 && !stopWords[word] {
			wordFreq[word]++
		}
	}

	return wordFreq
}

func downloadAndSaveWordCloud(text string) (string, error) {
	keywords := extractKeywords(text)

	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range keywords {
		sorted = append(sorted, kv{k, v})
	}

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Value > sorted[i].Value {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	limit := 30
	if len(sorted) < limit {
		limit = len(sorted)
	}

	labels := make([]string, limit)
	for i := 0; i < limit; i++ {
		labels[i] = sorted[i].Key
	}

	textParam := strings.Join(labels, " ")
	encodedText := url.QueryEscape(textParam)
	chartURL := fmt.Sprintf("https://quickchart.io/wordcloud?text=%s&width=800&height=600&format=png", encodedText)

	log.Printf("Downloading word cloud from: %s", chartURL)

	resp, err := http.Get(chartURL)
	if err != nil {
		return "", fmt.Errorf("failed to download word cloud: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download word cloud: status %d", resp.StatusCode)
	}

	imageData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read word cloud data: %w", err)
	}

	tmpDir := "/tmp"
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("wordcloud_%d.png", time.Now().UnixNano()))

	if err := ioutil.WriteFile(tmpFile, imageData, 0644); err != nil {
		return "", fmt.Errorf("failed to save word cloud image: %w", err)
	}

	log.Printf("Word cloud saved to: %s", tmpFile)
	return tmpFile, nil
}
