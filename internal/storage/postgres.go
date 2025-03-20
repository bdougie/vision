package storage

import (
	"context"
	"fmt"
	"log"
	"time"

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
	Vectors  []Vector
	// Add other relevant fields here
}

// Vector represents a vector embedding.
type Vector struct {
	Data []float64
}

// PostgresStorage manages interaction with PostgreSQL
type PostgresStorage struct {
	pool      *pgxpool.Pool
	videoID   int
	videoName string
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

	// Create storage instance
	storage := &PostgresStorage{
		pool:      pool,
		videoName: videoName,
	}

	// Get or create video ID
	videoID, err := storage.getOrCreateVideo(ctx, videoName)
	if err != nil {
		return nil, err
	}
	storage.videoID = videoID

	return storage, nil
}

// Close closes the database connection
func (s *PostgresStorage) Close() {
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
    
    // First, store the frame information
    var frameID int
    err := s.pool.QueryRow(ctx,
        `INSERT INTO frames 
        (video_id, frame_number, frame_path, timestamp, created_at) 
        VALUES ($1, $2, $3, $4, $5) 
        RETURNING id`,
        s.videoID, frameNum, frameName, timestamp, time.Now()).Scan(&frameID)
    
    if err != nil {
        return fmt.Errorf("failed to store frame information: %w", err)
    }
    
    // Generate embedding for content
    embedding, err := s.generateEmbedding(ctx, result.Content)
    if err != nil {
        // Log error but continue without embedding
        fmt.Printf("Warning: Failed to generate embedding: %v\n", err)
    }
    
    // Store the analysis result with embedding
    _, err = s.pool.Exec(ctx,
        `INSERT INTO analyses 
        (frame_id, content, embedding, created_at) 
        VALUES ($1, $2, $3, $4)`,
        frameID, result.Content, pgvector.NewVector(embedding), time.Now())
    
    if err != nil {
        return fmt.Errorf("failed to store analysis: %w", err)
    }
    
    return nil
}

// Flush implements the Storage interface - no-op for Postgres as we save immediately
func (s *PostgresStorage) Flush() error {
    return nil
}

// generateEmbedding creates a vector embedding for the content
// This is a placeholder - you'll need to implement actual embedding generation
func (s *PostgresStorage) generateEmbedding(ctx context.Context, content string) ([]float32, error) {
	// This is a placeholder - in a real application, you would:
	// 1. Call an embedding API like OpenAI
	// 2. Process the content to create embeddings
	// For now, we'll return a simple dummy embedding
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

// SearchSimilarFrames finds frames with similar content
func (s *PostgresStorage) SearchSimilarFrames(ctx context.Context, query string, limit int) ([]models.FrameSearchResult, error) {
	// Generate embedding for query
	queryEmbedding, err := s.generateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

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

	return nil
}

func main() {
	// Example Usage (Replace with your actual configuration)
	config := PostgresConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "your_user",
		Password: "your_password",
		DBName:   "your_db",
	}

	ctx := context.Background()

	// Initialize schema
	err := InitSchema(ctx, config)
	if err != nil {
		log.Fatalf("Error initializing schema: %v", err)
	}

	// Create a new PostgresStorage instance
	storage, err := NewPostgresStorage(ctx, config, "example_video")
	if err != nil {
		log.Fatalf("Error creating PostgresStorage: %v", err)
	}
	defer storage.Close()

	// Create a sample analysis result
	result := models.AnalysisResult{
		Frame:   "frame_0001.jpg",
		Content: "This is a sample content for analysis.",
	}

	// Add the analysis result
	err = storage.AddResult(ctx, result)
	if err != nil {
		log.Fatalf("Error adding analysis result: %v", err)
	}

	fmt.Println("Analysis result added successfully!")

	// Search for similar frames
	query := "sample content"
	limit := 5
	results, err := storage.SearchSimilarFrames(ctx, query, limit)
	if err != nil {
		log.Fatalf("Error searching similar frames: %v", err)
	}

	fmt.Println("Search Results:")
	for _, res := range results {
		fmt.Printf("Frame Number: %d, Frame Path: %s, Description: %s, Similarity: %f\n",
			res.FrameNumber, res.FramePath, res.Description, res.Similarity)
	}
}