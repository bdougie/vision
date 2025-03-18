package extractor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExtractFrames extracts frames from a video file at specified intervals
func ExtractFrames(videoPath, outputDir string, interval int) error {
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

	// Extract frames using ffmpeg
	ffmpegCommand := exec.Command(
		"ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=1/%d", interval),
		fmt.Sprintf("%s/frame_%%04d.jpg", frameDirPath),
	)

	// Capture output for better error reporting
	output, err := ffmpegCommand.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %v\nOutput: %s", err, string(output))
	}

	fmt.Printf("Successfully extracted frames to %s\n", frameDirPath)
	return nil
}
