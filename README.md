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
### **1️⃣ Install Dependencies**  
#### **MacOS (Homebrew)**
```sh
brew install ffmpeg
go mod tidy
```
### **2️⃣ Run the Analysis**
```sh
go run main.go --video path/to/video.mp4 --output output_frames
```

## 🛠 Usage Example
```
go run main.go --video input.mp4 --output frames
```

## 📂 Folder Structure
When you run the analysis, the following structure will be created:
```
output_frames/
└── video_name/
    ├── frame_0001.jpg
    ├── frame_0002.jpg
    ├── analysis_results.json
    └── ...
```

The `analysis_results.json` file contains the AI analysis for each frame in JSON format:
```json
[
  {
    "frame": "frame_0001.jpg",
    "content": "Description of the first frame..."
  },
  {
    "frame": "frame_0002.jpg",
    "content": "Description of the second frame..."
  }
]
```

## 🛠 Usage

```sh
# Basic usage
go run main.go --video path/to/video.mp4

# Specify custom output directory
go run main.go --video path/to/video.mp4 --output custom_output

# Show help
go run main.go --help
```

Available flags:
- `--video`: Path to the video file (required)
- `--output`: Directory to store extracted frames (default: "output_frames")

## 📌 Use Cases

📽️ Automated Video Analysis – Extract insights from video feeds  
🔍 Content Moderation – Detect and describe images in video content  
🛠 Machine Learning Pipelines – Pre-process video datasets for AI models  

## 📜 License

MIT License. See LICENSE for details.
