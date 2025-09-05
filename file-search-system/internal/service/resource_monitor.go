package service

import (
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// ResourceUsage represents current system resource usage
type ResourceUsage struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryUsedMB  uint64  `json:"memory_used_mb"`
	MemoryTotalMB uint64  `json:"memory_total_mb"`
	DiskUsedGB    float64 `json:"disk_used_gb"`
	DiskTotalGB   float64 `json:"disk_total_gb"`
	DiskPercent   float64 `json:"disk_percent"`
	LoadAvg1      float64 `json:"load_avg_1"`
	LoadAvg5      float64 `json:"load_avg_5"`
	LoadAvg15     float64 `json:"load_avg_15"`
	GoroutineCount int    `json:"goroutine_count"`
	ProcessCPU    float64 `json:"process_cpu_percent"`
	ProcessMemMB  uint64  `json:"process_memory_mb"`
}

// ResourceConfig holds configuration for resource monitoring
type ResourceConfig struct {
	CPUThreshold    float64       `json:"cpu_threshold"`
	MemoryThreshold float64       `json:"memory_threshold"`
	DiskThreshold   float64       `json:"disk_threshold"`
	CheckInterval   time.Duration `json:"check_interval"`
	HistorySize     int           `json:"history_size"`
	
	// Auto-pause thresholds
	AutoPauseCPU    float64 `json:"auto_pause_cpu"`
	AutoPauseMemory float64 `json:"auto_pause_memory"`
	AutoPauseDisk   float64 `json:"auto_pause_disk"`
	
	// Resume thresholds (should be lower than pause thresholds)
	ResumeCPU    float64 `json:"resume_cpu"`
	ResumeMemory float64 `json:"resume_memory"`
	ResumeDisk   float64 `json:"resume_disk"`
}

// ResourceMonitor monitors system resource usage
type ResourceMonitor struct {
	config  *ResourceConfig
	log     *logrus.Logger
	
	// Resource history
	history     []ResourceUsage
	historyLock sync.RWMutex
	
	// Current state
	currentUsage ResourceUsage
	usageLock    sync.RWMutex
	
	// Process monitoring
	process *process.Process
	
	// Thresholds and state
	lastPauseTime time.Time
	pauseCount    int64
	
	// Metrics
	startTime time.Time
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(config *ResourceConfig, log *logrus.Logger) *ResourceMonitor {
	// Set default values if not provided
	if config.CPUThreshold == 0 {
		config.CPUThreshold = 80.0
	}
	if config.MemoryThreshold == 0 {
		config.MemoryThreshold = 85.0
	}
	if config.DiskThreshold == 0 {
		config.DiskThreshold = 90.0
	}
	if config.CheckInterval == 0 {
		config.CheckInterval = 5 * time.Second
	}
	if config.HistorySize == 0 {
		config.HistorySize = 60 // 5 minutes of history at 5-second intervals
	}
	
	// Set auto-pause thresholds (higher than warning thresholds)
	if config.AutoPauseCPU == 0 {
		config.AutoPauseCPU = config.CPUThreshold + 10
	}
	if config.AutoPauseMemory == 0 {
		config.AutoPauseMemory = config.MemoryThreshold + 5
	}
	if config.AutoPauseDisk == 0 {
		config.AutoPauseDisk = config.DiskThreshold + 5
	}
	
	// Set resume thresholds (lower than pause thresholds)
	if config.ResumeCPU == 0 {
		config.ResumeCPU = config.AutoPauseCPU - 15
	}
	if config.ResumeMemory == 0 {
		config.ResumeMemory = config.AutoPauseMemory - 10
	}
	if config.ResumeDisk == 0 {
		config.ResumeDisk = config.AutoPauseDisk - 10
	}
	
	// Get current process for monitoring
	proc, err := process.NewProcess(int32(runtime.GOMAXPROCS(0)))
	if err != nil {
		log.WithError(err).Warn("Could not get current process for monitoring")
	}
	
	rm := &ResourceMonitor{
		config:    config,
		log:       log,
		history:   make([]ResourceUsage, 0, config.HistorySize),
		process:   proc,
		startTime: time.Now(),
	}
	
	// Get initial reading
	rm.updateUsage()
	
	return rm
}

// GetCurrentUsage returns the current resource usage
func (rm *ResourceMonitor) GetCurrentUsage() ResourceUsage {
	rm.usageLock.RLock()
	defer rm.usageLock.RUnlock()
	return rm.currentUsage
}

// GetUsageHistory returns the resource usage history
func (rm *ResourceMonitor) GetUsageHistory() []ResourceUsage {
	rm.historyLock.RLock()
	defer rm.historyLock.RUnlock()
	
	// Return a copy of the history
	history := make([]ResourceUsage, len(rm.history))
	copy(history, rm.history)
	return history
}

// ShouldPauseIndexing determines if indexing should be paused based on resource usage
func (rm *ResourceMonitor) ShouldPauseIndexing(usage ResourceUsage) bool {
	// Check if any threshold is exceeded
	cpuExceeded := usage.CPUPercent > rm.config.AutoPauseCPU
	memoryExceeded := usage.MemoryPercent > rm.config.AutoPauseMemory
	diskExceeded := usage.DiskPercent > rm.config.AutoPauseDisk
	
	// Additional check: don't pause too frequently
	timeSinceLastPause := time.Since(rm.lastPauseTime)
	if timeSinceLastPause < 30*time.Second {
		return false
	}
	
	shouldPause := cpuExceeded || memoryExceeded || diskExceeded
	
	if shouldPause {
		rm.lastPauseTime = time.Now()
		rm.pauseCount++
		
		rm.log.WithFields(logrus.Fields{
			"cpu_percent":    usage.CPUPercent,
			"memory_percent": usage.MemoryPercent,
			"disk_percent":   usage.DiskPercent,
			"cpu_threshold":  rm.config.AutoPauseCPU,
			"mem_threshold":  rm.config.AutoPauseMemory,
			"disk_threshold": rm.config.AutoPauseDisk,
		}).Warn("Resource thresholds exceeded, recommending pause")
	}
	
	return shouldPause
}

// ShouldResumeIndexing determines if indexing can be resumed based on resource usage
func (rm *ResourceMonitor) ShouldResumeIndexing(usage ResourceUsage) bool {
	// Check if all thresholds are below resume levels
	cpuOk := usage.CPUPercent < rm.config.ResumeCPU
	memoryOk := usage.MemoryPercent < rm.config.ResumeMemory
	diskOk := usage.DiskPercent < rm.config.ResumeDisk
	
	canResume := cpuOk && memoryOk && diskOk
	
	if canResume {
		rm.log.WithFields(logrus.Fields{
			"cpu_percent":    usage.CPUPercent,
			"memory_percent": usage.MemoryPercent,
			"disk_percent":   usage.DiskPercent,
		}).Info("Resource usage normalized, can resume indexing")
	}
	
	return canResume
}

// UpdateUsage updates the current resource usage measurements
func (rm *ResourceMonitor) UpdateUsage() {
	rm.updateUsage()
}

// updateUsage collects current resource usage data
func (rm *ResourceMonitor) updateUsage() {
	usage := ResourceUsage{
		GoroutineCount: runtime.NumGoroutine(),
	}
	
	// Get CPU usage
	if cpuPercents, err := cpu.Percent(time.Second, false); err == nil && len(cpuPercents) > 0 {
		usage.CPUPercent = cpuPercents[0]
	} else {
		rm.log.WithError(err).Debug("Could not get CPU usage")
	}
	
	// Get memory usage
	if memInfo, err := mem.VirtualMemory(); err == nil {
		usage.MemoryPercent = memInfo.UsedPercent
		usage.MemoryUsedMB = memInfo.Used / 1024 / 1024
		usage.MemoryTotalMB = memInfo.Total / 1024 / 1024
	} else {
		rm.log.WithError(err).Debug("Could not get memory usage")
	}
	
	// Get disk usage for root partition
	if diskInfo, err := disk.Usage("/"); err == nil {
		usage.DiskUsedGB = float64(diskInfo.Used) / 1024 / 1024 / 1024
		usage.DiskTotalGB = float64(diskInfo.Total) / 1024 / 1024 / 1024
		usage.DiskPercent = diskInfo.UsedPercent
	} else {
		rm.log.WithError(err).Debug("Could not get disk usage")
	}
	
	// Get process-specific CPU and memory usage
	if rm.process != nil {
		if processCPU, err := rm.process.CPUPercent(); err == nil {
			usage.ProcessCPU = processCPU
		}
		
		if processMemInfo, err := rm.process.MemoryInfo(); err == nil {
			usage.ProcessMemMB = processMemInfo.RSS / 1024 / 1024
		}
	}
	
	// Update current usage
	rm.usageLock.Lock()
	rm.currentUsage = usage
	rm.usageLock.Unlock()
	
	// Add to history
	rm.addToHistory(usage)
}

// addToHistory adds a usage measurement to the history
func (rm *ResourceMonitor) addToHistory(usage ResourceUsage) {
	rm.historyLock.Lock()
	defer rm.historyLock.Unlock()
	
	// Add new measurement
	rm.history = append(rm.history, usage)
	
	// Trim history if it exceeds the size limit
	if len(rm.history) > rm.config.HistorySize {
		rm.history = rm.history[1:]
	}
}

// GetAverageUsage returns average resource usage over the specified duration
func (rm *ResourceMonitor) GetAverageUsage(duration time.Duration) ResourceUsage {
	rm.historyLock.RLock()
	defer rm.historyLock.RUnlock()
	
	if len(rm.history) == 0 {
		return rm.currentUsage
	}
	
	// Calculate how many samples to include based on duration
	samplesNeeded := int(duration / rm.config.CheckInterval)
	if samplesNeeded > len(rm.history) {
		samplesNeeded = len(rm.history)
	}
	
	// Start from the most recent samples
	startIndex := len(rm.history) - samplesNeeded
	
	var totalCPU, totalMemory, totalDisk float64
	var totalMemUsed, totalMemTotal uint64
	count := 0
	
	for i := startIndex; i < len(rm.history); i++ {
		totalCPU += rm.history[i].CPUPercent
		totalMemory += rm.history[i].MemoryPercent
		totalDisk += rm.history[i].DiskPercent
		totalMemUsed += rm.history[i].MemoryUsedMB
		totalMemTotal += rm.history[i].MemoryTotalMB
		count++
	}
	
	if count == 0 {
		return rm.currentUsage
	}
	
	return ResourceUsage{
		CPUPercent:     totalCPU / float64(count),
		MemoryPercent:  totalMemory / float64(count),
		DiskPercent:    totalDisk / float64(count),
		MemoryUsedMB:   totalMemUsed / uint64(count),
		MemoryTotalMB:  totalMemTotal / uint64(count),
		GoroutineCount: rm.currentUsage.GoroutineCount, // Use current value
	}
}

// IsUnderResourcePressure checks if the system is under resource pressure
func (rm *ResourceMonitor) IsUnderResourcePressure() bool {
	usage := rm.GetCurrentUsage()
	
	cpuPressure := usage.CPUPercent > rm.config.CPUThreshold
	memoryPressure := usage.MemoryPercent > rm.config.MemoryThreshold
	diskPressure := usage.DiskPercent > rm.config.DiskThreshold
	
	return cpuPressure || memoryPressure || diskPressure
}

// GetResourceSummary returns a summary of current resource state
func (rm *ResourceMonitor) GetResourceSummary() map[string]interface{} {
	usage := rm.GetCurrentUsage()
	avgUsage := rm.GetAverageUsage(5 * time.Minute)
	
	return map[string]interface{}{
		"current": usage,
		"average_5min": avgUsage,
		"under_pressure": rm.IsUnderResourcePressure(),
		"thresholds": map[string]interface{}{
			"cpu_warning":     rm.config.CPUThreshold,
			"memory_warning":  rm.config.MemoryThreshold,
			"disk_warning":    rm.config.DiskThreshold,
			"cpu_auto_pause":  rm.config.AutoPauseCPU,
			"mem_auto_pause":  rm.config.AutoPauseMemory,
			"disk_auto_pause": rm.config.AutoPauseDisk,
		},
		"pause_count": rm.pauseCount,
		"uptime": time.Since(rm.startTime),
	}
}

// Start begins resource monitoring
func (rm *ResourceMonitor) Start() {
	go rm.monitorLoop()
}

// monitorLoop runs the resource monitoring loop
func (rm *ResourceMonitor) monitorLoop() {
	ticker := time.NewTicker(rm.config.CheckInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		rm.updateUsage()
		
		// Log warnings if thresholds are exceeded
		usage := rm.GetCurrentUsage()
		if usage.CPUPercent > rm.config.CPUThreshold {
			rm.log.WithField("cpu_percent", usage.CPUPercent).Warn("High CPU usage detected")
		}
		if usage.MemoryPercent > rm.config.MemoryThreshold {
			rm.log.WithField("memory_percent", usage.MemoryPercent).Warn("High memory usage detected")
		}
		if usage.DiskPercent > rm.config.DiskThreshold {
			rm.log.WithField("disk_percent", usage.DiskPercent).Warn("High disk usage detected")
		}
	}
}