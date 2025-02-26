package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
    prompt := fmt.Sprintf(`
    [
      {
        "type": "text",
        "text": "Please describe what you see in this image. Be specific and detailed about the main subjects, actions, and setting."
      },
      {
        "type": "image",
        "source": {
          "type": "base64",
          "media_type": "image/jpeg",
          "data": "%s"
        }
      }
    ]`, base64Image)

    fmt.Printf("Sending image to model for analysis...\n")
    
    // Call the LLM
    response, err := a.Run(ctx, prompt, agent.DefaultStopCondition)
    if err != nil {
        return "", fmt.Errorf("LLM failed to analyze image '%s': %v", imagePath, err)
    }

    if len(response) == 0 || response[0].Message == nil {
        return "", fmt.Errorf("LLM returned empty response for image '%s'", imagePath)
    }

    // Debug the structure of the response
    fmt.Printf("\n---RESPONSE STRUCTURE---\n")
    fmt.Printf("Response type: %T\n", response)
    fmt.Printf("Response length: %d\n", len(response))
    fmt.Printf("First response: %+v\n", response[0])
    if response[0].Message != nil {
        fmt.Printf("Message type: %T\n", response[0].Message)
        fmt.Printf("Message: %+v\n", response[0].Message)
        fmt.Printf("Content type: %T\n", response[0].Message.Content)
        fmt.Printf("Content length: %d\n", len(response[0].Message.Content))
    }
    
    content := response[0].Message.Content
    
    // Try to clean up the content if it contains JSON or base64
    cleanContent := content
    if strings.Contains(content, "base64") || strings.Contains(content, "data:") {
        cleanContent = "Contains base64 data - cleaning needed"
    }
    
    // Log both raw and cleaned content
    fmt.Printf("\n---AI RESPONSE START---\n%s\n---AI RESPONSE END---\n\n", content)
        
    return cleanContent, nil
}

// New type to store frame analysis results
type FrameAnalysis struct {
    FrameName   string `json:"frame_name"`
    Description string `json:"description"`
    Timestamp   string `json:"timestamp"`
}

// New type for the whole video analysis results
type VideoAnalysis struct {
    VideoName   string         `json:"video_name"`
    ProcessedAt string         `json:"processed_at"`
    Frames      []FrameAnalysis `json:"frames"`
}

// Save analysis results to JSON file
func saveAnalysisToJSON(outputPath string, analysis VideoAnalysis) error {
    // Create the JSON data
    jsonData, err := json.MarshalIndent(analysis, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal JSON data: %v", err)
    }
    
    // Write to file
    err = os.WriteFile(outputPath, jsonData, 0644)
    if err != nil {
        return fmt.Errorf("failed to write JSON file: %v", err)
    }
        
    fmt.Printf("Analysis results saved to: %s\n", outputPath)
    return nil
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

    // Initialize video analysis struct
    analysis := VideoAnalysis{
        VideoName:   videoName,
        ProcessedAt: time.Now().Format(time.RFC3339),
        Frames:      []FrameAnalysis{},
    }

    for i, frame := range frames {
        framePath := filepath.Join(frameDirPath, frame)
        fmt.Printf("\n===== Analyzing frame %d/%d: %s =====\n", i+1, len(frames), frame)
           
        description, err := analyzeImage(ctx, a, framePath)
        if err != nil {
            return fmt.Errorf("failed to analyze frame '%s': %v", framePath, err)
        }

        // Calculate approximate timestamp based on interval used in extractFrames
        frameNumber := 0
        fmt.Sscanf(frame, "frame_%04d.jpg", &frameNumber)
        timestamp := fmt.Sprintf("%02d:%02d", (frameNumber * 5) / 60, (frameNumber * 5) % 60)
               
        // Add to analysis results
        analysis.Frames = append(analysis.Frames, FrameAnalysis{
            FrameName:   frame,
            Description: description,
            Timestamp:   timestamp,
        })
    }

    // Save analysis results to JSON file
    jsonPath := filepath.Join(outputDir, videoName + "_analysis.json")
    if err := saveAnalysisToJSON(jsonPath, analysis); err != nil {
        return fmt.Errorf("failed to save analysis results: %v", err)
    }

    fmt.Printf("\nSuccessfully processed all %d frames from video '%s'\n", len(frames), videoPath)
    return nil
}

func main() {
    // Create context first
    ctx := context.Background()

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

    // Parse command line arguments
    videoPath := "input.mp4"
    outputDir := "output_frames"
    
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

    // Process video
    fmt.Printf("Starting video analysis process...\n")
    err = processVideo(ctx, visionAgent, videoPath, outputDir)
    if err != nil {
        log.Printf("Error processing video: %v", err)
        os.Exit(1)
    }

    fmt.Println("Video processing completed successfully!")
}
