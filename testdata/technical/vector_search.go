package search

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/pgvector/pgvector-go"
)

// VectorSearchEngine implements semantic search using embeddings
type VectorSearchEngine struct {
	db             *sql.DB
	embeddingDim   int
	similarityFunc string
	cache          *sync.Map
}

// SearchResult represents a vector search result with relevance scoring
type SearchResult struct {
	DocumentID string  `json:"document_id"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// NewVectorSearchEngine creates a new vector search engine instance
func NewVectorSearchEngine(db *sql.DB, dim int) *VectorSearchEngine {
	return &VectorSearchEngine{
		db:             db,
		embeddingDim:   dim,
		similarityFunc: "cosine",
		cache:          &sync.Map{},
	}
}

// Search performs semantic similarity search using vector embeddings
func (v *VectorSearchEngine) Search(ctx context.Context, queryEmbedding []float32, limit int) ([]SearchResult, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%v:%d", queryEmbedding[:5], limit)
	if cached, ok := v.cache.Load(cacheKey); ok {
		return cached.([]SearchResult), nil
	}

	// Build the similarity search query
	query := `
		SELECT 
			d.id,
			d.content,
			d.metadata,
			1 - (e.embedding <=> $1::vector) as similarity_score
		FROM documents d
		JOIN embeddings e ON d.id = e.document_id
		WHERE e.embedding <=> $1::vector < 0.5  -- Similarity threshold
		ORDER BY e.embedding <=> $1::vector
		LIMIT $2
	`

	rows, err := v.db.QueryContext(ctx, query, pgvector.NewVector(queryEmbedding), limit)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var metadata []byte
		
		err := rows.Scan(&result.DocumentID, &result.Content, &metadata, &result.Score)
		if err != nil {
			continue
		}

		// Parse metadata JSON
		if err := json.Unmarshal(metadata, &result.Metadata); err != nil {
			result.Metadata = make(map[string]interface{})
		}

		results = append(results, result)
	}

	// Apply re-ranking using cross-encoder if available
	results = v.rerank(results, string(queryEmbedding))

	// Cache the results
	v.cache.Store(cacheKey, results)

	return results, nil
}

// rerank applies sophisticated re-ranking to improve result relevance
func (v *VectorSearchEngine) rerank(results []SearchResult, query string) []SearchResult {
	// Calculate additional features for ranking
	for i := range results {
		// Term frequency scoring
		termScore := calculateTermFrequency(query, results[i].Content)
		
		// Recency boost (if metadata contains timestamp)
		recencyScore := 1.0
		if ts, ok := results[i].Metadata["timestamp"].(time.Time); ok {
			age := time.Since(ts).Hours() / 24 // Days old
			recencyScore = math.Exp(-age / 30) // Exponential decay over 30 days
		}
		
		// Combine scores with weights
		results[i].Score = 0.7*results[i].Score + 0.2*termScore + 0.1*recencyScore
	}

	// Sort by final score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// calculateTermFrequency computes TF-IDF style scoring
func calculateTermFrequency(query, content string) float64 {
	// Simplified TF calculation
	queryTerms := strings.Fields(strings.ToLower(query))
	contentLower := strings.ToLower(content)
	
	score := 0.0
	for _, term := range queryTerms {
		count := strings.Count(contentLower, term)
		if count > 0 {
			// Log normalization to prevent domination by high frequency
			score += math.Log(1 + float64(count))
		}
	}
	
	// Normalize by document length
	docLength := len(strings.Fields(content))
	if docLength > 0 {
		score = score / math.Log(float64(docLength))
	}
	
	return score
}

// IndexDocument adds a new document with its embedding to the search index
func (v *VectorSearchEngine) IndexDocument(ctx context.Context, docID string, content string, embedding []float32) error {
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert document
	_, err = tx.ExecContext(ctx,
		"INSERT INTO documents (id, content, indexed_at) VALUES ($1, $2, $3)",
		docID, content, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	// Insert embedding
	_, err = tx.ExecContext(ctx,
		"INSERT INTO embeddings (document_id, embedding) VALUES ($1, $2)",
		docID, pgvector.NewVector(embedding),
	)
	if err != nil {
		return fmt.Errorf("failed to insert embedding: %w", err)
	}

	return tx.Commit()
}