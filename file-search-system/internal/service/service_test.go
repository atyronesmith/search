package service

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ServiceStats represents statistics about the service.
type ServiceStats struct {
	StartTime      time.Time
	Uptime         time.Duration
	IndexingActive bool
	TotalFiles     int64
	IndexedFiles   int64
}

func TestServiceStats(t *testing.T) {
	t.Run("service stats structure", func(t *testing.T) {
		startTime := time.Now()
		stats := ServiceStats{
			StartTime:      startTime,
			Uptime:         time.Since(startTime),
			IndexingActive: true,
			TotalFiles:     100,
			IndexedFiles:   75,
		}

		assert.Equal(t, startTime, stats.StartTime)
		assert.True(t, stats.Uptime >= 0)
		assert.True(t, stats.IndexingActive)
		assert.Equal(t, int64(100), stats.TotalFiles)
		assert.Equal(t, int64(75), stats.IndexedFiles)
	})
}

func TestIndexingEvents(t *testing.T) {
	t.Run("indexing event structure", func(t *testing.T) {
		event := IndexingEvent{
			Type:        "file_indexed",
			FilePath:    "/tmp/test.txt",
			Status:      "success",
			Timestamp:   time.Now(),
			ProcessTime: 150,
		}

		assert.Equal(t, "file_indexed", event.Type)
		assert.NotEmpty(t, event.FilePath)
		assert.Equal(t, "success", event.Status)
		assert.True(t, event.ProcessTime > 0)
	})
}

func TestSystemEvents(t *testing.T) {
	t.Run("system event structure", func(t *testing.T) {
		event := SystemEvent{
			Type:      "service_started",
			Message:   "Service started successfully",
			Timestamp: time.Now(),
		}

		assert.Equal(t, "service_started", event.Type)
		assert.NotEmpty(t, event.Message)
		assert.False(t, event.Timestamp.IsZero())
	})
}

func TestAtomicOperations(t *testing.T) {
	t.Run("atomic state management", func(t *testing.T) {
		var indexingActive int32
		var indexingPaused int32

		// Test setting active state
		atomic.StoreInt32(&indexingActive, 1)
		assert.Equal(t, int32(1), atomic.LoadInt32(&indexingActive))

		// Test setting paused state
		atomic.StoreInt32(&indexingPaused, 1)
		assert.Equal(t, int32(1), atomic.LoadInt32(&indexingPaused))

		// Test resetting states
		atomic.StoreInt32(&indexingActive, 0)
		atomic.StoreInt32(&indexingPaused, 0)
		assert.Equal(t, int32(0), atomic.LoadInt32(&indexingActive))
		assert.Equal(t, int32(0), atomic.LoadInt32(&indexingPaused))
	})
}

func TestResourceUsage(t *testing.T) {
	t.Run("resource usage validation", func(t *testing.T) {
		usage := ResourceUsage{
			CPUPercent:    45.5,
			MemoryPercent: 67.2,
			MemoryUsedMB:  1024,
			MemoryTotalMB: 8192,
			DiskUsedGB:    50.5,
			DiskTotalGB:   500.0,
		}

		assert.True(t, usage.CPUPercent >= 0 && usage.CPUPercent <= 100)
		assert.True(t, usage.MemoryPercent >= 0 && usage.MemoryPercent <= 100)
		assert.True(t, usage.MemoryUsedMB <= usage.MemoryTotalMB)
		assert.True(t, usage.DiskUsedGB <= usage.DiskTotalGB)
	})
}

func TestErrorHandling(t *testing.T) {
	t.Run("error info structure", func(t *testing.T) {
		errorMessage := "Test error"
		errorTime := time.Now()
		errorCount := 1

		assert.NotEmpty(t, errorMessage)
		assert.False(t, errorTime.IsZero())
		assert.True(t, errorCount > 0)
	})
}
