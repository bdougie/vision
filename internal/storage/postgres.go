package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bdougie/vision/internal/embeddings"
	"github.com/bdougie/vision/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool" // Import the PostgreSQL driver
	"github.com/pgvector/pgvector-go"
)

// PostgresConfig holds connection details for PostgreSQL
type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// AnalyzeResult represents the result of an analysis.
type AnalyzeResult struct {
	ID       int
	Text     string
	Vectors  []embeddings.Vector
	// Add other relevant fields here
}

// Vector represents a vector embedding.
type Vector struct {
	Data []float64
}

// EmbeddingResult represents the result of embedding generation
type EmbeddingResult struct {
	Content   string
	Embedding []float32
	Error     error
}

// EmbeddingWork represents a unit of embedding work
type EmbeddingWork struct {
	Content string
	Result  chan<- EmbeddingResult
}

// PostgresStorage manages interaction with PostgreSQL
type PostgresStorage struct {
	pool            *pgxpool.Pool
	videoID         int
	videoName       string
	embeddingService *embeddings.Service
	wg               sync.WaitGroup
}

// NewPostgresStorage creates a new PostgreSQL storage connection
func NewPostgresStorage(ctx context.Context, config PostgresConfig, videoName string) (*PostgresStorage, error) {
	// Build connection string
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.DBName,
	)

	// Connect to PostgreSQL
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create embedding service with workers
	embeddingWorkers := 4 // Number of concurrent embedding generators
	embeddingService := embeddings.NewService(embeddingWorkers)
	
	storage := &PostgresStorage{
		pool:            pool,
		videoName:       videoName,
		embeddingService: embeddingService,
	}

	// Get or create video ID
	videoID, err := storage.getOrCreateVideo(ctx, videoName)
	if err != nil {
		return nil, err
	}
	storage.videoID = videoID

	return storage, nil
}

// Close closes the database connection and worker goroutines
func (s *PostgresStorage) Close() {
	// Close embedding service
	if s.embeddingService != nil {
		s.embeddingService.Close()
	}
	
	// Wait for all remaining operations to finish
	s.wg.Wait()
	
	if s.pool != nil {
		s.pool.Close()
	}
}

// getOrCreateVideo gets an existing video entry or creates a new one
func (s *PostgresStorage) getOrCreateVideo(ctx context.Context, videoName string) (int, error) {
	// Check if video exists
	var id int
	err := s.pool.QueryRow(ctx,
		"SELECT id FROM videos WHERE name = $1",
		videoName).Scan(&id)

	if err == nil {
		// Video exists, return ID
		return id, nil
	} else if err != pgx.ErrNoRows {
		// Unexpected error
		return 0, fmt.Errorf("error checking for existing video: %w", err)
	}

	// Video doesn't exist, create it
	err = s.pool.QueryRow(ctx,
		"INSERT INTO videos (name, created_at) VALUES ($1, $2) RETURNING id",
		videoName, time.Now()).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to create video entry: %w", err)
	}

	return id, nil
}

// AddResult adds a frame analysis result to the database
func (s *PostgresStorage) AddResult(ctx context.Context, result models.AnalysisResult) error {
	// Extract frame number from filename
	frameName := result.Frame
	frameNum := 0
	if _, err := fmt.Sscanf(frameName, "frame_%04d.jpg", &frameNum); err != nil {
		return fmt.Errorf("invalid frame filename format: %s", frameName)
	}
	
	// Calculate timestamp from frame number (15 seconds per frame)
	timestamp := frameNum * 15
	
	// Check if this frame already exists with embeddings
	var frameID int
	var hasEmbedding bool
	err := s.pool.QueryRow(ctx, `
		SELECT f.id, 
		EXISTS(SELECT 1 FROM analyses a WHERE a.frame_id = f.id AND a.embedding IS NOT NULL) as has_embedding
		FROM frames f
		WHERE f.video_id = $1 AND f.frame_number = $2
	`, s.videoID, frameNum).Scan(&frameID, &hasEmbedding)
	
	if err == nil {
		// Frame exists, check if it has embeddings
		if hasEmbedding {
			// Frame already has embeddings, skip processing
			fmt.Printf("Frame %d already has embeddings, skipping\n", frameNum)
			return nil
		}
		
		// Frame exists but doesn't have embeddings, we'll add them
		fmt.Printf("Frame %d exists but has no embeddings, adding them\n", frameNum)
	} else if err != pgx.ErrNoRows {
		// Unexpected error
		return fmt.Errorf("error checking for existing frame: %w", err)
	} else {
		// Frame doesn't exist, insert it
		err = s.pool.QueryRow(ctx,
			`INSERT INTO frames 
			(video_id, frame_number, frame_path, timestamp, created_at) 
			VALUES ($1, $2, $3, $4, $5) 
			RETURNING id`,
			s.videoID, frameNum, frameName, timestamp, time.Now()).Scan(&frameID)
		
		if err != nil {
			return fmt.Errorf("failed to store frame information: %w", err)
		}
	}
	
	// Request embedding generation asynchronously using the embedding service
	embeddingResultChan := s.embeddingService.GetEmbedding(result.Content)
	embeddingResult := <-embeddingResultChan
	
	var embedding []float32
	if embeddingResult.Error != nil {
		// Log error but continue without embedding
		fmt.Printf("Warning: Failed to generate embedding: %v\n", embeddingResult.Error)
		embedding = []float32{0.1, 0.2, 0.3, 0.4} // Default fallback
	} else {
		embedding = embeddingResult.Embedding
	}
	
	// Store the analysis result with embedding
	_, err = s.pool.Exec(ctx,
		`INSERT INTO analyses 
		(frame_id, content, embedding, created_at) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (frame_id) DO UPDATE
		SET content = $2, embedding = $3, created_at = $4`,
		frameID, result.Content, pgvector.NewVector(embedding), time.Now())
	
	if err != nil {
		return fmt.Errorf("failed to store analysis: %w", err)
	}
	
	return nil
}

// BatchAddResults adds multiple analysis results in parallel
func (s *PostgresStorage) BatchAddResults(ctx context.Context, results []models.AnalysisResult) error {
	// Create channels for parallel processing
	errChan := make(chan error, len(results))
	semaphore := make(chan struct{}, 8) // Limit concurrent DB operations
	var wg sync.WaitGroup
	
	// Process each result concurrently
	for _, result := range results {
		wg.Add(1)
		go func(r models.AnalysisResult) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			if err := s.AddResult(ctx, r); err != nil {
				errChan <- fmt.Errorf("failed to add result for frame %s: %w", r.Frame, err)
			}
		}(result)
	}
	
	// Wait for all operations to complete
	wg.Wait()
	close(errChan)
	
	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors during batch processing", len(errs))
	}
	
	return nil
}

// Flush implements the Storage interface - no-op for Postgres as we save immediately
func (s *PostgresStorage) Flush() error {
	return nil
}

// generateEmbedding creates a vector embedding for the content
func (s *PostgresStorage) generateEmbedding(ctx context.Context, content string) ([]float32, error) {
	// This is a placeholder - in a real application, you would:
	// 1. Call an embedding API like OpenAI
	// 2. Process the content to create embeddings
	
	// Simulate a computation-heavy task
	time.Sleep(50 * time.Millisecond)
	
	// For now, we'll return a simple dummy embedding
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

// SearchSimilarFrames finds frames with similar content
func (s *PostgresStorage) SearchSimilarFrames(ctx context.Context, query string, limit int) ([]models.FrameSearchResult, error) {
	// Generate embedding for query using the embedding service
	embeddingResultChan := s.embeddingService.GetEmbedding(query)
	embeddingResult := <-embeddingResultChan
	
	if embeddingResult.Error != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", embeddingResult.Error)
	}
	
	queryEmbedding := embeddingResult.Embedding

	// Search for similar frames
	rows, err := s.pool.Query(ctx,
		`SELECT f.frame_number, f.frame_path, a.content, 
		1 - (a.embedding <=> $1) AS similarity
		FROM analyses a
		JOIN frames f ON a.frame_id = f.id
		JOIN videos v ON f.video_id = v.id
		WHERE v.id = $2
		ORDER BY a.embedding <=> $1
		LIMIT $3`,
		pgvector.NewVector(queryEmbedding), s.videoID, limit)

	if err != nil {
		return nil, fmt.Errorf("failed to search similar frames: %w", err)
	}
	defer rows.Close()

	// Process results
	var results []models.FrameSearchResult
	for rows.Next() {
		var result models.FrameSearchResult
		if err := rows.Scan(&result.FrameNumber, &result.FramePath,
			&result.Description, &result.Similarity); err != nil {
			return nil, fmt.Errorf("failed to scan search results: %w", err)
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

// TextSearchFrames finds frames containing specific text without using embeddings
func (s *PostgresStorage) TextSearchFrames(ctx context.Context, query string, limit int) ([]models.FrameSearchResult, error) {
	// Check if the video exists in the database
	var count int
	err := s.pool.QueryRow(ctx, 
		"SELECT COUNT(*) FROM frames WHERE video_id = $1", 
		s.videoID).Scan(&count)
	
	if err != nil {
		return nil, fmt.Errorf("failed to check for video frames: %w", err)
	}
	
	if count == 0 {
		return nil, fmt.Errorf("no frames found for video '%s'. Run:\n\nexport DB_ENABLED=true\n./visionanalyzer --video %s\n\nto process and embed this video first", 
			s.videoName, s.videoName)
	}
	
	// Simple text search using ILIKE
	rows, err := s.pool.Query(ctx,
		`SELECT f.frame_number, f.frame_path, a.content, 
		0.5 AS similarity
		FROM analyses a
		JOIN frames f ON a.frame_id = f.id
		JOIN videos v ON f.video_id = v.id
		WHERE v.id = $1 AND a.content ILIKE $2
		ORDER BY f.frame_number
		LIMIT $3`,
		s.videoID, "%"+query+"%", limit)
	
	if err != nil {
		return nil, fmt.Errorf("failed to search frames: %w", err)
	}
	defer rows.Close()
	
	// Process results
	var results []models.FrameSearchResult
	for rows.Next() {
		var result models.FrameSearchResult
		if err := rows.Scan(&result.FrameNumber, &result.FramePath,
			&result.Description, &result.Similarity); err != nil {
			return nil, fmt.Errorf("failed to scan search results: %w", err)
		}
		results = append(results, result)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading search results: %w", err)
	}
	
	if len(results) == 0 {
		// If no results found with ILIKE, inform the user
		return nil, fmt.Errorf("no frames found containing '%s' for video '%s'", query, s.videoName)
	}
	
	return results, nil
}

// InitSchema creates the database schema if it doesn't exist
func InitSchema(ctx context.Context, config PostgresConfig) error {
	// Build connection string
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.DBName,
	)

	// Connect to PostgreSQL
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	// Check if vector extension exists
	var exists bool
	err = conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check for vector extension: %w", err)
	}

	// Create vector extension if it doesn't exist
	if !exists {
		_, err = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
		if err != nil {
			return fmt.Errorf("failed to create vector extension: %w", err)
		}
	}

	// Create tables
	_, err = conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS videos (
            id SERIAL PRIMARY KEY,
            name VARCHAR(255) NOT NULL,
            created_at TIMESTAMPTZ NOT NULL,
            UNIQUE(name)
        );
        
        CREATE TABLE IF NOT EXISTS frames (
            id SERIAL PRIMARY KEY,
            video_id INTEGER REFERENCES videos(id) ON DELETE CASCADE,
            frame_number INTEGER NOT NULL,
            frame_path VARCHAR(255) NOT NULL,
            timestamp INTEGER NOT NULL,
            created_at TIMESTAMPTZ NOT NULL,
            UNIQUE(video_id, frame_number)
        );
        
        CREATE TABLE IF NOT EXISTS analyses (
            id SERIAL PRIMARY KEY,
            frame_id INTEGER REFERENCES frames(id) ON DELETE CASCADE,
            content TEXT NOT NULL,
            embedding vector(4),
            created_at TIMESTAMPTZ NOT NULL
        );
    `)

	if err != nil {
		return fmt.Errorf("failed to create database schema: %w", err)
	}

	// Create indexes
	_, err = conn.Exec(ctx, `
        CREATE INDEX IF NOT EXISTS idx_frames_video_id ON frames(video_id);
        CREATE INDEX IF NOT EXISTS idx_analyses_frame_id ON analyses(frame_id);
        CREATE INDEX IF NOT EXISTS idx_embedding_vector ON analyses USING ivfflat (embedding vector_l2_ops) WITH (lists = 100);
    `)

	if err != nil {
		return fmt.Errorf("failed to create database indexes: %w", err)
	}

	// Add unique constraint for analyses.frame_id
	if err := UpdateSchema(ctx, config); err != nil {
		return fmt.Errorf("failed to update schema: %w", err)
	}

	return nil
}

// UpdateSchema adds UNIQUE constraint on frame_id if needed
func UpdateSchema(ctx context.Context, config PostgresConfig) error {
    // Build connection string
    connString := fmt.Sprintf(
        "postgres://%s:%s@%s:%s/%s",
        config.User,
        config.Password,
        config.Host,
        config.Port,
        config.DBName,
    )

    // Connect to PostgreSQL
    conn, err := pgx.Connect(ctx, connString)
    if err != nil {
        return fmt.Errorf("failed to connect to database: %w", err)
    }
    defer conn.Close(ctx)
    
    // Check if unique constraint exists
    var constraintExists bool
    err = conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 
            FROM pg_constraint 
            WHERE conname = 'analyses_frame_id_key'
        )
    `).Scan(&constraintExists)
    
    if err != nil {
        return fmt.Errorf("failed to check constraint existence: %w", err)
    }
    
    // Add unique constraint if it doesn't exist
    if !constraintExists {
        _, err = conn.Exec(ctx, `
            ALTER TABLE analyses 
            ADD CONSTRAINT analyses_frame_id_key 
            UNIQUE (frame_id)
        `)
        
        if err != nil {
            return fmt.Errorf("failed to add unique constraint: %w", err)
        }
    }
    
    return nil
}