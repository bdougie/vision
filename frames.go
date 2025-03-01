package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

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
