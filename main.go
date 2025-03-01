package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agent-api/core/pkg/agent"
	"github.com/agent-api/core/types"
	"github.com/agent-api/ollama"

	"log/slog"

	"github.com/lmittmann/tint"
)

const (
	maxWorkers = 4  // Adjust based on your CPU cores
	batchSize  = 10 // Number of results to batch write
)

var (
	outputDir  string
	videoName  string
)

type WorkItem struct {
	framePath string
	frameNum  int
	total     int
}

type AnalysisResult struct {
	Frame   string `json:"frame"`
	Content string `json:"content"`
}

type AnalysisResultBatch struct {
	results []AnalysisResult
	mu      sync.Mutex
}

func (batch *AnalysisResultBatch) addResult(result AnalysisResult) {
	batch.mu.Lock()
	defer batch.mu.Unlock()
	batch.results = append(batch.results, result)

	// Write to disk when batch is full
	if len(batch.results) >= batchSize {
		if err := batch.flush(outputDir, videoName); err != nil {
			log.Printf("Error flushing results: %v", err)
		}
	}
}

func (batch *AnalysisResultBatch) flush(outputDir, videoName string) error {
	if len(batch.results) == 0 {
		return nil
	}

	resultsFilePath := filepath.Join(outputDir, videoName, "analysis_results.json")

	var existingResults []AnalysisResult
	if data, err := os.ReadFile(resultsFilePath); err == nil {
		json.Unmarshal(data, &existingResults)
	}

	allResults := append(existingResults, batch.results...)

	file, err := os.Create(resultsFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(allResults); err != nil {
		return err
	}

	batch.results = nil // Clear the batch
	return nil
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

func extractFrames(videoPath, outputDir string, interval int) error {
	// Check if video file exists
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("video file does not exist at path: '%s'", videoPath)
	}

	// Create base output directory if it doesn't exist
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create output directory '%s': %v", outputDir, err)
		}
	}

	// Create a subfolder with the video's name
	videoName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	frameDirPath := filepath.Join(outputDir, videoName)

	// Check if frames already exist in the subfolder
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

	frameChan := make(chan string, 100) // Buffer size for frame paths

	// Start ffmpeg with pipe output
	cmd := exec.Command(
		"ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=1/%d", interval),
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Process frames in parallel
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		frameNum := 1

		buffer := make([]byte, 32*1024)
		for {
			n, err := stdout.Read(buffer)
			if n == 0 || err != nil {
				break
			}

			framePath := filepath.Join(frameDirPath, fmt.Sprintf("frame_%04d.jpg", frameNum))
			frameNum++

			// Save frame asynchronously
			go func(data []byte, path string) {
				if err := os.WriteFile(path, data, 0644); err != nil {
					log.Printf("Error saving frame: %v", err)
				}
				frameChan <- path
				}(buffer[:n], framePath)
			}
		}()

	wg.Wait()
	close(frameChan)

	return cmd.Wait()
}

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

func processVideo(ctx context.Context, a *agent.DefaultAgent, videoPath, outputDir string) error {
	fmt.Printf("Processing video: '%s'\n", videoPath)

	err := extractFrames(videoPath, outputDir, 5)
	if err != nil {
		return err
	}

	// Get the subfolder path that contains the frames
	videoName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
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

	workChan := make(chan WorkItem, len(frames))
	resultsChan := make(chan AnalysisResult, len(frames))
	errorsChan := make(chan error, len(frames))

	var wg sync.WaitGroup
	batch := &AnalysisResultBatch{}

	remainingFrames := atomic.Int64{}
	remainingFrames.Store(int64(len(frames)))

	// Start worker pool
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workChan {
				framePath := filepath.Join(frameDirPath, work.framePath)
				analysis, err := analyzeImage(ctx, a, framePath, outputDir, videoName)
				if err != nil {
					errorsChan <- fmt.Errorf("frame %d/%d failed: %v", work.frameNum, work.total, err)
					continue
				}

				resultsChan <- AnalysisResult{
					Frame:   work.framePath,
					Content: analysis,
					}
				
				remaining := remainingFrames.Add(-1)
				fmt.Printf("\rRemaining frames to analyze: %d/%d", remaining, len(frames))
			}
		}()
	}

	// Send work to workers
	go func() {
		for i, frame := range frames {
			workChan <- WorkItem{
				framePath: frame,
				frameNum:  i + 1,
				total:     len(frames),
			}
		}
		close(workChan)
	}()

	// Collect results
	go func() {
		for result := range resultsChan {
			batch.addResult(result)
		}
	}()

	// Wait for all workers to finish
	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Flush any remaining results
	if err := batch.flush(outputDir, videoName); err != nil {
		return fmt.Errorf("failed to flush final results: %v", err)
	}

	// Check for any errors
	if len(errorsChan) > 0 {
		return fmt.Errorf("encountered errors during processing: %v", <-errorsChan)
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
		SystemPrompt: "You are a visual analysis assistant specialized in detailed image descriptions. If there is a person in the image describe what they are doing in step by step format.",
	}

	// Initialize agent
	visionAgent := agent.NewAgent(agentConf)

	// Parse command line arguments
	videoPath := "path/to/your/video.mp4"
	outputDir = "output_frames"  // default value

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
		fmt.Println("Usage: go run main.go --video path/to/video.mp4 [--output output_directory]")
		os.Exit(1)
	}

	// After parsing the video path, set the videoName
	videoName = strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	// Process video
	fmt.Printf("Starting video analysis...\n")
	err := processVideo(ctx, visionAgent, videoPath, outputDir)
	if err != nil {
		log.Printf("Error processing video: %v", err)
		os.Exit(1)
	}

	fmt.Println("Video processing completed successfully!")
}
