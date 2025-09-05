package search

import (
	"math"
	"sort"
	"strings"
	"time"
)

// Ranker handles result ranking and re-ranking
type Ranker struct {
	config *RankerConfig
}

// RankerConfig holds ranker configuration
type RankerConfig struct {
	RecencyBoost      float64 `json:"recency_boost"`
	FileSizeBoost     float64 `json:"file_size_boost"`
	PathDepthPenalty  float64 `json:"path_depth_penalty"`
	DuplicatePenalty  float64 `json:"duplicate_penalty"`
	TitleMatchBoost   float64 `json:"title_match_boost"`
	ExactMatchBoost   float64 `json:"exact_match_boost"`
	ProximityBoost    float64 `json:"proximity_boost"`
	DiversityFactor   float64 `json:"diversity_factor"`
}

// DefaultRankerConfig returns default ranker configuration
func DefaultRankerConfig() *RankerConfig {
	return &RankerConfig{
		RecencyBoost:     0.1,
		FileSizeBoost:    0.05,
		PathDepthPenalty: 0.05,
		DuplicatePenalty: 0.3,
		TitleMatchBoost:  0.2,
		ExactMatchBoost:  0.3,
		ProximityBoost:   0.15,
		DiversityFactor:  0.1,
	}
}

// NewRanker creates a new result ranker
func NewRanker(config *RankerConfig) *Ranker {
	if config == nil {
		config = DefaultRankerConfig()
	}
	
	return &Ranker{
		config: config,
	}
}

// RankResults re-ranks search results with advanced scoring
func (r *Ranker) RankResults(results []SearchResult, query string, pq *ProcessedQuery) []SearchResult {
	if len(results) == 0 {
		return results
	}
	
	// Calculate additional scores for each result
	for i := range results {
		result := &results[i]
		
		// Calculate feature scores
		recencyScore := r.calculateRecencyScore(result)
		pathScore := r.calculatePathScore(result)
		titleScore := r.calculateTitleScore(result, query)
		exactMatchScore := r.calculateExactMatchScore(result, query)
		proximityScore := r.calculateProximityScore(result, pq.Terms)
		
		// Combine with existing scores
		result.Score = r.combineScores(
			result.Score,
			recencyScore,
			pathScore,
			titleScore,
			exactMatchScore,
			proximityScore,
		)
	}
	
	// Apply diversity boost to prevent too many results from same file
	results = r.applyDiversityBoost(results)
	
	// Sort by final score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	
	// Apply duplicate penalty for very similar content
	results = r.penalizeDuplicates(results)
	
	return results
}

// calculateRecencyScore calculates score based on file recency
func (r *Ranker) calculateRecencyScore(result *SearchResult) float64 {
	if result.Metadata == nil {
		return 0
	}
	
	// Get modification time from metadata
	if modTime, ok := result.Metadata["modified_at"].(time.Time); ok {
		daysSince := time.Since(modTime).Hours() / 24
		
		// Exponential decay based on age
		score := math.Exp(-daysSince / 365) // Decay over a year
		return score * r.config.RecencyBoost
	}
	
	return 0
}

// calculatePathScore calculates score based on file path depth
func (r *Ranker) calculatePathScore(result *SearchResult) float64 {
	// Penalize deeply nested files
	depth := strings.Count(result.FilePath, "/")
	
	// Normalize depth (assume max depth of 10)
	normalizedDepth := float64(depth) / 10.0
	if normalizedDepth > 1.0 {
		normalizedDepth = 1.0
	}
	
	// Invert so shallower paths get higher scores
	score := 1.0 - normalizedDepth
	return score * r.config.PathDepthPenalty
}

// calculateTitleScore calculates score based on title/filename match
func (r *Ranker) calculateTitleScore(result *SearchResult, query string) float64 {
	query = strings.ToLower(query)
	filename := strings.ToLower(result.Filename)
	
	// Check if query appears in filename
	if strings.Contains(filename, query) {
		return r.config.TitleMatchBoost
	}
	
	// Check if any query terms appear in filename
	terms := strings.Fields(query)
	matchCount := 0
	for _, term := range terms {
		if strings.Contains(filename, term) {
			matchCount++
		}
	}
	
	if matchCount > 0 {
		score := float64(matchCount) / float64(len(terms))
		return score * r.config.TitleMatchBoost
	}
	
	return 0
}

// calculateExactMatchScore calculates score for exact phrase matches
func (r *Ranker) calculateExactMatchScore(result *SearchResult, query string) float64 {
	content := strings.ToLower(result.Content)
	query = strings.ToLower(query)
	
	// Check for exact query match
	if strings.Contains(content, query) {
		return r.config.ExactMatchBoost
	}
	
	return 0
}

// calculateProximityScore calculates score based on term proximity
func (r *Ranker) calculateProximityScore(result *SearchResult, terms []string) float64 {
	if len(terms) < 2 {
		return 0
	}
	
	content := strings.ToLower(result.Content)
	words := strings.Fields(content)
	
	// Create position map for each term
	positions := make(map[string][]int)
	for i, word := range words {
		for _, term := range terms {
			if word == term {
				positions[term] = append(positions[term], i)
			}
		}
	}
	
	// Calculate minimum distance between different terms
	minDistance := len(words)
	foundPairs := 0
	
	for i := 0; i < len(terms)-1; i++ {
		for j := i + 1; j < len(terms); j++ {
			term1Positions := positions[terms[i]]
			term2Positions := positions[terms[j]]
			
			if len(term1Positions) > 0 && len(term2Positions) > 0 {
				foundPairs++
				for _, pos1 := range term1Positions {
					for _, pos2 := range term2Positions {
						distance := abs(pos1 - pos2)
						if distance < minDistance {
							minDistance = distance
						}
					}
				}
			}
		}
	}
	
	if foundPairs == 0 {
		return 0
	}
	
	// Normalize distance (closer terms get higher score)
	score := 1.0 / float64(minDistance+1)
	return score * r.config.ProximityBoost
}

// combineScores combines multiple score components
func (r *Ranker) combineScores(baseScore, recency, path, title, exact, proximity float64) float64 {
	// Apply boosts additively
	totalScore := baseScore + recency + path + title + exact + proximity
	
	// Ensure score stays in reasonable range
	if totalScore > 1.0 {
		totalScore = 1.0
	}
	
	return totalScore
}

// applyDiversityBoost promotes diversity in results
func (r *Ranker) applyDiversityBoost(results []SearchResult) []SearchResult {
	// Track files we've seen
	fileScores := make(map[int64]int)
	
	for i := range results {
		result := &results[i]
		
		// Count how many times we've seen this file
		count := fileScores[result.FileID]
		fileScores[result.FileID]++
		
		// Apply penalty for repeated files
		if count > 0 {
			penalty := float64(count) * r.config.DiversityFactor
			result.Score *= (1.0 - penalty)
			if result.Score < 0 {
				result.Score = 0
			}
		}
	}
	
	return results
}

// penalizeDuplicates reduces scores for very similar content
func (r *Ranker) penalizeDuplicates(results []SearchResult) []SearchResult {
	if len(results) < 2 {
		return results
	}
	
	for i := 1; i < len(results); i++ {
		current := &results[i]
		
		// Check similarity with previous results
		for j := 0; j < i; j++ {
			prev := &results[j]
			
			// Skip if different files
			if current.FileID != prev.FileID {
				continue
			}
			
			// Calculate content similarity
			similarity := r.calculateSimilarity(current.Content, prev.Content)
			
			// Apply penalty if too similar
			if similarity > 0.8 {
				penalty := similarity * r.config.DuplicatePenalty
				current.Score *= (1.0 - penalty)
			}
		}
	}
	
	return results
}

// calculateSimilarity calculates Jaccard similarity between two texts
func (r *Ranker) calculateSimilarity(text1, text2 string) float64 {
	// Tokenize
	words1 := strings.Fields(strings.ToLower(text1))
	words2 := strings.Fields(strings.ToLower(text2))
	
	// Create sets
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)
	
	for _, word := range words1 {
		set1[word] = true
	}
	for _, word := range words2 {
		set2[word] = true
	}
	
	// Calculate intersection and union
	intersection := 0
	for word := range set1 {
		if set2[word] {
			intersection++
		}
	}
	
	union := len(set1) + len(set2) - intersection
	
	if union == 0 {
		return 0
	}
	
	return float64(intersection) / float64(union)
}

// GroupResults groups results by file for better presentation
func (r *Ranker) GroupResults(results []SearchResult) map[int64][]SearchResult {
	grouped := make(map[int64][]SearchResult)
	
	for _, result := range results {
		grouped[result.FileID] = append(grouped[result.FileID], result)
	}
	
	// Sort chunks within each file by position
	for fileID := range grouped {
		sort.Slice(grouped[fileID], func(i, j int) bool {
			return grouped[fileID][i].CharStart < grouped[fileID][j].CharStart
		})
	}
	
	return grouped
}

// FilterByRelevance filters results by minimum relevance threshold
func (r *Ranker) FilterByRelevance(results []SearchResult, threshold float64) []SearchResult {
	var filtered []SearchResult
	
	for _, result := range results {
		if result.Score >= threshold {
			filtered = append(filtered, result)
		}
	}
	
	return filtered
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ReRankWithFeedback re-ranks results based on user feedback
func (r *Ranker) ReRankWithFeedback(results []SearchResult, clickedResults []int64, ignoredResults []int64) []SearchResult {
	// Create maps for quick lookup
	clicked := make(map[int64]bool)
	ignored := make(map[int64]bool)
	
	for _, id := range clickedResults {
		clicked[id] = true
	}
	for _, id := range ignoredResults {
		ignored[id] = true
	}
	
	// Adjust scores based on feedback
	for i := range results {
		result := &results[i]
		
		if clicked[result.ChunkID] {
			// Boost clicked results
			result.Score *= 1.5
		} else if ignored[result.ChunkID] {
			// Penalize ignored results
			result.Score *= 0.5
		}
		
		// Look for similar results to clicked ones
		for clickedID := range clicked {
			if r.isSimilarResult(result, clickedID, results) {
				result.Score *= 1.2
			}
		}
	}
	
	// Re-sort
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	
	return results
}

// isSimilarResult checks if a result is similar to a clicked result
func (r *Ranker) isSimilarResult(result *SearchResult, clickedID int64, allResults []SearchResult) bool {
	// Find the clicked result
	var clickedResult *SearchResult
	for _, res := range allResults {
		if res.ChunkID == clickedID {
			clickedResult = &res
			break
		}
	}
	
	if clickedResult == nil {
		return false
	}
	
	// Check if same file
	if result.FileID == clickedResult.FileID {
		return true
	}
	
	// Check if similar file type
	if result.FileType == clickedResult.FileType {
		return true
	}
	
	// Check content similarity
	similarity := r.calculateSimilarity(result.Content, clickedResult.Content)
	return similarity > 0.5
}