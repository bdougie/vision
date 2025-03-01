package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agent-api/core/pkg/agent"
	"github.com/agent-api/core/types"
	"github.com/agent-api/ollama"

	"log/slog"

	"github.com/lmittmann/tint"
)

func main() {
	ctx := context.Background()

	// Configure logger
	logger := slog.New(
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}),
	)

	// Parse command line arguments
	videoPath := "path/to/your/video.mp4"
	OutputDir = "output_frames" // default value

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--video":
			if i+1 < len(os.Args) {
				videoPath = os.Args[i+1]
				i++
			}
		case "--output":
			if i+1 < len(os.Args) {
				OutputDir = os.Args[i+1]
				i++
			}
		}
	}

	// Ensure video path is provided
	if videoPath == "path/to/your/video.mp4" {
		fmt.Println("Usage: go run . --video path/to/video.mp4 [--output output_directory]")
		os.Exit(1)
	}

	// After parsing the video path, set the VideoName
	VideoName = strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	// Set up Ollama provider
	opts := &ollama.ProviderOpts{
		Logger:  logger,
		BaseURL: "http://localhost",
		Port:    11434,
	}
	provider := ollama.NewProvider(opts)

	// Use the correct model
	model := &types.Model{
		ID: "llama3.2-vision:11b",
	}
	provider.UseModel(ctx, model)

	// Create agent configuration
	agentConf := &agent.NewAgentConfig{
		Provider:     provider,
		Logger:       logger,
		SystemPrompt: "You are a visual analysis assistant specialized in detailed image descriptions. If there is a person in the image describe what they are doing in step by step format.",
	}

	// Initialize agent
	visionAgent := agent.NewAgent(agentConf)

	// Process video
	fmt.Printf("Starting video analysis...\n")
	err := processVideo(ctx, visionAgent, videoPath, OutputDir)
	if err != nil {
		log.Printf("Error processing video: %v", err)
		os.Exit(1)
	}

	fmt.Println("\nVideo processing completed successfully!")
}
