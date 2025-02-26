package main

import (
	"context"
	"encoding/base64"
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
	// Create output directory if it doesn't exist
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create output directory '%s': %v", outputDir, err)
		}
	}

	// Check if video file exists
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("video file does not exist at path: '%s'", videoPath)
	} else if err != nil {
		return fmt.Errorf("error checking video file: %v", err)
	}

	// Create a folder name based on the video file name
	videoBaseName := filepath.Base(videoPath)
	videoName := strings.TrimSuffix(videoBaseName, filepath.Ext(videoBaseName))
	frameDirPath := filepath.Join(outputDir, videoName)
	
	// Check if frames already exist
	if files, err := os.ReadDir(frameDirPath); err == nil && len(files) > 0 {
		// Count the number of jpg files
		frameCount := 0
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(strings.ToLower(file.Name()), ".jpg") {
				frameCount++
			}
		}
		
		if frameCount > 0 {
			fmt.Printf("Frames already exist in %s. Skipping extraction. Found %d frames.\n", frameDirPath, frameCount)
			return nil
		}
	}
	
	// Create the frame directory
	if err := os.MkdirAll(frameDirPath, 0755); err != nil {
		return fmt.Errorf("failed to create frame directory '%s': %v", frameDirPath, err)
	}

	fmt.Printf("Extracting frames from '%s' to '%s' at %d second intervals...\n", videoPath, frameDirPath, interval)
	
	// Check if ffmpeg is installed
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH. Please install ffmpeg to extract video frames: %v", err)
	}

	// Extract frames using ffmpeg
	ffmpegCommand := exec.Command(
		"ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=1/%d", interval),
		fmt.Sprintf("%s/frame_%%04d.jpg", frameDirPath),
	)

	output, err := ffmpegCommand.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed to extract frames:\nCommand: %v\nError: %v\nOutput: %s", 
			ffmpegCommand.Args, err, string(output))
	}

	fmt.Printf("Successfully extracted frames to %s\n", frameDirPath)
	return nil
}

func analyzeImage(ctx context.Context, a *agent.DefaultAgent, imagePath string) (string, error) {
	// Check if image exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return "", fmt.Errorf("image file does not exist at path: '%s'", imagePath)
	} else if err != nil {
		return "", fmt.Errorf("error checking image file: %v", err)
	}

	// Read image data
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file '%s': %v", imagePath, err)
	}

	if len(imageData) == 0 {
		return "", fmt.Errorf("image file '%s' is empty", imagePath)
	}

	// Base64 encode the image data
	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// Create vision prompt with base64 encoded image data
	prompt := fmt.Sprintf(`[
		{"type": "text", "text": "Describe this image in detail."},
		{"type": "image", "source": {"data": "data:image/jpeg;base64,%s", "media_type": "image/jpeg"}}
	]`, base64Image)

	// Call the LLM
	response, err := a.Run(ctx, prompt, agent.DefaultStopCondition)
	if err != nil {
		return "", fmt.Errorf("LLM failed to analyze image '%s': %v", imagePath, err)
	}

	if len(response) == 0 || response[0].Message == nil {
		return "", fmt.Errorf("LLM returned empty response for image '%s'", imagePath)
	}

	return response[0].Message.Content, nil
}

func processVideo(ctx context.Context, a *agent.DefaultAgent, videoPath, outputDir string) error {
	fmt.Printf("Processing video: '%s'\n", videoPath)
	
	err := extractFrames(videoPath, outputDir, 5)
	if err != nil {
		return fmt.Errorf("frame extraction failed: %v", err)
	}

	// Create a folder name based on the video file name
	videoBaseName := filepath.Base(videoPath)
	videoName := strings.TrimSuffix(videoBaseName, filepath.Ext(videoBaseName))
	frameDirPath := filepath.Join(outputDir, videoName)

	files, err := os.ReadDir(frameDirPath)
	if err != nil {
		return fmt.Errorf("failed to read frames directory '%s': %v", frameDirPath, err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no frames found in directory '%s'", frameDirPath)
	}

	var frames []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(strings.ToLower(file.Name()), ".jpg") {
			frames = append(frames, file.Name())
		}
	}
	
	if len(frames) == 0 {
		return fmt.Errorf("no JPEG frames found in directory '%s'", frameDirPath)
	}
	
	fmt.Printf("Found %d frames to analyze\n", len(frames))
	sort.Strings(frames)

	for i, frame := range frames {
		framePath := filepath.Join(frameDirPath, frame)
		fmt.Printf("Analyzing frame %d/%d: %s\n", i+1, len(frames), frame)
		
		analysis, err := analyzeImage(ctx, a, framePath)
		if err != nil {
			return fmt.Errorf("failed to analyze frame '%s': %v", framePath, err)
		}
		
		fmt.Printf("Analysis for frame %d/%d:\n%s\n\n", i+1, len(frames), analysis)
	}

	fmt.Printf("Successfully processed all %d frames from video '%s'\n", len(frames), videoPath)
	return nil
}

func main() {
	// Configure logger
	logger := slog.New(
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}),
	)

	// Check if Ollama is running
	_, err := exec.Command("curl", "-s", "http://localhost:11434/api/tags").Output()
	if err != nil {
		log.Printf("Error: Ollama server is not running. Please start it with 'ollama serve'")
		os.Exit(1)
	}

	fmt.Println("Setting up Ollama provider...")
	
	// Set up Ollama provider
	opts := &ollama.ProviderOpts{
		Logger:  logger,
		BaseURL: "http://localhost",
		Port:    11434,
	}
	provider := ollama.NewProvider(opts)

	// Check if the model exists
	modelName := "llama3.2-vision:11b"
	_, err = exec.Command("ollama", "list").Output()
	if err != nil {
		log.Printf("Warning: Could not check if model '%s' exists. You may need to pull it with 'ollama pull %s'", 
			modelName, modelName)
	}

	// Use the correct model
	fmt.Printf("Using vision model: %s\n", modelName)
	model := &types.Model{
		ID: modelName,
	}
	provider.UseModel(ctx, model)

	// Create agent configuration
	fmt.Println("Creating visual analysis agent...")
	agentConf := &agent.NewAgentConfig{
		Provider:     provider,
		Logger:       logger,
		SystemPrompt: "You are a visual analysis assistant specialized in detailed image descriptions.",
	}

	// Initialize agent
	visionAgent := agent.NewAgent(agentConf)

	// Get video path from command line or use default
	videoPath := "input.mp4"
	if len(os.Args) > 1 {
		videoPath = os.Args[1]
	}
	
	outputDir := "output_frames"
	if len(os.Args) > 2 {
		outputDir = os.Args[2]
	}

	// Process video
	fmt.Printf("Starting video analysis process...\n")
	ctx := context.Background()
	err = processVideo(ctx, visionAgent, videoPath, outputDir)
	if err != nil {
		log.Printf("Error processing video: %v", err)
		os.Exit(1)
	}

	fmt.Println("Video processing completed successfully!")
}
