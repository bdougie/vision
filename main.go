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
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
				return fmt.Errorf("failed to create output directory: %v", err)
		}
}

if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("video file does not exist: %s", videoPath)
}

ffmpegCommand := exec.Command(
		"ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=1/%d", interval),
		fmt.Sprintf("%s/frame_%%04d.jpg", outputDir),
)

output, err := ffmpegCommand.CombinedOutput()
if err != nil {
		return fmt.Errorf("ffmpeg error: %v\nOutput: %s", err, string(output))
}

return nil
}

func analyzeImage(ctx context.Context, a *agent.DefaultAgent, imagePath string) (string, error) {
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
			return "", err
	}

	// Base64 encode the image data
	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// Create vision prompt with base64 encoded image data
	prompt := fmt.Sprintf(`[
			{"type": "text", "text": "Describe this image in detail."},
			{"type": "image", "source": {"data": "data:image/jpeg;base64,%s", "media_type": "image/jpeg"}}
	]`, base64Image)

	response, err := a.Run(ctx, prompt, agent.DefaultStopCondition)
	if err != nil {
			return "", err
	}

	return response[0].Message.Content, nil
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
		SystemPrompt: "You are a visual analysis assistant specialized in detailed image descriptions.",
	}

	// Initialize agent
	visionAgent := agent.NewAgent(agentConf)

	// Process video
	videoPath := "input.mp4"
	outputDir := "output_frames"

	err := processVideo(ctx, visionAgent, videoPath, outputDir)
	if err != nil {
		log.Printf("Error processing video: %v", err)
		os.Exit(1)
	}
}
