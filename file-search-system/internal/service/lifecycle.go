package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// State represents the current state of the service
type State int

// Service state constants
const (
	// StateUnknown indicates unknown service state
	StateUnknown State = iota
	StateStarting
	StateRunning
	StatePausing
	StatePaused
	StateResuming
	StateStopping
	StateStopped
	StateError
)

// String returns the string representation of the service state
func (s State) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StatePausing:
		return "pausing"
	case StatePaused:
		return "paused"
	case StateResuming:
		return "resuming"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// LifecycleManager manages the service lifecycle
type LifecycleManager struct {
	service *Service
	log     *logrus.Logger
	
	// State management
	state      State
	stateLock  sync.RWMutex
	stateTime  time.Time
	
	// Lifecycle control
	ctx        context.Context
	cancel     context.CancelFunc
	
	// Signal handling
	signalChan chan os.Signal
	
	// Shutdown management
	shutdownTimeout time.Duration
	gracefulStop    chan struct{}
	forceStop       chan struct{}
	
	// Health checking
	healthChecker *HealthChecker
	
	// State change callbacks
	stateCallbacks     map[State][]func(State, State)
	callbacksLock     sync.RWMutex
	
	// Restart management
	restartCount      int
	maxRestarts       int
	restartCooldown   time.Duration
	lastRestart       time.Time
	autoRestart       bool
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager(service *Service, log *logrus.Logger) *LifecycleManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	lm := &LifecycleManager{
		service:         service,
		log:            log,
		state:          StateUnknown,
		stateTime:      time.Now(),
		ctx:            ctx,
		cancel:         cancel,
		signalChan:     make(chan os.Signal, 1),
		shutdownTimeout: 30 * time.Second,
		gracefulStop:   make(chan struct{}),
		forceStop:      make(chan struct{}),
		stateCallbacks: make(map[State][]func(State, State)),
		maxRestarts:    3,
		restartCooldown: 5 * time.Minute,
		autoRestart:    true,
	}
	
	// Initialize health checker
	lm.healthChecker = NewHealthChecker(service, log)
	
	// Set up signal handling
	signal.Notify(lm.signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	
	return lm
}

// Start starts the service lifecycle management
func (lm *LifecycleManager) Start() error {
	lm.setState(StateStarting)
	
	lm.log.Info("Starting service lifecycle management")
	
	// Start signal handler
	go lm.handleSignals()
	
	// Start health checker
	go lm.healthChecker.Start()
	
	// Start the service
	if err := lm.service.Start(); err != nil {
		lm.setState(StateError)
		return fmt.Errorf("failed to start service: %w", err)
	}
	
	lm.setState(StateRunning)
	lm.log.Info("Service lifecycle management started")
	
	return nil
}

// Stop stops the service gracefully
func (lm *LifecycleManager) Stop() error {
	return lm.StopWithTimeout(lm.shutdownTimeout)
}

// StopWithTimeout stops the service with a custom timeout
func (lm *LifecycleManager) StopWithTimeout(timeout time.Duration) error {
	lm.setState(StateStopping)
	lm.log.Info("Stopping service")
	
	// Cancel context to signal shutdown
	lm.cancel()
	
	// Start graceful shutdown
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- lm.service.Stop()
	}()
	
	// Wait for graceful shutdown or timeout
	select {
	case err := <-shutdownDone:
		lm.setState(StateStopped)
		if err != nil {
			lm.log.WithError(err).Error("Error during graceful shutdown")
			return err
		}
		lm.log.Info("Service stopped gracefully")
		return nil
		
	case <-time.After(timeout):
		lm.log.Warn("Graceful shutdown timeout, forcing stop")
		lm.setState(StateStopped)
		return fmt.Errorf("shutdown timeout after %v", timeout)
	}
}

// Pause pauses the service operations
func (lm *LifecycleManager) Pause() error {
	currentState := lm.getState()
	if currentState != StateRunning {
		return fmt.Errorf("cannot pause service in state: %s", currentState)
	}
	
	lm.setState(StatePausing)
	lm.log.Info("Pausing service")
	
	// Pause indexing
	if err := lm.service.PauseIndexing(); err != nil {
		lm.setState(StateError)
		return fmt.Errorf("failed to pause indexing: %w", err)
	}
	
	// Pause rate limiter
	lm.service.rateLimiter.Pause()
	
	lm.setState(StatePaused)
	lm.log.Info("Service paused")
	
	return nil
}

// Resume resumes the service operations
func (lm *LifecycleManager) Resume() error {
	currentState := lm.getState()
	if currentState != StatePaused {
		return fmt.Errorf("cannot resume service in state: %s", currentState)
	}
	
	lm.setState(StateResuming)
	lm.log.Info("Resuming service")
	
	// Resume rate limiter
	lm.service.rateLimiter.Resume()
	
	// Resume indexing
	if err := lm.service.ResumeIndexing(); err != nil {
		lm.setState(StateError)
		return fmt.Errorf("failed to resume indexing: %w", err)
	}
	
	lm.setState(StateRunning)
	lm.log.Info("Service resumed")
	
	return nil
}

// Restart restarts the service
func (lm *LifecycleManager) Restart() error {
	if !lm.canRestart() {
		return fmt.Errorf("restart not allowed: max restarts (%d) reached or cooldown period active", lm.maxRestarts)
	}
	
	lm.log.Info("Restarting service")
	lm.restartCount++
	lm.lastRestart = time.Now()
	
	// Stop the service
	if err := lm.Stop(); err != nil {
		lm.log.WithError(err).Error("Error stopping service during restart")
	}
	
	// Wait a moment before restarting
	time.Sleep(2 * time.Second)
	
	// Start the service again
	if err := lm.Start(); err != nil {
		lm.setState(StateError)
		return fmt.Errorf("failed to restart service: %w", err)
	}
	
	lm.log.Info("Service restarted successfully")
	return nil
}

// GetState returns the current service state
func (lm *LifecycleManager) GetState() State {
	return lm.getState()
}

// GetStateInfo returns detailed state information
func (lm *LifecycleManager) GetStateInfo() map[string]interface{} {
	lm.stateLock.RLock()
	defer lm.stateLock.RUnlock()
	
	return map[string]interface{}{
		"state":           lm.state.String(),
		"state_since":     lm.stateTime,
		"uptime":          time.Since(lm.stateTime),
		"restart_count":   lm.restartCount,
		"can_restart":     lm.canRestart(),
		"auto_restart":    lm.autoRestart,
		"health_status":   lm.healthChecker.GetStatus(),
	}
}

// SetAutoRestart enables or disables automatic restart on failure
func (lm *LifecycleManager) SetAutoRestart(enabled bool) {
	lm.autoRestart = enabled
	lm.log.WithField("auto_restart", enabled).Info("Auto-restart setting changed")
}

// AddStateCallback adds a callback function that will be called on state changes
func (lm *LifecycleManager) AddStateCallback(state State, callback func(State, State)) {
	lm.callbacksLock.Lock()
	defer lm.callbacksLock.Unlock()
	
	lm.stateCallbacks[state] = append(lm.stateCallbacks[state], callback)
}

// Wait waits for the service to stop
func (lm *LifecycleManager) Wait() {
	for {
		state := lm.getState()
		if state == StateStopped || state == StateError {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// handleSignals handles OS signals
func (lm *LifecycleManager) handleSignals() {
	for {
		select {
		case sig := <-lm.signalChan:
			lm.log.WithField("signal", sig).Info("Received signal")
			
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				lm.log.Info("Received shutdown signal")
				go lm.Stop()
				
			case syscall.SIGHUP:
				lm.log.Info("Received reload signal")
				// Could implement configuration reload here
				
			default:
				lm.log.WithField("signal", sig).Debug("Ignored signal")
			}
			
		case <-lm.ctx.Done():
			return
		}
	}
}

// setState updates the service state and calls callbacks
func (lm *LifecycleManager) setState(newState State) {
	lm.stateLock.Lock()
	oldState := lm.state
	lm.state = newState
	lm.stateTime = time.Now()
	lm.stateLock.Unlock()
	
	lm.log.WithFields(logrus.Fields{
		"old_state": oldState.String(),
		"new_state": newState.String(),
	}).Info("Service state changed")
	
	// Call state change callbacks
	lm.callStateCallbacks(oldState, newState)
	
	// Handle auto-restart logic
	if newState == StateError && lm.autoRestart && lm.canRestart() {
		lm.log.Info("Auto-restart triggered due to error state")
		go func() {
			time.Sleep(5 * time.Second) // Wait before restart
			if err := lm.Restart(); err != nil {
				lm.log.WithError(err).Error("Auto-restart failed")
			}
		}()
	}
}

// getState returns the current state (thread-safe)
func (lm *LifecycleManager) getState() State {
	lm.stateLock.RLock()
	defer lm.stateLock.RUnlock()
	return lm.state
}

// callStateCallbacks calls all registered callbacks for state changes
func (lm *LifecycleManager) callStateCallbacks(oldState, newState State) {
	lm.callbacksLock.RLock()
	defer lm.callbacksLock.RUnlock()
	
	// Call callbacks for the new state
	if callbacks, exists := lm.stateCallbacks[newState]; exists {
		for _, callback := range callbacks {
			go func(cb func(State, State)) {
				defer func() {
					if r := recover(); r != nil {
						lm.log.WithField("panic", r).Error("State callback panicked")
					}
				}()
				cb(oldState, newState)
			}(callback)
		}
	}
}

// canRestart checks if the service can be restarted
func (lm *LifecycleManager) canRestart() bool {
	// Check restart count
	if lm.restartCount >= lm.maxRestarts {
		return false
	}
	
	// Check cooldown period
	if time.Since(lm.lastRestart) < lm.restartCooldown {
		return false
	}
	
	return true
}

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Healthy     bool                   `json:"healthy"`
	Status      string                 `json:"status"`
	Message     string                 `json:"message"`
	LastCheck   time.Time              `json:"last_check"`
	CheckCount  int64                  `json:"check_count"`
	FailCount   int64                  `json:"fail_count"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// HealthChecker performs health checks on service components
type HealthChecker struct {
	service     *Service
	log         *logrus.Logger
	
	// Health status
	status      map[string]*HealthStatus
	statusLock  sync.RWMutex
	
	// Configuration
	checkInterval   time.Duration
	timeout        time.Duration
	
	// Control
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(service *Service, log *logrus.Logger) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &HealthChecker{
		service:       service,
		log:          log,
		status:       make(map[string]*HealthStatus),
		checkInterval: 30 * time.Second,
		timeout:      10 * time.Second,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start starts the health checker
func (hc *HealthChecker) Start() {
	if hc.running {
		return
	}
	
	hc.running = true
	hc.log.Info("Starting health checker")
	
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()
	
	// Perform initial health check
	hc.performHealthChecks()
	
	for {
		select {
		case <-ticker.C:
			hc.performHealthChecks()
			
		case <-hc.ctx.Done():
			hc.running = false
			return
		}
	}
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	hc.cancel()
}

// GetStatus returns the current health status
func (hc *HealthChecker) GetStatus() map[string]*HealthStatus {
	hc.statusLock.RLock()
	defer hc.statusLock.RUnlock()
	
	// Return a copy of the status map
	status := make(map[string]*HealthStatus)
	for k, v := range hc.status {
		statusCopy := *v
		status[k] = &statusCopy
	}
	
	return status
}

// IsHealthy returns true if all components are healthy
func (hc *HealthChecker) IsHealthy() bool {
	hc.statusLock.RLock()
	defer hc.statusLock.RUnlock()
	
	for _, status := range hc.status {
		if !status.Healthy {
			return false
		}
	}
	
	return true
}

// performHealthChecks performs health checks on all components
func (hc *HealthChecker) performHealthChecks() {
	hc.log.Debug("Performing health checks")
	
	// Check database
	hc.checkDatabase()
	
	// Check embeddings service
	hc.checkEmbeddings()
	
	// Check search engine
	hc.checkSearchEngine()
	
	// Check resource usage
	hc.checkResourceUsage()
	
	// Check rate limiter
	hc.checkRateLimiter()
}

// checkDatabase checks database health
func (hc *HealthChecker) checkDatabase() {
	ctx, cancel := context.WithTimeout(hc.ctx, hc.timeout)
	defer cancel()
	
	status := &HealthStatus{
		LastCheck:  time.Now(),
		CheckCount: hc.getCheckCount("database") + 1,
	}
	
	// Simple ping test
	err := hc.service.db.Ping(ctx)
	if err != nil {
		status.Healthy = false
		status.Status = "unhealthy"
		status.Message = fmt.Sprintf("Database ping failed: %v", err)
		status.FailCount = hc.getFailCount("database") + 1
	} else {
		status.Healthy = true
		status.Status = "healthy"
		status.Message = "Database is responding"
		status.FailCount = 0
	}
	
	hc.setStatus("database", status)
}

// checkEmbeddings checks embeddings service health
func (hc *HealthChecker) checkEmbeddings() {
	status := &HealthStatus{
		LastCheck:  time.Now(),
		CheckCount: hc.getCheckCount("embeddings") + 1,
	}
	
	// Check if embeddings service is available
	if hc.service.embedder == nil {
		status.Healthy = false
		status.Status = "unavailable"
		status.Message = "Embeddings service not initialized"
		status.FailCount = hc.getFailCount("embeddings") + 1
	} else {
		// You could add a simple health check here
		status.Healthy = true
		status.Status = "healthy"
		status.Message = "Embeddings service available"
		status.FailCount = 0
	}
	
	hc.setStatus("embeddings", status)
}

// checkSearchEngine checks search engine health
func (hc *HealthChecker) checkSearchEngine() {
	status := &HealthStatus{
		LastCheck:  time.Now(),
		CheckCount: hc.getCheckCount("search_engine") + 1,
	}
	
	if hc.service.engine == nil {
		status.Healthy = false
		status.Status = "unavailable"
		status.Message = "Search engine not initialized"
		status.FailCount = hc.getFailCount("search_engine") + 1
	} else {
		// Check cache size and other metrics
		cacheStats := hc.service.engine.GetCacheStats()
		
		status.Healthy = true
		status.Status = "healthy"
		status.Message = "Search engine operational"
		status.FailCount = 0
		status.Details = map[string]interface{}{
			"cache_size": cacheStats.Size,
			"cache_hits": cacheStats.TotalHits,
		}
	}
	
	hc.setStatus("search_engine", status)
}

// checkResourceUsage checks system resource usage
func (hc *HealthChecker) checkResourceUsage() {
	status := &HealthStatus{
		LastCheck:  time.Now(),
		CheckCount: hc.getCheckCount("resources") + 1,
	}
	
	usage := hc.service.resourceMonitor.GetCurrentUsage()
	
	// Consider unhealthy if resources are critically high
	if usage.CPUPercent > 95 || usage.MemoryPercent > 95 {
		status.Healthy = false
		status.Status = "critical"
		status.Message = "Critical resource usage detected"
		status.FailCount = hc.getFailCount("resources") + 1
	} else if usage.CPUPercent > 80 || usage.MemoryPercent > 80 {
		status.Healthy = true
		status.Status = "warning"
		status.Message = "High resource usage detected"
		status.FailCount = 0
	} else {
		status.Healthy = true
		status.Status = "healthy"
		status.Message = "Resource usage normal"
		status.FailCount = 0
	}
	
	status.Details = map[string]interface{}{
		"cpu_percent":    usage.CPUPercent,
		"memory_percent": usage.MemoryPercent,
		"disk_percent":   usage.DiskPercent,
	}
	
	hc.setStatus("resources", status)
}

// checkRateLimiter checks rate limiter health
func (hc *HealthChecker) checkRateLimiter() {
	status := &HealthStatus{
		LastCheck:  time.Now(),
		CheckCount: hc.getCheckCount("rate_limiter") + 1,
	}
	
	if hc.service.rateLimiter.IsHealthy() {
		status.Healthy = true
		status.Status = "healthy"
		status.Message = "Rate limiter operating normally"
		status.FailCount = 0
	} else {
		status.Healthy = false
		status.Status = "degraded"
		status.Message = "Rate limiter showing high block rates"
		status.FailCount = hc.getFailCount("rate_limiter") + 1
	}
	
	rateLimiterStats := hc.service.rateLimiter.GetStats()
	status.Details = map[string]interface{}{
		"indexing_requests": rateLimiterStats.IndexingRequests,
		"indexing_blocked":  rateLimiterStats.IndexingBlocked,
		"adaptive_factor":   rateLimiterStats.AdaptiveFactor,
	}
	
	hc.setStatus("rate_limiter", status)
}

// setStatus sets the health status for a component
func (hc *HealthChecker) setStatus(component string, status *HealthStatus) {
	hc.statusLock.Lock()
	defer hc.statusLock.Unlock()
	
	hc.status[component] = status
}

// getCheckCount gets the check count for a component
func (hc *HealthChecker) getCheckCount(component string) int64 {
	hc.statusLock.RLock()
	defer hc.statusLock.RUnlock()
	
	if status, exists := hc.status[component]; exists {
		return status.CheckCount
	}
	return 0
}

// getFailCount gets the fail count for a component
func (hc *HealthChecker) getFailCount(component string) int64 {
	hc.statusLock.RLock()
	defer hc.statusLock.RUnlock()
	
	if status, exists := hc.status[component]; exists {
		return status.FailCount
	}
	return 0
}