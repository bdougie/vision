package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Function to extract frames from video using FFmpeg
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

// Function to analyze an image using Llama 3.2
func analyzeImage(imagePath string) (string, error) {
	imageData, err := ioutil.ReadFile(imagePath)
	if err != nil {
		return "", err
	}

	client, err := api.ClientFromEnvironment()
	if err != nil {
		return "", err
	}

	response, err := client.Chat(api.ChatRequest{
		Model: "llama3.2-11b-vision",
		Messages: []api.Message{
			{
				Role:    "user",
				Content: "Describe this image in detail.",
				Images:  []string{string(imageData)},
			},
		},
	})

	if err != nil {
		return "", err
	}

	return response.Message.Content, nil
}

// Main function to process video and analyze frames
func processVideo(videoPath, outputDir string) error {
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
		analysis, err := analyzeImage(framePath)
		if err != nil {
			return err
		}
		fmt.Printf("Analysis: %s\n\n", analysis)
	}

	return nil
}

func main() {
	videoPath := "path/to/your/video.mp4"
	outputDir := "output_frames"

	err := processVideo(videoPath, outputDir)
	if err != nil {
		log.Fatal(err)
	}
}
