package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agent-api/core/pkg/agent"
	"github.com/agent-api/ollama"
	"github.com/lmittmann/tint"
	"golang.org/x/exp/slog"
)

func extractFrames(videoPath, outputDir string, interval int) error {
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			return err
		}
	}

	ffmpegCommand := exec.Command(
		"ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=1/%d", interval),
		fmt.Sprintf("%s/frame_%%04d.jpg", outputDir),
	)

	return ffmpegCommand.Run()
}

func analyzeImage(ctx context.Context, agent *agent.Agent, imagePath string) (string, error) {
	imageData, err := ioutil.ReadFile(imagePath)
	if err != nil {
		return "", err
	}

	// Create vision prompt with image data
	prompt := fmt.Sprintf(`[
		{"type": "text", "text": "Describe this image in detail."},
		{"type": "image", "source": {"data": "%s", "media_type": "image/jpeg"}}
	]`, imageData)

	response, err := agent.Run(ctx, prompt, agent.DefaultStopCondition)
	if err != nil {
		return "", err
	}

	return response[0].Message.Content, nil
}

func processVideo(ctx context.Context, agent *agent.Agent, videoPath, outputDir string) error {
	err := extractFrames(videoPath, outputDir, 5)
	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir(outputDir)
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
		analysis, err := analyzeImage(ctx, agent, framePath)
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
	provider.UseModel(ctx, "llama3.2-11b-vision")

	// Create agent configuration
	agentConf := &agent.NewAgentConfig{
		Provider:     provider,
		Logger:       logger,
		SystemPrompt: "You are a visual analysis assistant specialized in detailed image descriptions",
	}

	// Initialize agent
	visionAgent := agent.NewAgent(agentConf)

	// Process video
	videoPath := "path/to/your/video.mp4"
	outputDir := "output_frames"

	err := processVideo(ctx, visionAgent, videoPath, outputDir)
	if err != nil {
		log.Fatal(err)
	}
}
