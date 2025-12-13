package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"
)

type Report struct {
	FileName    string    `json:"file_name"`
	Sender      string    `json:"sender"`
	WorkID      string    `json:"work_id"`
	Plagiarized bool      `json:"plagiarized"`
	Similarity  float64   `json:"similarity,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Error       string    `json:"error,omitempty"`
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

func getCurrentTime() time.Time {
	return time.Now()
}
