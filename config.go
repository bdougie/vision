package main

// Constants for program configuration
const (
	MaxWorkers = 4  // Adjust based on your CPU cores
	BatchSize  = 10 // Number of results to batch write
)

// Global variables for shared state
var (
	OutputDir  string // Directory where frames and analysis will be stored
	VideoName  string // Name of the video being processed (without extension)
)
