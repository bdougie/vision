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

### Prerequisites
- Go (1.21+ recommended)
- FFmpeg
- Ollama

If you don't have Go installed or are experiencing GOROOT issues, install/fix it first:
```sh
# macOS with Homebrew
brew install go

# Verify installation
go version

# If you see GOROOT errors, set it explicitly in your shell profile
echo 'export GOROOT=$(brew --prefix go)/libexec' >> ~/.zshrc  # or ~/.bash_profile
echo 'export PATH=$PATH:$GOROOT/bin:$HOME/go/bin' >> ~/.zshrc
source ~/.zshrc  # or source ~/.bash_profile
```

### Option 1: Local Installation
#### **MacOS (Homebrew)**
```sh
brew install ffmpeg
brew install ollama
go mod tidy
```

### Option 2: Run Directly (No Installation)
```sh
# Navigate to project directory
cd /path/to/vision

# Build and run in one step
go run ./cmd/visionanalyzer --video path/to/video.mp4 --output output_directory

# Or build an executable and run it
go build -o ./bin/visionanalyzer ./cmd/visionanalyzer
./bin/visionanalyzer --video path/to/video.mp4 --output output_directory
```

### Option 3: Docker Installation
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
# Build and run directly
go build -o visionanalyzer ./cmd/visionanalyzer
./visionanalyzer --video path/to/video.mp4

# Or run without building
go run ./cmd/visionanalyzer --video path/to/video.mp4

# Specify custom output directory
./visionanalyzer --video path/to/video.mp4 --output custom_output
```

# Show help
./visionanalyzer --help

## 🚀 Quick Start

```bash
# Process a video
./visionanalyzer --video path/to/video.mp4

# Specify custom output directory
./visionanalyzer --video path/to/video.mp4 --output custom_output

# Search for frames containing specific content (requires PostgreSQL)
export DB_ENABLED=true
./visionanalyzer --search "person cooking" --limit 10 --video path/to/video.mp4

# Show help
./visionanalyzer --help
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

## 🛢️ PostgreSQL with pgvector Setup

VisionFrameAnalyzer can store analysis results in PostgreSQL with pgvector for vector similarity search.

### Prerequisites

1. Install PostgreSQL (14+ recommended)

```bash
# macOS with Homebrew
brew install postgresql@14
brew services start postgresql@14

# Verify installation
psql --version
```

### Docker Compose Setup

For easy development, you can use Docker Compose to set up PostgreSQL with pgvector:

```yaml
version: '3.8'

services:
  db:
    image: postgres:14
    environment:
      POSTGRES_USER: user
      POSTGRES_PASSWORD: password
      POSTGRES_DB: visiondb
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

### PostgreSQL Schema

Create the necessary tables and enable pgvector extension:

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE videos (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE frames (
    id SERIAL PRIMARY KEY,
    video_id INTEGER REFERENCES videos(id),
    frame_number INTEGER NOT NULL,
    image_path TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE analyses (
    id SERIAL PRIMARY KEY,
    frame_id INTEGER REFERENCES frames(id),
    content JSONB,
    vector VECTOR(768), -- Adjust dimension based on your model
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Vector Similarity Search

The pgvector implementation allows you to search for frames with similar content using vector similarity, which is much more powerful than basic text search.

### Searching Frames

VisionFrameAnalyzer offers two ways to search for frames:

1. **Vector Similarity Search** - Find frames that are semantically similar to your query:

```bash
# Search for frames showing a person cooking (top 5 results)
export DB_ENABLED=true
./visionanalyzer --search "person cooking" --video path/to/video.mp4

# Increase the number of results
./visionanalyzer --search "person cooking" --limit 10 --video path/to/video.mp4
```

## 📁 Project Structure
```
vision/
├── cmd/
│   └── visionanalyzer/      # Main executable package
├── internal/
│   ├── analyzer/            # AI vision analysis functionality
│   ├── extractor/           # Video frame extraction functionality
│   ├── models/              # Shared data structures
│   └── storage/             # Result storage and persistence
```

## 📌 Use Cases

📽️ Automated Video Analysis – Extract insights from video feeds  
🔍 Content Moderation – Detect and describe images in video content  
🛠 Machine Learning Pipelines – Pre-process video datasets for AI models  

## 📜 License

MIT License. See LICENSE for details.
