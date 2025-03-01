package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type WorkItem struct {
	framePath string
	frameNum  int
	total     int
}

type AnalysisResult struct {
	Frame   string `json:"frame"`
	Content string `json:"content"`
}

type AnalysisResultBatch struct {
	results []AnalysisResult
	mu      sync.Mutex
}

func (batch *AnalysisResultBatch) addResult(result AnalysisResult) {
	batch.mu.Lock()
	defer batch.mu.Unlock()
	batch.results = append(batch.results, result)

	// Write to disk when batch is full
	if len(batch.results) >= BatchSize {
		if err := batch.flush(OutputDir, VideoName); err != nil {
			log.Printf("Error flushing results: %v", err)
		}
	}
}

func (batch *AnalysisResultBatch) flush(outputDir, videoName string) error {
	if len(batch.results) == 0 {
		return nil
	}

	resultsFilePath := filepath.Join(outputDir, videoName, "analysis_results.json")

	var existingResults []AnalysisResult
	if data, err := os.ReadFile(resultsFilePath); err == nil {
		json.Unmarshal(data, &existingResults)
	}

	allResults := append(existingResults, batch.results...)

	file, err := os.Create(resultsFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(allResults); err != nil {
		return err
	}

	batch.results = nil // Clear the batch
	return nil
}
