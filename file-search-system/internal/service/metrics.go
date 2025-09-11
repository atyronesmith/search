package service

import (
	"context"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// MetricsCollector collects and aggregates service metrics
type MetricsCollector struct {
	service *Service
	log     *logrus.Logger
	
	// Metrics storage
	metrics     map[string]*Metric
	metricsLock sync.RWMutex
	
	// Time-series data
	timeSeries     map[string]*TimeSeries
	timeSeriesLock sync.RWMutex
	
	// Collection configuration
	collectionInterval time.Duration
	retentionPeriod   time.Duration
	
	// Control
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
}

// Metric represents a single metric
type Metric struct {
	Name        string      `json:"name"`
	Value       interface{} `json:"value"`
	Type        string      `json:"type"`        // counter, gauge, histogram
	Unit        string      `json:"unit"`
	Description string      `json:"description"`
	LastUpdated time.Time   `json:"last_updated"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// TimeSeries represents time-series data for a metric
type TimeSeries struct {
	Name       string           `json:"name"`
	DataPoints []DataPoint      `json:"data_points"`
	MaxPoints  int              `json:"max_points"`
	Aggregates map[string]float64 `json:"aggregates"`
}

// DataPoint represents a single data point in time series
type DataPoint struct {
	Timestamp time.Time   `json:"timestamp"`
	Value     float64     `json:"value"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// Metrics represents all collected service metrics
type Metrics struct {
	// System metrics
	Uptime             time.Duration `json:"uptime"`
	CPUPercent         float64       `json:"cpu_percent"`
	MemoryPercent      float64       `json:"memory_percent"`
	MemoryUsedMB       uint64        `json:"memory_used_mb"`
	DiskPercent        float64       `json:"disk_percent"`
	GoroutineCount     int           `json:"goroutine_count"`
	
	// Service state
	ServiceState       string        `json:"service_state"`
	IndexingActive     bool          `json:"indexing_active"`
	IndexingPaused     bool          `json:"indexing_paused"`
	
	// File processing metrics
	FilesTotal         int64         `json:"files_total"`
	FilesIndexed       int64         `json:"files_indexed"`
	FilesPending       int64         `json:"files_pending"`
	FilesFailed        int64         `json:"files_failed"`
	IndexingRate       float64       `json:"indexing_rate_per_minute"`
	EmbeddingsGenerated int64        `json:"embeddings_generated"`
	EmbeddingRate      float64       `json:"embedding_rate_per_minute"`
	
	// Search metrics
	SearchQueries      int64         `json:"search_queries"`
	SearchQPS          float64       `json:"search_qps"`
	AvgSearchTime      float64       `json:"avg_search_time_ms"`
	CacheHits          int64         `json:"cache_hits"`
	CacheMisses        int64         `json:"cache_misses"`
	CacheHitRate       float64       `json:"cache_hit_rate"`
	
	// Rate limiting metrics
	RateLimitHits      int64         `json:"rate_limit_hits"`
	IndexingBlocked    int64         `json:"indexing_blocked"`
	EmbeddingBlocked   int64         `json:"embedding_blocked"`
	
	// Error metrics
	ErrorRate          float64       `json:"error_rate"`
	RecentErrors       []ErrorMetric `json:"recent_errors"`
	
	// Database metrics
	DatabaseSize       int64         `json:"database_size_bytes"`
	DatabaseConnections int          `json:"database_connections"`
	
	// Custom metrics
	CustomMetrics      map[string]interface{} `json:"custom_metrics,omitempty"`
}

// ErrorMetric represents error metrics
type ErrorMetric struct {
	Component   string    `json:"component"`
	Message     string    `json:"message"`
	Count       int64     `json:"count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Severity    string    `json:"severity"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(service *Service, log *logrus.Logger) *MetricsCollector {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MetricsCollector{
		service:            service,
		log:               log,
		metrics:           make(map[string]*Metric),
		timeSeries:        make(map[string]*TimeSeries),
		collectionInterval: 10 * time.Second,
		retentionPeriod:   24 * time.Hour,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Start starts the metrics collection
func (mc *MetricsCollector) Start() {
	if mc.running {
		return
	}
	
	mc.running = true
	mc.log.Info("Starting metrics collector")
	
	// Initialize basic metrics
	mc.initializeMetrics()
	
	// Start collection loop
	go mc.collectionLoop()
	
	// Start cleanup loop
	go mc.cleanupLoop()
}

// Stop stops the metrics collection
func (mc *MetricsCollector) Stop() {
	mc.cancel()
	mc.running = false
}

// GetMetrics returns current service metrics
func (mc *MetricsCollector) GetMetrics() *Metrics {
	mc.metricsLock.RLock()
	defer mc.metricsLock.RUnlock()
	
	// Collect current metrics
	metrics := &Metrics{
		Uptime: time.Since(mc.service.startTime),
	}
	
	// Get resource usage
	if mc.service.resourceMonitor != nil {
		usage := mc.service.resourceMonitor.GetCurrentUsage()
		metrics.CPUPercent = usage.CPUPercent
		metrics.MemoryPercent = usage.MemoryPercent
		metrics.MemoryUsedMB = usage.MemoryUsedMB
		metrics.DiskPercent = usage.DiskPercent
		metrics.GoroutineCount = usage.GoroutineCount
	}
	
	// Get service state
	metrics.IndexingActive = atomic.LoadInt32(&mc.service.indexingActive) == 1
	metrics.IndexingPaused = atomic.LoadInt32(&mc.service.indexingPaused) == 1
	
	// Get service statistics
	stats := mc.service.GetStats()
	metrics.FilesTotal = stats.TotalFiles
	metrics.FilesIndexed = stats.IndexedFiles
	metrics.FilesPending = stats.PendingFiles
	metrics.FilesFailed = stats.FailedFiles
	metrics.IndexingRate = float64(stats.ProcessingRate)
	metrics.SearchQPS = stats.SearchQPS
	metrics.AvgSearchTime = stats.AvgSearchTime
	
	// Get rate limiter stats
	if mc.service.rateLimiter != nil {
		rlStats := mc.service.rateLimiter.GetStats()
		metrics.IndexingBlocked = rlStats.IndexingBlocked
		metrics.EmbeddingBlocked = rlStats.EmbeddingBlocked
		metrics.RateLimitHits = rlStats.IndexingBlocked + rlStats.EmbeddingBlocked + rlStats.SearchBlocked
	}
	
	// Calculate derived metrics
	metrics.CacheHitRate = mc.calculateCacheHitRate()
	metrics.ErrorRate = mc.calculateErrorRate()
	
	// Get recent errors
	metrics.RecentErrors = mc.getRecentErrors()
	
	// Add custom metrics
	metrics.CustomMetrics = mc.getCustomMetrics()
	
	return metrics
}

// GetTimeSeries returns time series data for a metric
func (mc *MetricsCollector) GetTimeSeries(metricName string, duration time.Duration) *TimeSeries {
	mc.timeSeriesLock.RLock()
	defer mc.timeSeriesLock.RUnlock()
	
	ts, exists := mc.timeSeries[metricName]
	if !exists {
		return nil
	}
	
	// Filter data points by duration
	cutoff := time.Now().Add(-duration)
	filteredTS := &TimeSeries{
		Name:       ts.Name,
		MaxPoints:  ts.MaxPoints,
		DataPoints: make([]DataPoint, 0),
		Aggregates: make(map[string]float64),
	}
	
	var values []float64
	for _, dp := range ts.DataPoints {
		if dp.Timestamp.After(cutoff) {
			filteredTS.DataPoints = append(filteredTS.DataPoints, dp)
			values = append(values, dp.Value)
		}
	}
	
	// Calculate aggregates
	if len(values) > 0 {
		filteredTS.Aggregates = calculateAggregates(values)
	}
	
	return filteredTS
}

// RecordMetric records a custom metric
func (mc *MetricsCollector) RecordMetric(name string, value interface{}, metricType, unit, description string, tags map[string]string) {
	mc.metricsLock.Lock()
	defer mc.metricsLock.Unlock()
	
	metric := &Metric{
		Name:        name,
		Value:       value,
		Type:        metricType,
		Unit:        unit,
		Description: description,
		LastUpdated: time.Now(),
		Tags:        tags,
	}
	
	mc.metrics[name] = metric
	
	// Add to time series if it's a numeric value
	if numValue, ok := convertToFloat64(value); ok {
		mc.addToTimeSeries(name, numValue, tags)
	}
}

// IncrementCounter increments a counter metric
func (mc *MetricsCollector) IncrementCounter(name string, tags map[string]string) {
	mc.metricsLock.Lock()
	defer mc.metricsLock.Unlock()
	
	if metric, exists := mc.metrics[name]; exists {
		if counter, ok := metric.Value.(int64); ok {
			metric.Value = counter + 1
			metric.LastUpdated = time.Now()
		}
	} else {
		mc.metrics[name] = &Metric{
			Name:        name,
			Value:       int64(1),
			Type:        "counter",
			LastUpdated: time.Now(),
			Tags:        tags,
		}
	}
}

// SetGauge sets a gauge metric value
func (mc *MetricsCollector) SetGauge(name string, value float64, tags map[string]string) {
	mc.metricsLock.Lock()
	defer mc.metricsLock.Unlock()
	
	mc.metrics[name] = &Metric{
		Name:        name,
		Value:       value,
		Type:        "gauge",
		LastUpdated: time.Now(),
		Tags:        tags,
	}
	
	mc.addToTimeSeries(name, value, tags)
}

// GetMetricsSummary returns a summary of all metrics
func (mc *MetricsCollector) GetMetricsSummary() map[string]interface{} {
	mc.metricsLock.RLock()
	defer mc.metricsLock.RUnlock()
	
	summary := make(map[string]interface{})
	
	// Group metrics by type
	counters := make(map[string]interface{})
	gauges := make(map[string]interface{})
	histograms := make(map[string]interface{})
	
	for name, metric := range mc.metrics {
		switch metric.Type {
		case "counter":
			counters[name] = metric.Value
		case "gauge":
			gauges[name] = metric.Value
		case "histogram":
			histograms[name] = metric.Value
		}
	}
	
	summary["counters"] = counters
	summary["gauges"] = gauges
	summary["histograms"] = histograms
	summary["collection_time"] = time.Now()
	summary["total_metrics"] = len(mc.metrics)
	
	return summary
}

// initializeMetrics initializes basic system metrics
func (mc *MetricsCollector) initializeMetrics() {
	// Initialize system metrics
	mc.RecordMetric("system.goroutines", runtime.NumGoroutine(), "gauge", "count", "Number of goroutines", nil)
	mc.RecordMetric("system.uptime", time.Since(mc.service.startTime).Seconds(), "gauge", "seconds", "Service uptime", nil)
	
	// Initialize service metrics
	mc.RecordMetric("service.files.total", int64(0), "gauge", "count", "Total files discovered", nil)
	mc.RecordMetric("service.files.indexed", int64(0), "gauge", "count", "Files successfully indexed", nil)
	mc.RecordMetric("service.files.failed", int64(0), "gauge", "count", "Files that failed indexing", nil)
	
	// Initialize search metrics
	mc.RecordMetric("search.queries.total", int64(0), "counter", "count", "Total search queries", nil)
	mc.RecordMetric("search.cache.hits", int64(0), "counter", "count", "Search cache hits", nil)
	mc.RecordMetric("search.cache.misses", int64(0), "counter", "count", "Search cache misses", nil)
	
	// Initialize time series
	mc.initializeTimeSeries()
}

// initializeTimeSeries initializes time series for key metrics
func (mc *MetricsCollector) initializeTimeSeries() {
	timeSeriesMetrics := []string{
		"system.cpu.percent",
		"system.memory.percent",
		"system.disk.percent",
		"service.indexing.rate",
		"service.search.qps",
		"service.search.latency",
	}
	
	for _, name := range timeSeriesMetrics {
		mc.timeSeries[name] = &TimeSeries{
			Name:       name,
			DataPoints: make([]DataPoint, 0),
			MaxPoints:  1440, // 24 hours of 1-minute data points
			Aggregates: make(map[string]float64),
		}
	}
}

// collectionLoop runs the metrics collection loop
func (mc *MetricsCollector) collectionLoop() {
	ticker := time.NewTicker(mc.collectionInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			mc.collectMetrics()
			
		case <-mc.ctx.Done():
			return
		}
	}
}

// collectMetrics collects current metrics
func (mc *MetricsCollector) collectMetrics() {
	now := time.Now()
	
	// Collect system metrics
	if mc.service.resourceMonitor != nil {
		usage := mc.service.resourceMonitor.GetCurrentUsage()
		
		mc.addToTimeSeries("system.cpu.percent", usage.CPUPercent, nil)
		mc.addToTimeSeries("system.memory.percent", usage.MemoryPercent, nil)
		mc.addToTimeSeries("system.disk.percent", usage.DiskPercent, nil)
		
		mc.SetGauge("system.cpu.percent", usage.CPUPercent, nil)
		mc.SetGauge("system.memory.percent", usage.MemoryPercent, nil)
		mc.SetGauge("system.memory.used_mb", float64(usage.MemoryUsedMB), nil)
		mc.SetGauge("system.disk.percent", usage.DiskPercent, nil)
		mc.SetGauge("system.goroutines", float64(runtime.NumGoroutine()), nil)
	}
	
	// Collect service metrics
	stats := mc.service.GetStats()
	mc.SetGauge("service.files.total", float64(stats.TotalFiles), nil)
	mc.SetGauge("service.files.indexed", float64(stats.IndexedFiles), nil)
	mc.SetGauge("service.files.pending", float64(stats.PendingFiles), nil)
	mc.SetGauge("service.files.failed", float64(stats.FailedFiles), nil)
	
	mc.addToTimeSeries("service.indexing.rate", float64(stats.ProcessingRate), nil)
	
	// Collect rate limiter metrics
	if mc.service.rateLimiter != nil {
		rlStats := mc.service.rateLimiter.GetStats()
		mc.SetGauge("ratelimit.indexing.blocked", float64(rlStats.IndexingBlocked), nil)
		mc.SetGauge("ratelimit.embedding.blocked", float64(rlStats.EmbeddingBlocked), nil)
		mc.SetGauge("ratelimit.adaptive.factor", rlStats.AdaptiveFactor, nil)
	}
	
	mc.log.WithField("timestamp", now).Debug("Metrics collected")
}

// addToTimeSeries adds a data point to a time series
func (mc *MetricsCollector) addToTimeSeries(name string, value float64, tags map[string]string) {
	mc.timeSeriesLock.Lock()
	defer mc.timeSeriesLock.Unlock()
	
	ts, exists := mc.timeSeries[name]
	if !exists {
		ts = &TimeSeries{
			Name:       name,
			DataPoints: make([]DataPoint, 0),
			MaxPoints:  1440,
			Aggregates: make(map[string]float64),
		}
		mc.timeSeries[name] = ts
	}
	
	// Add new data point
	dp := DataPoint{
		Timestamp: time.Now(),
		Value:     value,
		Tags:      tags,
	}
	
	ts.DataPoints = append(ts.DataPoints, dp)
	
	// Trim if exceeds max points
	if len(ts.DataPoints) > ts.MaxPoints {
		ts.DataPoints = ts.DataPoints[1:]
	}
}

// cleanupLoop runs the cleanup loop for old metrics
func (mc *MetricsCollector) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			mc.cleanupOldData()
			
		case <-mc.ctx.Done():
			return
		}
	}
}

// cleanupOldData removes old data points beyond retention period
func (mc *MetricsCollector) cleanupOldData() {
	mc.timeSeriesLock.Lock()
	defer mc.timeSeriesLock.Unlock()
	
	cutoff := time.Now().Add(-mc.retentionPeriod)
	
	for name, ts := range mc.timeSeries {
		originalLen := len(ts.DataPoints)
		
		// Filter out old data points
		filtered := make([]DataPoint, 0)
		for _, dp := range ts.DataPoints {
			if dp.Timestamp.After(cutoff) {
				filtered = append(filtered, dp)
			}
		}
		
		ts.DataPoints = filtered
		
		if len(filtered) < originalLen {
			mc.log.WithFields(logrus.Fields{
				"metric":      name,
				"removed":     originalLen - len(filtered),
				"remaining":   len(filtered),
			}).Debug("Cleaned up old metric data")
		}
	}
}

// calculateCacheHitRate calculates the cache hit rate
func (mc *MetricsCollector) calculateCacheHitRate() float64 {
	mc.metricsLock.RLock()
	defer mc.metricsLock.RUnlock()
	
	hits, hitsOk := mc.metrics["search.cache.hits"]
	misses, missesOk := mc.metrics["search.cache.misses"]
	
	if !hitsOk || !missesOk {
		return 0.0
	}
	
	hitsVal, hitsFloat := hits.Value.(int64)
	missesVal, missesFloat := misses.Value.(int64)
	
	if !hitsFloat || !missesFloat {
		return 0.0
	}
	
	total := hitsVal + missesVal
	if total == 0 {
		return 0.0
	}
	
	return float64(hitsVal) / float64(total) * 100.0
}

// calculateErrorRate calculates the current error rate
func (mc *MetricsCollector) calculateErrorRate() float64 {
	// This would calculate error rate based on recent error metrics
	// For now, return a placeholder
	return 0.0
}

// getRecentErrors returns recent error metrics
func (mc *MetricsCollector) getRecentErrors() []ErrorMetric {
	// This would get recent errors from the service statistics
	// For now, return empty slice
	return []ErrorMetric{}
}

// getCustomMetrics returns custom metrics
func (mc *MetricsCollector) getCustomMetrics() map[string]interface{} {
	mc.metricsLock.RLock()
	defer mc.metricsLock.RUnlock()
	
	custom := make(map[string]interface{})
	
	for name, metric := range mc.metrics {
		if metric.Tags != nil {
			if customTag, exists := metric.Tags["custom"]; exists && customTag == "true" {
				custom[name] = metric.Value
			}
		}
	}
	
	return custom
}

// calculateAggregates calculates statistical aggregates for a set of values
func calculateAggregates(values []float64) map[string]float64 {
	if len(values) == 0 {
		return map[string]float64{}
	}
	
	// Calculate basic statistics
	var sum, min, max float64
	min = values[0]
	max = values[0]
	
	for _, v := range values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	
	mean := sum / float64(len(values))
	
	// Calculate standard deviation
	var variance float64
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(values))
	stddev := math.Sqrt(variance)
	
	return map[string]float64{
		"count":  float64(len(values)),
		"sum":    sum,
		"mean":   mean,
		"min":    min,
		"max":    max,
		"stddev": stddev,
	}
}

// convertToFloat64 converts various numeric types to float64
func convertToFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}