package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/go-logr/logr"
	"github.com/lmittmann/tint"

	"github.com/bdougie/vision/internal/analyzer"
	"github.com/bdougie/vision/internal/storage"
)

func main() {
	ctx := context.Background()

	// Configure logger
	logger := logr.FromSlogHandler(
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: "15:04:05",
		}),
	)

	// Parse command line arguments
	videoPath := "path/to/your/video.mp4"
	outputDir := "output_frames" // default value

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
	visionAgent, err := analyzer.NewAgent(ctx, &logger)
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

	// Add these flags to your main function
	searchQuery := flag.String("search", "", "Search for frames matching this description")
	searchLimit := flag.Int("limit", 5, "Maximum number of search results")
	flag.Parse()

	// Add this after processing video
	if *searchQuery != "" && dbEnabled {
		fmt.Printf("Searching for frames matching: %s\n", *searchQuery)

		// Get PostgreSQL configuration
		pgConfig := storage.PostgresConfig{
			Host:     getEnvOrDefault("DB_HOST", "localhost"),
			Port:     getEnvOrDefault("DB_PORT", "5432"),
			User:     getEnvOrDefault("DB_USER", "postgres"),
			Password: getEnvOrDefault("DB_PASSWORD", "postgres"),
			DBName:   getEnvOrDefault("DB_NAME", "vision_analysis"),
		}

		// Create PostgreSQL storage
		pgStorage, err := storage.NewPostgresStorage(ctx, pgConfig, videoName)
		if err != nil {
			log.Fatalf("Failed to create PostgreSQL storage: %v", err)
		}
		defer pgStorage.Close()

		// Search for similar frames
		results, err := pgStorage.SearchSimilarFrames(ctx, *searchQuery, *searchLimit)
		if err != nil {
			log.Fatalf("Failed to search for similar frames: %v", err)
		}

		// Display results
		fmt.Printf("Found %d matching frames:\n", len(results))
		for i, result := range results {
			fmt.Printf("%d. Frame %d (%.2f%% similarity)\n", i+1, result.FrameNumber, result.Similarity*100)
			fmt.Printf("   Description: %s\n\n", result.Description)
		}
	}
}
