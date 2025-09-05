package service

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiterConfig holds configuration for rate limiting
type RateLimiterConfig struct {
	IndexingRate   int           `json:"indexing_rate"`    // files per minute
	EmbeddingRate  int           `json:"embedding_rate"`   // embeddings per minute
	SearchRate     int           `json:"search_rate"`      // searches per minute
	BurstSize      int           `json:"burst_size"`       // burst capacity
	
	// Adaptive rate limiting
	EnableAdaptive      bool    `json:"enable_adaptive"`
	CPUThreshold        float64 `json:"cpu_threshold"`
	MemoryThreshold     float64 `json:"memory_threshold"`
	ReductionFactor     float64 `json:"reduction_factor"`     // Factor to reduce rate when under pressure
	RecoveryFactor      float64 `json:"recovery_factor"`      // Factor to increase rate when recovering
	
	// Time-based rate limiting (e.g., slow down during business hours)
	EnableTimeBased     bool `json:"enable_time_based"`
	BusinessHourStart   int  `json:"business_hour_start"`   // 24-hour format
	BusinessHourEnd     int  `json:"business_hour_end"`     // 24-hour format
	BusinessHourFactor  float64 `json:"business_hour_factor"` // Rate reduction factor during business hours
}

// RateLimiter provides rate limiting for various operations
type RateLimiter struct {
	config *RateLimiterConfig
	
	// Individual rate limiters
	indexingLimiter  *rate.Limiter
	embeddingLimiter *rate.Limiter
	searchLimiter    *rate.Limiter
	
	// Statistics
	stats     RateLimiterStats
	statsLock sync.RWMutex
	
	// Adaptive rate limiting state
	adaptiveLock    sync.RWMutex
	currentFactor   float64
	lastAdjustment  time.Time
	
	// Resource monitor reference for adaptive limiting
	resourceMonitor *ResourceMonitor
}

// RateLimiterStats holds rate limiter statistics
type RateLimiterStats struct {
	IndexingRequests     int64   `json:"indexing_requests"`
	IndexingAllowed      int64   `json:"indexing_allowed"`
	IndexingBlocked      int64   `json:"indexing_blocked"`
	IndexingRate         float64 `json:"indexing_rate"`       // current requests per second
	
	EmbeddingRequests    int64   `json:"embedding_requests"`
	EmbeddingAllowed     int64   `json:"embedding_allowed"`
	EmbeddingBlocked     int64   `json:"embedding_blocked"`
	EmbeddingRate        float64 `json:"embedding_rate"`
	
	SearchRequests       int64   `json:"search_requests"`
	SearchAllowed        int64   `json:"search_allowed"`
	SearchBlocked        int64   `json:"search_blocked"`
	SearchRate           float64 `json:"search_rate"`
	
	AdaptiveFactor       float64 `json:"adaptive_factor"`
	TimeFactor           float64 `json:"time_factor"`
	LastAdjustment       time.Time `json:"last_adjustment"`
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	// Set default values
	if config.IndexingRate == 0 {
		config.IndexingRate = 60 // 60 files per minute
	}
	if config.EmbeddingRate == 0 {
		config.EmbeddingRate = 120 // 120 embeddings per minute
	}
	if config.SearchRate == 0 {
		config.SearchRate = 300 // 300 searches per minute
	}
	if config.BurstSize == 0 {
		config.BurstSize = 10
	}
	if config.ReductionFactor == 0 {
		config.ReductionFactor = 0.5 // Reduce to 50% when under pressure
	}
	if config.RecoveryFactor == 0 {
		config.RecoveryFactor = 1.1 // Increase by 10% when recovering
	}
	if config.BusinessHourFactor == 0 {
		config.BusinessHourFactor = 0.7 // Reduce to 70% during business hours
	}
	if config.BusinessHourStart == 0 {
		config.BusinessHourStart = 9 // 9 AM
	}
	if config.BusinessHourEnd == 0 {
		config.BusinessHourEnd = 17 // 5 PM
	}
	
	rl := &RateLimiter{
		config:        config,
		currentFactor: 1.0,
	}
	
	// Create rate limiters with initial rates
	rl.updateRateLimiters()
	
	return rl
}

// SetResourceMonitor sets the resource monitor for adaptive rate limiting
func (rl *RateLimiter) SetResourceMonitor(rm *ResourceMonitor) {
	rl.resourceMonitor = rm
}

// AllowIndexing checks if an indexing operation is allowed
func (rl *RateLimiter) AllowIndexing() bool {
	rl.statsLock.Lock()
	rl.stats.IndexingRequests++
	rl.statsLock.Unlock()
	
	allowed := rl.indexingLimiter.Allow()
	
	rl.statsLock.Lock()
	if allowed {
		rl.stats.IndexingAllowed++
	} else {
		rl.stats.IndexingBlocked++
	}
	rl.statsLock.Unlock()
	
	return allowed
}

// AllowEmbedding checks if an embedding operation is allowed
func (rl *RateLimiter) AllowEmbedding() bool {
	rl.statsLock.Lock()
	rl.stats.EmbeddingRequests++
	rl.statsLock.Unlock()
	
	allowed := rl.embeddingLimiter.Allow()
	
	rl.statsLock.Lock()
	if allowed {
		rl.stats.EmbeddingAllowed++
	} else {
		rl.stats.EmbeddingBlocked++
	}
	rl.statsLock.Unlock()
	
	return allowed
}

// AllowSearch checks if a search operation is allowed
func (rl *RateLimiter) AllowSearch() bool {
	rl.statsLock.Lock()
	rl.stats.SearchRequests++
	rl.statsLock.Unlock()
	
	allowed := rl.searchLimiter.Allow()
	
	rl.statsLock.Lock()
	if allowed {
		rl.stats.SearchAllowed++
	} else {
		rl.stats.SearchBlocked++
	}
	rl.statsLock.Unlock()
	
	return allowed
}

// WaitForIndexing waits until an indexing operation is allowed
func (rl *RateLimiter) WaitForIndexing() {
	rl.statsLock.Lock()
	rl.stats.IndexingRequests++
	rl.stats.IndexingAllowed++
	rl.statsLock.Unlock()
	
	rl.indexingLimiter.Wait(nil)
}

// WaitForEmbedding waits until an embedding operation is allowed
func (rl *RateLimiter) WaitForEmbedding() {
	rl.statsLock.Lock()
	rl.stats.EmbeddingRequests++
	rl.stats.EmbeddingAllowed++
	rl.statsLock.Unlock()
	
	rl.embeddingLimiter.Wait(nil)
}

// WaitForSearch waits until a search operation is allowed
func (rl *RateLimiter) WaitForSearch() {
	rl.statsLock.Lock()
	rl.stats.SearchRequests++
	rl.stats.SearchAllowed++
	rl.statsLock.Unlock()
	
	rl.searchLimiter.Wait(nil)
}

// UpdateRates updates the rate limits based on current conditions
func (rl *RateLimiter) UpdateRates() {
	rl.adaptiveLock.Lock()
	defer rl.adaptiveLock.Unlock()
	
	oldFactor := rl.currentFactor
	newFactor := rl.calculateAdaptiveFactor()
	
	// Apply time-based factor
	timeFactor := rl.calculateTimeFactor()
	newFactor *= timeFactor
	
	// Only update if there's a significant change
	if abs(newFactor-oldFactor) > 0.05 {
		rl.currentFactor = newFactor
		rl.lastAdjustment = time.Now()
		rl.updateRateLimiters()
		
		rl.statsLock.Lock()
		rl.stats.AdaptiveFactor = rl.currentFactor
		rl.stats.TimeFactor = timeFactor
		rl.stats.LastAdjustment = rl.lastAdjustment
		rl.statsLock.Unlock()
	}
}

// calculateAdaptiveFactor calculates the adaptive rate factor based on resource usage
func (rl *RateLimiter) calculateAdaptiveFactor() float64 {
	if !rl.config.EnableAdaptive || rl.resourceMonitor == nil {
		return 1.0
	}
	
	usage := rl.resourceMonitor.GetCurrentUsage()
	
	// Check if we're under resource pressure
	cpuPressure := usage.CPUPercent > rl.config.CPUThreshold
	memoryPressure := usage.MemoryPercent > rl.config.MemoryThreshold
	
	if cpuPressure || memoryPressure {
		// Reduce rate when under pressure
		newFactor := rl.currentFactor * rl.config.ReductionFactor
		if newFactor < 0.1 {
			newFactor = 0.1 // Don't go below 10% of original rate
		}
		return newFactor
	} else {
		// Gradually increase rate when not under pressure
		newFactor := rl.currentFactor * rl.config.RecoveryFactor
		if newFactor > 1.0 {
			newFactor = 1.0 // Don't exceed original rate
		}
		return newFactor
	}
}

// calculateTimeFactor calculates the time-based rate factor
func (rl *RateLimiter) calculateTimeFactor() float64 {
	if !rl.config.EnableTimeBased {
		return 1.0
	}
	
	now := time.Now()
	hour := now.Hour()
	
	// Check if we're in business hours
	if hour >= rl.config.BusinessHourStart && hour < rl.config.BusinessHourEnd {
		// During business hours, reduce the rate
		return rl.config.BusinessHourFactor
	}
	
	return 1.0
}

// updateRateLimiters updates the actual rate limiters with current factors
func (rl *RateLimiter) updateRateLimiters() {
	// Calculate effective rates
	indexingRate := rate.Limit(float64(rl.config.IndexingRate) * rl.currentFactor / 60.0) // per second
	embeddingRate := rate.Limit(float64(rl.config.EmbeddingRate) * rl.currentFactor / 60.0) // per second
	searchRate := rate.Limit(float64(rl.config.SearchRate) * rl.currentFactor / 60.0) // per second
	
	// Create new rate limiters
	rl.indexingLimiter = rate.NewLimiter(indexingRate, rl.config.BurstSize)
	rl.embeddingLimiter = rate.NewLimiter(embeddingRate, rl.config.BurstSize)
	rl.searchLimiter = rate.NewLimiter(searchRate, rl.config.BurstSize)
}

// GetStats returns current rate limiter statistics
func (rl *RateLimiter) GetStats() RateLimiterStats {
	rl.statsLock.RLock()
	defer rl.statsLock.RUnlock()
	
	// Calculate current rates (requests per second over last minute)
	stats := rl.stats
	
	// Calculate rates based on recent activity
	if rl.stats.IndexingRequests > 0 {
		stats.IndexingRate = float64(rl.stats.IndexingAllowed) / 60.0 // approximate
	}
	if rl.stats.EmbeddingRequests > 0 {
		stats.EmbeddingRate = float64(rl.stats.EmbeddingAllowed) / 60.0 // approximate
	}
	if rl.stats.SearchRequests > 0 {
		stats.SearchRate = float64(rl.stats.SearchAllowed) / 60.0 // approximate
	}
	
	return stats
}

// ResetStats resets the rate limiter statistics
func (rl *RateLimiter) ResetStats() {
	rl.statsLock.Lock()
	defer rl.statsLock.Unlock()
	
	rl.stats = RateLimiterStats{
		AdaptiveFactor: rl.currentFactor,
		LastAdjustment: rl.lastAdjustment,
	}
}

// GetCurrentRates returns the current effective rates
func (rl *RateLimiter) GetCurrentRates() map[string]float64 {
	rl.adaptiveLock.RLock()
	defer rl.adaptiveLock.RUnlock()
	
	return map[string]float64{
		"indexing_per_minute":  float64(rl.config.IndexingRate) * rl.currentFactor,
		"embedding_per_minute": float64(rl.config.EmbeddingRate) * rl.currentFactor,
		"search_per_minute":    float64(rl.config.SearchRate) * rl.currentFactor,
		"adaptive_factor":      rl.currentFactor,
	}
}

// SetRates allows manual adjustment of rate limits
func (rl *RateLimiter) SetRates(indexingRate, embeddingRate, searchRate int) {
	rl.adaptiveLock.Lock()
	defer rl.adaptiveLock.Unlock()
	
	rl.config.IndexingRate = indexingRate
	rl.config.EmbeddingRate = embeddingRate
	rl.config.SearchRate = searchRate
	
	rl.updateRateLimiters()
}

// Pause temporarily pauses all rate limiting (sets rates to 0)
func (rl *RateLimiter) Pause() {
	rl.adaptiveLock.Lock()
	defer rl.adaptiveLock.Unlock()
	
	rl.indexingLimiter = rate.NewLimiter(0, 0)
	rl.embeddingLimiter = rate.NewLimiter(0, 0)
	// Don't pause search limiter - searches should always be allowed
}

// Resume resumes rate limiting with current configuration
func (rl *RateLimiter) Resume() {
	rl.adaptiveLock.Lock()
	defer rl.adaptiveLock.Unlock()
	
	rl.updateRateLimiters()
}

// IsHealthy checks if the rate limiter is operating within normal parameters
func (rl *RateLimiter) IsHealthy() bool {
	stats := rl.GetStats()
	
	// Check if blocking rate is too high (more than 50% blocked)
	indexingBlockRate := float64(stats.IndexingBlocked) / float64(stats.IndexingRequests)
	embeddingBlockRate := float64(stats.EmbeddingBlocked) / float64(stats.EmbeddingRequests)
	
	if indexingBlockRate > 0.5 || embeddingBlockRate > 0.5 {
		return false
	}
	
	return true
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}