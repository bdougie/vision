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

// FileStorage implements Storage interface for file-based storage
type FileStorage struct {
    outputDir string
    videoName string
    results   []models.AnalysisResult
    mu        sync.Mutex  // Add mutex for thread safety
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(outputDir, videoName string) *FileStorage {
    return &FileStorage{
        outputDir: outputDir,
        videoName: videoName,
        results:   []models.AnalysisResult{},
    }
}

// AddResult adds a single analysis result
func (s *FileStorage) AddResult(ctx context.Context, result models.AnalysisResult) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.results = append(s.results, result)
    return nil
}

// Flush writes all results to a JSON file
func (s *FileStorage) Flush() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if len(s.results) == 0 {
        return nil
    }
    
    // Create output directory if it doesn't exist
    frameDirPath := filepath.Join(s.outputDir, s.videoName)
    if err := os.MkdirAll(frameDirPath, 0755); err != nil {
        return fmt.Errorf("failed to create output directory: %w", err)
    }
    
    // Create the results file
    filePath := filepath.Join(frameDirPath, "analysis_results.json")
    
    // Convert results to JSON
    data, err := json.MarshalIndent(s.results, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal results: %w", err)
    }
    
    // Write to file
    if err := os.WriteFile(filePath, data, 0644); err != nil {
        return fmt.Errorf("failed to write results file: %w", err)
    }
    
    fmt.Printf("Saved %d analysis results to %s\n", len(s.results), filePath)
    s.results = nil // Clear after saving
    return nil
}
