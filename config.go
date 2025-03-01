package main

const (
	maxWorkers = 4  // Adjust based on your CPU cores
	batchSize  = 10 // Number of results to batch write
)

var (
	outputDir string
	videoName string
)
