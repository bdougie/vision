package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/bdougie/vision/internal/models"
)

const batchSize = 10 // Number of results to batch write

// Storage manages saving and retrieving analysis results
type Storage struct {
	results    []models.AnalysisResult
	mu         sync.Mutex
	outputDir  string
	videoName  string
}

// NewStorage creates a new storage manager
func NewStorage(outputDir, videoName string) *Storage {
	return &Storage{
		results:   []models.AnalysisResult{},
		outputDir: outputDir,
		videoName: videoName,
	}
}

// AddResult adds a result to the batch and flushes if the batch is full
func (s *Storage) AddResult(result models.AnalysisResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = append(s.results, result)

	// Write to disk when batch is full
	if len(s.results) >= batchSize {
		if err := s.flush(); err != nil {
			fmt.Printf("Error flushing results: %v\n", err)
		}
	}
}

// Flush writes all pending results to disk
func (s *Storage) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flush()
}

// Internal flush implementation
func (s *Storage) flush() error {
	if len(s.results) == 0 {
		return nil
	}

	resultsFilePath := filepath.Join(s.outputDir, s.videoName, "analysis_results.json")

	var existingResults []models.AnalysisResult
	if data, err := os.ReadFile(resultsFilePath); err == nil {
		json.Unmarshal(data, &existingResults)
	}

	allResults := append(existingResults, s.results...)

	// Create directory if it doesn't exist
	dir := filepath.Dir(resultsFilePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for results: %v", err)
		}
	}

	file, err := os.Create(resultsFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(allResults); err != nil {
		return err
	}

	s.results = nil // Clear the batch
	return nil
}

// SaveResult saves a single result directly to disk
func (s *Storage) SaveResult(result models.AnalysisResult) error {
	resultsFilePath := filepath.Join(s.outputDir, s.videoName, "analysis_results.json")

	var results []models.AnalysisResult

	// Read existing results if the file exists
	if _, err := os.Stat(resultsFilePath); err == nil {
		file, err := os.ReadFile(resultsFilePath)
		if err != nil {
			return fmt.Errorf("failed to read results file: %v", err)
		}
		if err := json.Unmarshal(file, &results); err != nil {
			return fmt.Errorf("failed to unmarshal results: %v", err)
		}
	}

	// Append new result
	results = append(results, result)

	// Create directory if it doesn't exist
	dir := filepath.Dir(resultsFilePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for results: %v", err)
		}
	}

	// Write updated results to file
	file, err := os.Create(resultsFilePath)
	if err != nil {
		return fmt.Errorf("failed to create results file: %v", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(results); err != nil {
		return fmt.Errorf("failed to encode results: %v", err)
	}

	return nil
}
