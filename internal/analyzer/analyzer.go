package analyzer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/agent-api/core/agent"
	"github.com/bdougie/vision/internal/extractor"
	"github.com/bdougie/vision/internal/models"
	"github.com/bdougie/vision/internal/storage"
)

const maxWorkers = 4 // Adjust based on your CPU cores

type Processor struct {
	agent   *agent.Agent
	storage *storage.Storage
}

func NewProcessor(agent *agent.Agent, storage *storage.Storage) *Processor {
	return &Processor{
		agent:   agent,
		storage: storage,
	}
}

// ProcessVideo processes a video by extracting frames and analyzing them
func (p *Processor) ProcessVideo(ctx context.Context, videoPath, outputDir string) error {
	fmt.Printf("Processing video: '%s'\n", videoPath)

	// Extract frames
	videoName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	frameDirPath := filepath.Join(outputDir, videoName)

	err := extractor.ExtractFrames(videoPath, outputDir, 15)
	if err != nil {
		return err
	}

	// Get frames
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

	// Process frames
	return p.processFrames(ctx, frames, frameDirPath)
}

func (p *Processor) processFrames(ctx context.Context, frames []string, frameDirPath string) error {
	workChan := make(chan models.WorkItem, len(frames))
	resultsChan := make(chan models.AnalysisResult, len(frames))
	errorsChan := make(chan error, len(frames))

	var wg sync.WaitGroup

	remainingFrames := atomic.Int64{}
	remainingFrames.Store(int64(len(frames)))

	// Start worker pool
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workChan {
				framePath := filepath.Join(frameDirPath, work.FramePath)
				analysis, err := p.analyzeImage(ctx, framePath)
				if err != nil {
					errorsChan <- fmt.Errorf("frame %d/%d failed: %v", work.FrameNum, work.Total, err)
					continue
				}

				resultsChan <- models.AnalysisResult{
					Frame:   work.FramePath,
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
			workChan <- models.WorkItem{
				FramePath: frame,
				FrameNum:  i + 1,
				Total:     len(frames),
			}
		}
		close(workChan)
	}()

	// Collect results
	go func() {
		for result := range resultsChan {
			p.storage.AddResult(result)
		}
	}()

	// Wait for all workers to finish
	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Flush any remaining results
	if err := p.storage.Flush(); err != nil {
		return fmt.Errorf("failed to flush final results: %v", err)
	}

	// Check for any errors
	var errorMessages []string
	for err := range errorsChan {
		errorMessages = append(errorMessages, err.Error())
	}
	if len(errorMessages) > 0 {
		return fmt.Errorf("encountered errors during processing: %v", strings.Join(errorMessages, "; "))
	}

	return nil
}

func (p *Processor) analyzeImage(ctx context.Context, imagePath string) (string, error) {
	// Create vision prompt with image data
	response, err := p.agent.Run(
		ctx,
		agent.WithInput("What is happening in this image? Be specific and detailed. List item and describe items shown in the video."),
		agent.WithImagePath(imagePath),
	)
	if err != nil {
		return "", err
	}

	// Extract the actual response content
	if len(response.Messages) == 0 {
		return "", fmt.Errorf("no response messages received from model")
	}

	// Get the model's response (not the prompt)
	content := response.Messages[len(response.Messages)-1].Content

	// Debug log to see what we're getting
	fmt.Printf("Raw response content: %s\n", content)

	return content, nil
}
