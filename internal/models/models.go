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
