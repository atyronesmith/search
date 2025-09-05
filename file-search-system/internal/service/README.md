# Background Service Implementation

The background service orchestrates all components of the file search system, providing resource monitoring, rate limiting, lifecycle management, and comprehensive metrics collection.

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                Background Service                    │
├─────────────────────────────────────────────────────┤
│  Service Orchestrator (service.go)                  │
│  ├── Coordinates all components                     │
│  ├── Manages indexing lifecycle                     │
│  ├── Handles auto-pause/resume                      │
│  └── Provides unified statistics                    │
├─────────────────────────────────────────────────────┤
│  Resource Monitor (resource_monitor.go)             │
│  ├── CPU, Memory, Disk monitoring                   │
│  ├── Process-specific metrics                       │
│  ├── Auto-pause on high usage                       │
│  └── Historical data retention                      │
├─────────────────────────────────────────────────────┤
│  Rate Limiter (rate_limiter.go)                     │
│  ├── Adaptive rate limiting                         │
│  ├── Time-based throttling                          │
│  ├── Resource-aware scaling                         │
│  └── Per-operation limits                           │
├─────────────────────────────────────────────────────┤
│  Lifecycle Manager (lifecycle.go)                   │
│  ├── Service state management                       │
│  ├── Graceful shutdown                              │
│  ├── Auto-restart on failure                        │
│  └── Health checking                                │
├─────────────────────────────────────────────────────┤
│  Metrics Collector (metrics.go)                     │
│  ├── Time-series data collection                    │
│  ├── Real-time statistics                           │
│  ├── Performance monitoring                         │
│  └── Custom metrics support                         │
└─────────────────────────────────────────────────────┘
```

## Key Components

### 1. Service Orchestrator (`service.go`)

**Purpose**: Main coordination hub for all file search system components.

**Key Features**:
- **Component Integration**: Coordinates scanner, monitor, search engine, and embeddings
- **State Management**: Tracks indexing active/paused states using atomic operations
- **Event Processing**: Handles indexing and system events through channels
- **Statistics Aggregation**: Provides unified view of system performance
- **Auto-Management**: Automatic pause/resume based on resource usage

**Key Methods**:
```go
func NewService(cfg *config.Config, db *database.DB, log *logrus.Logger) (*Service, error)
func (s *Service) Start() error
func (s *Service) Stop() error
func (s *Service) StartIndexing(paths []string, recursive bool) error
func (s *Service) PauseIndexing() error
func (s *Service) ResumeIndexing() error
func (s *Service) GetStats() *ServiceStats
```

### 2. Resource Monitor (`resource_monitor.go`)

**Purpose**: Monitors system resource usage and implements auto-pause functionality.

**Key Features**:
- **Multi-Metric Monitoring**: CPU, Memory, Disk, Load Average, Goroutines
- **Process Monitoring**: Tracks application-specific resource usage
- **Threshold Management**: Configurable warning and auto-pause thresholds
- **Historical Data**: Maintains rolling history for trend analysis
- **Smart Pausing**: Prevents too-frequent pause/resume cycles

**Configuration Example**:
```go
config := &ResourceConfig{
    CPUThreshold:    80.0,     // Warning at 80% CPU
    MemoryThreshold: 85.0,     // Warning at 85% memory
    AutoPauseCPU:    90.0,     // Auto-pause at 90% CPU
    AutoPauseMemory: 90.0,     // Auto-pause at 90% memory
    CheckInterval:   5 * time.Second,
    HistorySize:     60,       // 5 minutes of history
}
```

### 3. Rate Limiter (`rate_limiter.go`)

**Purpose**: Provides intelligent rate limiting with adaptive and time-based controls.

**Key Features**:
- **Multi-Operation Limiting**: Separate limits for indexing, embeddings, searches
- **Adaptive Rate Control**: Automatically adjusts based on resource pressure
- **Time-Based Throttling**: Reduces activity during business hours
- **Burst Handling**: Allows short bursts while maintaining average rates
- **Statistics Tracking**: Detailed metrics on rate limiting effectiveness

**Rate Limiting Types**:
```go
// Base rates (per minute)
IndexingRate:  60   // files per minute
EmbeddingRate: 120  // embeddings per minute  
SearchRate:    300  // searches per minute

// Adaptive scaling
ReductionFactor: 0.5  // Reduce to 50% under pressure
RecoveryFactor:  1.1  // Increase by 10% when recovering

// Time-based scaling
BusinessHourFactor: 0.7  // Reduce to 70% during 9-5
```

### 4. Lifecycle Manager (`lifecycle.go`)

**Purpose**: Manages service lifecycle, health monitoring, and automatic recovery.

**Key Features**:
- **State Machine**: Tracks service states (starting, running, paused, stopping, etc.)
- **Signal Handling**: Graceful shutdown on SIGINT/SIGTERM
- **Auto-Restart**: Configurable automatic restart on failures
- **Health Checking**: Monitors all components and their health status
- **State Callbacks**: Extensible callback system for state changes

**Service States**:
```go
StateStarting  // Service is starting up
StateRunning   // Normal operation
StatePausing   // Transitioning to paused
StatePaused    // Operations paused
StateResuming  // Transitioning back to running
StateStopping  // Graceful shutdown in progress
StateStopped   // Service stopped
StateError     // Error state (triggers auto-restart)
```

**Health Check Components**:
- Database connectivity
- Embeddings service availability
- Search engine functionality
- Resource usage levels
- Rate limiter health

### 5. Metrics Collector (`metrics.go`)

**Purpose**: Comprehensive metrics collection and time-series data management.

**Key Features**:
- **Time-Series Data**: Historical metrics with configurable retention
- **Real-Time Metrics**: Current system and application state
- **Custom Metrics**: Support for application-specific measurements
- **Statistical Aggregates**: Mean, min, max, standard deviation calculations
- **Data Cleanup**: Automatic removal of old data points

**Metric Types**:
```go
// System Metrics
system.cpu.percent
system.memory.percent
system.disk.percent
system.goroutines

// Service Metrics  
service.files.total
service.files.indexed
service.indexing.rate
service.search.qps
service.search.latency

// Custom Metrics
custom.processing.errors
custom.cache.efficiency
```

## Integration with API Server

The background service integrates seamlessly with the API server:

```go
// API handlers can access service functionality
func (s *Server) handleStartIndexing(w http.ResponseWriter, r *http.Request) {
    // Parse request...
    if err := s.service.StartIndexing(req.Paths, req.Recursive); err != nil {
        s.sendError(w, http.StatusInternalServerError, "failed to start indexing")
        return
    }
    s.sendSuccess(w, map[string]interface{}{"message": "indexing started"})
}

// Real-time updates via WebSocket
func (s *Server) SendIndexingUpdate(status string, progress float64) {
    update := WSIndexingUpdate{
        Status:   status,
        Progress: progress,
        // ... other fields
    }
    s.broadcastWSMessage("indexing_update", update)
}
```

## Configuration

The service uses environment-based configuration:

```bash
# Resource Monitoring
CPU_THRESHOLD=80
MEMORY_THRESHOLD=85
AUTO_PAUSE_CPU=90
AUTO_PAUSE_MEMORY=90

# Rate Limiting
INDEXING_RATE=60
EMBEDDING_RATE=120
SEARCH_RATE=300
ENABLE_ADAPTIVE_RATE=true
ENABLE_TIME_BASED_RATE=true

# Lifecycle Management
AUTO_RESTART=true
MAX_RESTARTS=3
RESTART_COOLDOWN=300  # seconds
SHUTDOWN_TIMEOUT=30   # seconds

# Metrics Collection
METRICS_INTERVAL=10   # seconds
METRICS_RETENTION=24  # hours
```

## Usage Examples

### Starting the Service

```go
// Initialize service
svc, err := service.NewService(cfg, db, log)
if err != nil {
    log.Fatal(err)
}

// Start all components
if err := svc.Start(); err != nil {
    log.Fatal(err)
}

// Start indexing specific paths
err = svc.StartIndexing([]string{
    "/Users/john/Documents",
    "/Users/john/Projects",
}, true)
```

### Monitoring and Control

```go
// Get current statistics
stats := svc.GetStats()
fmt.Printf("Files indexed: %d/%d\n", stats.IndexedFiles, stats.TotalFiles)
fmt.Printf("Processing rate: %.1f files/min\n", stats.ProcessingRate)

// Check indexing status
status := svc.GetIndexingStatus()
fmt.Printf("Active: %v, Paused: %v\n", status["active"], status["paused"])

// Manual pause/resume
if highLoad {
    svc.PauseIndexing()
}
// Later...
svc.ResumeIndexing()
```

### Resource Monitoring

```go
// Get current resource usage
usage := svc.resourceMonitor.GetCurrentUsage()
fmt.Printf("CPU: %.1f%%, Memory: %.1f%%, Disk: %.1f%%\n", 
    usage.CPUPercent, usage.MemoryPercent, usage.DiskPercent)

// Check if under pressure
if svc.resourceMonitor.IsUnderResourcePressure() {
    log.Warn("System under resource pressure")
}

// Get 5-minute average
avgUsage := svc.resourceMonitor.GetAverageUsage(5 * time.Minute)
```

### Metrics Collection

```go
// Record custom metrics
svc.metricsCollector.IncrementCounter("custom.documents.processed", nil)
svc.metricsCollector.SetGauge("custom.queue.size", float64(queueSize), nil)

// Get metrics summary
summary := svc.metricsCollector.GetMetricsSummary()

// Get time-series data
timeSeries := svc.metricsCollector.GetTimeSeries("system.cpu.percent", 1*time.Hour)
for _, point := range timeSeries.DataPoints {
    fmt.Printf("%v: %.2f\n", point.Timestamp, point.Value)
}
```

## Error Handling and Recovery

The service implements comprehensive error handling:

1. **Graceful Degradation**: Components can fail independently without stopping the entire service
2. **Automatic Recovery**: Failed operations are retried with exponential backoff
3. **Resource Protection**: Auto-pause prevents system overload
4. **State Persistence**: Service state is maintained across restarts where possible
5. **Health Monitoring**: Continuous health checks detect and report issues

## Performance Characteristics

- **Low Overhead**: Resource monitoring adds <1% CPU overhead
- **Memory Efficient**: Time-series data uses rolling buffers with automatic cleanup
- **Scalable**: Rate limiting adapts to system capacity
- **Responsive**: Real-time metrics updates and WebSocket notifications
- **Resilient**: Multiple failure modes handled gracefully

## Future Enhancements

1. **Distributed Operation**: Support for multi-node deployments
2. **Machine Learning**: Predictive resource management and optimization
3. **Plugin System**: Extensible architecture for custom components
4. **Advanced Analytics**: More sophisticated metrics and alerting
5. **Configuration Hot-Reload**: Dynamic configuration updates without restart

The background service provides a robust foundation for the file search system with production-ready features for monitoring, control, and reliability.