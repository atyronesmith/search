package extractor

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// IsBinaryFile checks if a file is binary by examining its content
func IsBinaryFile(filePath string) bool {
	// First check by extension
	if isBinaryExtension(filePath) {
		return true
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return true // Assume binary if can't read
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Ignore close error
			_ = err
		}
	}()

	// Read the first 8KB to check for binary content
	buf := make([]byte, 8192)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return true // Assume binary if read error
	}

	if n == 0 {
		return false // Empty file, treat as text
	}

	// Check for null bytes (common in binary files)
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}

	// Check if content is valid UTF-8
	content := buf[:n]
	if !utf8.Valid(content) {
		return true
	}

	// Check for high ratio of non-printable characters
	nonPrintable := 0
	for i := 0; i < n; i++ {
		b := buf[i]
		// Count characters that are not printable ASCII or common whitespace
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonPrintable++
		} else if b > 126 {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider binary
	if float64(nonPrintable)/float64(n) > 0.30 {
		return true
	}

	return false
}

// isBinaryExtension checks if file extension indicates binary content
func isBinaryExtension(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	binaryExtensions := map[string]bool{
		// Executables and libraries
		".exe": true, ".dll": true, ".so": true, ".dylib": true, ".app": true,
		".bin": true, ".obj": true, ".o": true, ".lib": true, ".a": true,

		// Archives and compressed files
		".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true,
		".bz2": true, ".xz": true, ".lzma": true, ".dmg": true, ".pkg": true,

		// Images
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
		".tiff": true, ".tif": true, ".ico": true, ".webp": true, ".svg": false, // SVG is text

		// Audio/Video
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".mkv": true,
		".wav": true, ".flac": true, ".ogg": true, ".m4a": true, ".webm": true,

		// Documents (binary formats)
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".ppt": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true,

		// Database files
		".db": true, ".sqlite": true, ".sqlite3": true, ".mdb": true,

		// Font files
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,

		// Other binary formats
		".iso": true, ".img": true, ".class": true, ".jar": true, ".war": true,
		".pyc": true, ".pyo": true, ".pyd": true, ".wasm": true,
	}

	return binaryExtensions[ext]
}

// IsTextFile checks if a file should be treated as text
func IsTextFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	textExtensions := map[string]bool{
		// Source code
		".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".java": true, ".c": true, ".cpp": true, ".cc": true, ".cxx": true, ".h": true,
		".hpp": true, ".cs": true, ".php": true, ".rb": true, ".rs": true, ".swift": true,
		".kt": true, ".scala": true, ".clj": true, ".hs": true, ".ml": true, ".fs": true,
		".dart": true, ".elm": true, ".erl": true, ".ex": true, ".exs": true, ".r": true,
		".m": true, ".pl": true, ".perl": true, ".sh": true, ".bash": true, ".zsh": true,
		".fish": true, ".ps1": true, ".bat": true, ".cmd": true,

		// Markup and data
		".html": true, ".htm": true, ".xml": true, ".xhtml": true, ".svg": true,
		".json": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true,
		".cfg": true, ".conf": true, ".config": true, ".properties": true,

		// Text files
		".txt": true, ".md": true, ".rst": true, ".rtf": true, ".tex": true,
		".log": true, ".out": true, ".err": true, ".csv": true, ".tsv": true,

		// Web files
		".css": true, ".scss": true, ".sass": true, ".less": true, ".styl": true,
		".vue": true, ".svelte": true, ".astro": true,

		// Config and project files
		".dockerfile": true, ".gitignore": true, ".gitattributes": true,
		".editorconfig": true, ".npmrc": true, ".yarnrc": true,
		".env": true, ".envrc": true, ".bashrc": true, ".zshrc": true,
		".vimrc": true, ".tmux": true, ".profile": true,

		// Documentation
		".readme": true, ".license": true, ".changelog": true, ".authors": true,
		".contributors": true, ".notice": true, ".copying": true,
	}

	if textExtensions[ext] {
		return true
	}

	// Check for files without extensions that are typically text
	basename := strings.ToLower(filepath.Base(filePath))
	textBasenames := map[string]bool{
		"readme": true, "license": true, "changelog": true, "authors": true,
		"contributors": true, "notice": true, "copying": true, "makefile": true,
		"dockerfile": true, "vagrantfile": true, "gemfile": true, "rakefile": true,
		".gitignore": true, ".gitattributes": true, ".dockerignore": true,
	}

	return textBasenames[basename]
}

// HasValidEncoding checks if file content has valid text encoding
func HasValidEncoding(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Ignore close error
			_ = err
		}
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 1MB max line length

	lineCount := 0
	for scanner.Scan() && lineCount < 100 { // Check first 100 lines
		lineCount++
		if !utf8.Valid(scanner.Bytes()) {
			return false
		}
	}

	return scanner.Err() == nil
}
