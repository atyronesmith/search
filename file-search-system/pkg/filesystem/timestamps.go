package filesystem

import (
	"log"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/djherbis/times"
)

// FileTimestamps holds both creation and modification times
type FileTimestamps struct {
	CreatedAt    time.Time
	ModifiedAt   time.Time
	HasBirthTime bool
}

// GetFileTimestamps extracts both creation and modification times with fallbacks
func GetFileTimestamps(path string) (FileTimestamps, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileTimestamps{}, err
	}

	// Default fallback - use modification time for both
	result := FileTimestamps{
		CreatedAt:    info.ModTime(),
		ModifiedAt:   info.ModTime(),
		HasBirthTime: false,
	}

	// Try platform-specific approach first (fastest)
	if createdAt, modifiedAt, ok := getPlatformSpecificTimes(info); ok {
		result.CreatedAt = createdAt
		result.ModifiedAt = modifiedAt
		result.HasBirthTime = true
		return result, nil
	}

	// Fallback to times library
	t, err := times.Stat(path)
	if err != nil {
		log.Printf("Warning: Could not get extended timestamps for %s: %v", path, err)
		return result, nil
	}

	result.ModifiedAt = t.ModTime()
	if t.HasBirthTime() {
		result.CreatedAt = t.BirthTime()
		result.HasBirthTime = true
	}

	return result, nil
}

// getPlatformSpecificTimes tries to get times using platform-specific syscalls
func getPlatformSpecificTimes(info os.FileInfo) (createdAt, modifiedAt time.Time, ok bool) {
	switch runtime.GOOS {
	case "darwin":
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			createdAt = time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
			modifiedAt = time.Unix(stat.Mtimespec.Sec, stat.Mtimespec.Nsec)
			return createdAt, modifiedAt, true
		}
	case "windows":
		// Windows-specific code would go here
		// For cross-platform compatibility, we'll fall back to the times library
		// which handles Windows properly
	}
	return time.Time{}, time.Time{}, false
}

// GetFileTimestampsFromInfo is a convenience function that takes os.FileInfo
// This is useful when you already have FileInfo from a directory walk
func GetFileTimestampsFromInfo(path string, info os.FileInfo) FileTimestamps {
	// Default fallback - use modification time for both
	result := FileTimestamps{
		CreatedAt:    info.ModTime(),
		ModifiedAt:   info.ModTime(),
		HasBirthTime: false,
	}

	// Try platform-specific approach first (fastest)
	if createdAt, modifiedAt, ok := getPlatformSpecificTimes(info); ok {
		result.CreatedAt = createdAt
		result.ModifiedAt = modifiedAt
		result.HasBirthTime = true
		return result
	}

	// Fallback to times library
	t, err := times.Stat(path)
	if err != nil {
		log.Printf("Warning: Could not get extended timestamps for %s: %v", path, err)
		return result
	}

	result.ModifiedAt = t.ModTime()
	if t.HasBirthTime() {
		result.CreatedAt = t.BirthTime()
		result.HasBirthTime = true
	}

	return result
}