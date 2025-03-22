package embeddings

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Vector represents a vector embedding
type Vector struct {
	Data []float32
}

// Result represents the result of embedding generation
type Result struct {
	Content   string
	Embedding []float32
	Error     error
}

// Work represents a unit of embedding work
type Work struct {
	Content string
	Result  chan<- Result
}

// Service manages embedding generation and caching
type Service struct {
	numWorkers int
	workQueue  chan Work
	cache      sync.Map // Thread-safe map for caching embeddings
	wg         sync.WaitGroup
}

// NewService creates a new embedding service with the specified number of workers
func NewService(numWorkers int) *Service {
	if numWorkers <= 0 {
		numWorkers = 4 // Default to 4 workers if not specified
	}
	
	workQueue := make(chan Work, 100) // Buffer size for embedding requests
	
	service := &Service{
		numWorkers: numWorkers,
		workQueue:  workQueue,
	}
	
	// Start embedding workers
	service.startWorkers()
	
	return service
}

// startWorkers starts a pool of goroutines for generating embeddings
func (s *Service) startWorkers() {
	for i := 0; i < s.numWorkers; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for work := range s.workQueue {
				// Check cache first
				if cachedEmb, ok := s.cache.Load(work.Content); ok {
					if embedding, validCache := cachedEmb.([]float32); validCache {
						work.Result <- Result{
							Content:   work.Content,
							Embedding: embedding,
						}
						continue
					}
				}

				// Generate embedding
				embedding, err := s.generateEmbedding(context.Background(), work.Content)
				if err == nil {
					// Cache the successful result
					s.cache.Store(work.Content, embedding)
				}
				
				// Send result back
				work.Result <- Result{
					Content:   work.Content,
					Embedding: embedding,
					Error:     err,
				}
			}
		}()
	}
}

// GetEmbedding requests an embedding generation asynchronously
func (s *Service) GetEmbedding(content string) <-chan Result {
	resultChan := make(chan Result, 1)
	
	// Check if we're already at capacity
	select {
	case s.workQueue <- Work{
		Content: content,
		Result:  resultChan,
	}:
		// Work queued successfully
	default:
		// Queue is full, return an error immediately
		resultChan <- Result{
			Content: content,
			Error:   fmt.Errorf("embedding queue is full, try again later"),
		}
		close(resultChan)
	}
	
	return resultChan
}

// generateEmbedding creates a vector embedding for the content
func (s *Service) generateEmbedding(ctx context.Context, content string) ([]float32, error) {
	// This is a placeholder - in a real application, you would:
	// 1. Call an embedding API like OpenAI
	// 2. Process the content to create embeddings
	
	// Simulate a computation-heavy task
	time.Sleep(50 * time.Millisecond)
	
	// For now, we'll return a simple dummy embedding
	// In a real application, this would be a high-dimensional vector
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

// Close shuts down the embedding service and waits for all workers to finish
func (s *Service) Close() {
	if s.workQueue != nil {
		close(s.workQueue)
	}
	s.wg.Wait() // Wait for all workers to finish
}
