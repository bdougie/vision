# VisionFrameAnalyzer 📹🖼️  
**Extract frames from videos and analyze them using AI-powered image recognition.**  

## 🚀 Overview  
VisionFrameAnalyzer is a Go-based tool that:  
✅ Extracts frames from a video at set intervals using `ffmpeg`  
✅ Uses an AI-powered vision model to analyze and describe each frame  
✅ Provides a structured pipeline for video-to-image processing  

## ✨ Features  
- 🎞 **Frame Extraction** – Converts video frames into images  
- 🖼 **AI-Powered Analysis** – Describes each frame using an LLM vision model  
- ⚡ **Multi-Frame Processing** – Handles multiple frames efficiently  
- 📝 **Detailed Logging** – Provides structured logs for debugging  

## 🛠 Tech Stack  
- **Go (Golang)**  
- **FFmpeg** (Frame Extraction)  
- **Ollama** (LLM-powered image analysis)  
- **Slog + Tint** (Logging)  
- **Kubernetes Ready** (Optional Multi-Cluster Support)  

## 📦 Installation & Setup

### Option 1: Local Installation
#### **MacOS (Homebrew)**
```sh
brew install ffmpeg
brew install ollama
go mod tidy
```

### Option 2: Docker Installation
```sh
# Build the container
docker build -t vision-analyzer .

# Run the container
docker run -v $(pwd):/data vision-analyzer --video /data/input.mp4 --output /data/frames
```

## 🔧 Configuration & Usage

### Ollama Setup
1. Ensure Ollama is running locally on port 11434
2. The tool uses `llama3.2-vision:11b` model by default

### Command Line Flags
- `--video`: Path to input video file (required)
- `--output`: Output directory for frames (default: "output_frames")

### Basic Usage
```sh
# Basic usage
go run *.go --video path/to/video.mp4

# Specify custom output directory
go run *.go --video path/to/video.mp4 --output custom_output

# Show help
go run *.go --help
```

## 📂 Output Structure
```
output_frames/
└── video_name/
    ├── frame_0001.jpg
    ├── frame_0002.jpg
    ├── analysis_results.json
    └── ...
```

### Analysis Results Format
The `analysis_results.json` file contains frame-by-frame analysis:
```json
[
  {
    "frame": "frame_0001.jpg",
    "content": "Detailed analysis of frame contents..."
  }
]
```

## 📌 Use Cases

📽️ Automated Video Analysis – Extract insights from video feeds  
🔍 Content Moderation – Detect and describe images in video content  
🛠 Machine Learning Pipelines – Pre-process video datasets for AI models  

## 📜 License

MIT License. See LICENSE for details.
