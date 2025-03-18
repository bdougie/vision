package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/lmittmann/tint"

	"github.com/bdougie/vision/internal/analyzer"
	"github.com/bdougie/vision/internal/storage"
)

func main() {
	ctx := context.Background()

	// Configure logger
	logger := slog.New(
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: "15:04:05",
		}),
	)

	// Parse command line arguments
	videoPath := "path/to/your/video.mp4"
	outputDir := "output_frames"  // default value

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--video":
			if i+1 < len(os.Args) {
				videoPath = os.Args[i+1]
				i++
			}
		case "--output":
			if i+1 < len(os.Args) {
				outputDir = os.Args[i+1]
				i++
			}
		}
	}

	// Ensure video path is provided
	if videoPath == "path/to/your/video.mp4" {
		fmt.Println("Usage: visionanalyzer --video path/to/video.mp4 [--output output_directory]")
		os.Exit(1)
	}

	// After parsing the video path, set the videoName
	videoName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	// Initialize the storage
	store := storage.NewStorage(outputDir, videoName)

	// Initialize agent
	visionAgent, err := analyzer.NewAgent(ctx, logger)
	if err != nil {
		log.Fatalf("Failed to initialize vision agent: %v", err)
	}

	// Process video
	fmt.Printf("Starting video analysis...\n")
	processor := analyzer.NewProcessor(visionAgent, store)
	err = processor.ProcessVideo(ctx, videoPath, outputDir)
	if err != nil {
		log.Printf("Error processing video: %v", err)
		os.Exit(1)
	}

	fmt.Println("Video processing completed successfully!")
}
