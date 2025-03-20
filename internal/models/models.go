package models

// WorkItem represents a frame to be processed
type WorkItem struct {
    FramePath string
    FrameNum  int
    Total     int
}

// AnalysisResult represents the result of analyzing a frame
type AnalysisResult struct {
    Frame   string `json:"frame"`
    Content string `json:"content"`
}

// FrameSearchResult represents a search result when looking for similar frames
type FrameSearchResult struct {
    FrameNumber int     `json:"frame_number"`
    FramePath   string  `json:"frame_path"`
    Description string  `json:"description"`
    Similarity  float64 `json:"similarity"`
}
