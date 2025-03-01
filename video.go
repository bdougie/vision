package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/agent-api/core/pkg/agent"
)

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
	for i := 0; i < MaxWorkers; i++ {
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
