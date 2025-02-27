package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agent-api/core/pkg/agent"
	"github.com/agent-api/core/types"
	"github.com/agent-api/ollama"

	"log/slog"

	"github.com/lmittmann/tint"
)

func extractFrames(videoPath, outputDir string, interval int) error {
	// Check if video file exists
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("video file does not exist at path: '%s'", videoPath)
	}

	// Create output directory if it doesn't exist
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create output directory '%s': %v", outputDir, err)
		}
	}

	// Check if ffmpeg is available
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %v", err)
	}

	fmt.Printf("Extracting frames from '%s' to '%s' at %d second intervals...\n", videoPath, outputDir, interval)

	ffmpegCommand := exec.Command(
		"ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=1/%d", interval),
		fmt.Sprintf("%s/frame_%%04d.jpg", outputDir),
	)

	// Capture both stdout and stderr
	output, err := ffmpegCommand.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %v\nCommand output:\n%s", err, string(output))
	}

	return nil
}

func analyzeImage(ctx context.Context, a *agent.DefaultAgent, imagePath string) (string, error) {
	// Create vision prompt with image data
	response := a.Run(
		ctx,
		agent.WithInput("Describe this image in detail."),
		agent.WithImagePath(imagePath),
	)
	if response.Err != nil {
		return "", response.Err
	}

	return response.Messages[0].Content, nil
}

func processVideo(ctx context.Context, a *agent.DefaultAgent, videoPath, outputDir string) error {
	err := extractFrames(videoPath, outputDir, 5)
	if err != nil {
		return err
	}

	files, err := os.ReadDir(outputDir)
	if err != nil {
		return err
	}

	var frames []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".jpg") {
			frames = append(frames, file.Name())
		}
	}
	sort.Strings(frames)

	for _, frame := range frames {
		framePath := filepath.Join(outputDir, frame)
		fmt.Printf("Analyzing frame: %s\n", frame)
		analysis, err := analyzeImage(ctx, a, framePath)
		if err != nil {
			return err
		}
		fmt.Printf("Analysis: %s\n\n", analysis)
	}

	return nil
}

func main() {
	ctx := context.Background()

	// Parse command line arguments
	videoPath := "path/to/your/video.mp4"
	outputDir := "output_frames"
	
	// Simple command line argument parsing
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--video" && i+1 < len(args) {
			videoPath = args[i+1]
			i++
		} else if args[i] == "--output" && i+1 < len(args) {
			outputDir = args[i+1]
			i++
		}
	}

	// Check if videoPath is still the default value
	if videoPath == "path/to/your/video.mp4" {
		log.Printf("Error: Please provide a valid video path using --video")
		fmt.Println("Usage: go run main.go --video path/to/video.mp4 [--output output_directory]")
		os.Exit(1)
	}

	// Configure logger
	logger := slog.New(
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}),
	)

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

	fmt.Printf("Processing video: %s\n", videoPath)
	fmt.Printf("Output directory: %s\n", outputDir)

	err := processVideo(ctx, visionAgent, videoPath, outputDir)
	if err != nil {
		log.Printf("Error processing video: %v", err)
		os.Exit(1)
	}
}
