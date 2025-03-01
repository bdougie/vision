package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agent-api/core/pkg/agent"
)

func analyzeImage(ctx context.Context, a *agent.DefaultAgent, imagePath, outputDir, videoName string) (string, error) {
	// Create vision prompt with image data
	response := a.Run(
		ctx,
		agent.WithInput("What is happening in this image? Be specific and detailed. List item and describe items shown in the video."),
		agent.WithImagePath(imagePath),
	)
	if response.Err != nil {
		return "", response.Err
	}

	// Extract the actual response content
	if len(response.Messages) == 0 {
		return "", fmt.Errorf("no response messages received from model")
	}

	// Get the model's response (not the prompt)
	content := response.Messages[len(response.Messages)-1].Content

	// Debug log to see what we're getting
	fmt.Printf("Raw response content: %s\n", content)

	// Save analysis result
	result := AnalysisResult{
		Frame:   filepath.Base(imagePath),
		Content: content,
	}
	if err := saveAnalysisResult(outputDir, videoName, result); err != nil {
		return "", err
	}

	return content, nil
}

func saveAnalysisResult(outputDir, videoName string, result AnalysisResult) error {
	resultsFilePath := filepath.Join(outputDir, videoName, "analysis_results.json")

	var results []AnalysisResult

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
