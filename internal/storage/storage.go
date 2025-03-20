package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/bdougie/vision/internal/models"
)

const batchSize = 10 // Number of results to batch write

// Storage defines the interface for storing analysis results
type Storage interface {
	// AddResult adds a single analysis result
	AddResult(ctx context.Context, result models.AnalysisResult) error

	// Flush ensures all pending results are saved
	Flush() error
}

// storageImpl manages saving and retrieving analysis results
type storageImpl struct {
	results    []models.AnalysisResult
	mu         sync.Mutex
	outputDir  string
	videoName  string
}

// NewStorage creates a new storage manager
func NewStorage(outputDir, videoName string) *storageImpl {
	return &storageImpl{
		results:   []models.AnalysisResult{},
		outputDir: outputDir,
		videoName: videoName,
	}
}

// AddResult adds a result to the batch and flushes if the batch is full
func (s *storageImpl) AddResult(ctx context.Context, result models.AnalysisResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = append(s.results, result)

	// Write to disk when batch is full
	if len(s.results) >= batchSize {
		if err := s.flush(); err != nil {
			fmt.Printf("Error flushing results: %v\n", err)
			return err
		}
	}
	return nil
}

// Flush writes all pending results to disk
func (s *storageImpl) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flush()
}

// Internal flush implementation
func (s *storageImpl) flush() error {
	if len(s.results) == 0 {
		return nil
	}

	resultsFilePath := filepath.Join(s.outputDir, s.videoName, "analysis_results.json")

	var existingResults []models.AnalysisResult
	if data, err := os.ReadFile(resultsFilePath); err == nil {
		if err := json.Unmarshal(data, &existingResults); err != nil {
			return fmt.Errorf("failed to unmarshal existing results: %v", err)
		}
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
func (s *storageImpl) SaveResult(result models.AnalysisResult) error {
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
