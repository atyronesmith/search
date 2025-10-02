package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrPathTraversal indicates an attempt to access files outside allowed directories
	ErrPathTraversal = errors.New("path traversal attempt detected")
	// ErrInvalidPath indicates the path is malformed or invalid
	ErrInvalidPath = errors.New("invalid file path")
	// ErrSymlinkNotAllowed indicates symbolic links are not allowed
	ErrSymlinkNotAllowed = errors.New("symbolic links are not allowed")
)

// SanitizePath cleans and validates a file path to prevent directory traversal attacks
func SanitizePath(path string, allowedBasePaths []string) (string, error) {
	if path == "" {
		return "", ErrInvalidPath
	}

	// Clean the path to remove any ../ or ./ sequences
	cleanPath := filepath.Clean(path)

	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path contains suspicious patterns
	if containsSuspiciousPatterns(absPath) {
		return "", ErrPathTraversal
	}

	// If no base paths specified, only allow absolute paths without traversal
	if len(allowedBasePaths) == 0 {
		return absPath, nil
	}

	// Check if the path is within any of the allowed base paths
	for _, basePath := range allowedBasePaths {
		// Expand home directory if needed
		expandedBase := expandHomePath(basePath)

		// Convert base to absolute path
		absBase, err := filepath.Abs(expandedBase)
		if err != nil {
			continue
		}

		// Check if the path is within this base path
		if isPathWithin(absPath, absBase) {
			return absPath, nil
		}
	}

	return "", ErrPathTraversal
}

// ValidateFilePath validates that a path is safe to access
func ValidateFilePath(path string) error {
	if path == "" {
		return ErrInvalidPath
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return ErrInvalidPath
	}

	// Check for suspicious patterns
	if containsSuspiciousPatterns(path) {
		return ErrPathTraversal
	}

	return nil
}

// IsSymlink checks if a path is a symbolic link
func IsSymlink(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return info.Mode()&os.ModeSymlink != 0, nil
}

// ResolveSymlinks resolves a path but ensures it stays within allowed paths
func ResolveSymlinks(path string, allowedBasePaths []string) (string, error) {
	// Check if it's a symlink
	isLink, err := IsSymlink(path)
	if err != nil {
		return "", err
	}

	if !isLink {
		return path, nil
	}

	// Resolve the symlink
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlink: %w", err)
	}

	// Validate the resolved path is within allowed paths
	validated, err := SanitizePath(resolved, allowedBasePaths)
	if err != nil {
		return "", fmt.Errorf("symlink points outside allowed paths: %w", err)
	}

	return validated, nil
}

// containsSuspiciousPatterns checks for common path traversal patterns
func containsSuspiciousPatterns(path string) bool {
	suspicious := []string{
		"../",
		"..\\",
		"..",
		"//",
		"\\\\",
		"\x00", // null bytes
	}

	normalizedPath := strings.ToLower(path)
	for _, pattern := range suspicious {
		if strings.Contains(normalizedPath, pattern) {
			return true
		}
	}

	// Check for encoded traversal patterns
	encoded := []string{
		"%2e%2e%2f", // ../
		"%2e%2e/",
		"..%2f",
		"%2e%2e%5c", // ..\
		"..%5c",
		"%252e%252e", // double encoded
	}

	for _, pattern := range encoded {
		if strings.Contains(normalizedPath, pattern) {
			return true
		}
	}

	return false
}

// isPathWithin checks if a path is within a base directory
func isPathWithin(path, base string) bool {
	// Ensure both paths end with separator for accurate comparison
	basePath := filepath.Clean(base) + string(filepath.Separator)
	targetPath := filepath.Clean(path)

	// Check if target path starts with base path
	return strings.HasPrefix(targetPath, basePath) || targetPath == filepath.Clean(base)
}

// expandHomePath expands ~ to the user's home directory
func expandHomePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// SecureOpen opens a file after validating the path
func SecureOpen(path string, allowedPaths []string) (*os.File, error) {
	// Validate and sanitize the path
	safePath, err := SanitizePath(path, allowedPaths)
	if err != nil {
		return nil, fmt.Errorf("unsafe path: %w", err)
	}

	// Check for symlinks if not allowed
	isLink, err := IsSymlink(safePath)
	if err != nil {
		return nil, err
	}

	if isLink {
		// Optionally resolve symlink or reject
		// For security, we'll reject symlinks by default
		return nil, ErrSymlinkNotAllowed
	}

	// Open the file
	file, err := os.Open(safePath)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// JoinPath safely joins path components
func JoinPath(basePath string, elements ...string) (string, error) {
	// Clean base path
	base := filepath.Clean(basePath)

	// Join all elements
	joined := filepath.Join(append([]string{base}, elements...)...)

	// Ensure the result is still within the base path
	if !isPathWithin(joined, base) {
		return "", ErrPathTraversal
	}

	return joined, nil
}