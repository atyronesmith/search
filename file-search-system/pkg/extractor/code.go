package extractor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// CodeExtractor handles source code files
type CodeExtractor struct {
	config *Config
}

// NewCodeExtractor creates a new code extractor
func NewCodeExtractor(config *Config) *CodeExtractor {
	if config == nil {
		config = DefaultConfig()
	}
	return &CodeExtractor{config: config}
}

// CanExtract checks if this extractor can handle the file
func (e *CodeExtractor) CanExtract(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	supportedExts := map[string]bool{
		".py":   true,
		".js":   true,
		".ts":   true,
		".jsx":  true,
		".tsx":  true,
		".java": true,
		".cpp":  true,
		".c":    true,
		".h":    true,
		".hpp":  true,
		".go":   true,
		".rs":   true,
		".php":  true,
		".rb":   true,
		".swift": true,
		".kt":   true,
		".scala": true,
		".cs":   true,
		".sh":   true,
		".bash": true,
		".zsh":  true,
		".fish": true,
		".ps1":  true,
		".json": true,
		".yaml": true,
		".yml":  true,
		".toml": true,
		".xml":  true,
		".html": true,
		".css":  true,
		".scss": true,
		".sass": true,
		".less": true,
		".sql":  true,
		".r":    true,
		".m":    true,
		".pl":   true,
		".lua":  true,
		".vim":  true,
		".dockerfile": true,
		".makefile": true,
	}
	
	// Also check filename patterns
	filename := strings.ToLower(filepath.Base(filePath))
	filenamePatterns := []string{
		"dockerfile", "makefile", "rakefile", "gemfile",
		"requirements.txt", "package.json", "composer.json",
		"cargo.toml", "go.mod", "go.sum",
	}
	
	for _, pattern := range filenamePatterns {
		if strings.Contains(filename, pattern) {
			return true
		}
	}
	
	return supportedExts[ext]
}

// Extract extracts content from a code file
func (e *CodeExtractor) Extract(ctx context.Context, filePath string) (*ExtractedContent, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Check file size
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	maxSize := int64(e.config.MaxFileSizeMB * 1024 * 1024)
	if info.Size() > maxSize {
		return nil, ErrFileTooLarge
	}

	// Read content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	text := string(content)
	language := e.detectLanguage(filePath)

	// Create metadata
	metadata := map[string]interface{}{
		"file_size":  info.Size(),
		"language":   language,
		"line_count": strings.Count(text, "\n") + 1,
		"char_count": len(text),
		"file_type":  "code",
	}

	// Add language-specific metadata
	e.addLanguageMetadata(metadata, text, language)

	// Parse code structure
	sections := e.parseCodeStructure(text, language)

	return &ExtractedContent{
		Text:     text,
		Metadata: metadata,
		Sections: sections,
	}, nil
}

// GetName returns the extractor name
func (e *CodeExtractor) GetName() string {
	return "CodeExtractor"
}

// GetSupportedExtensions returns supported extensions
func (e *CodeExtractor) GetSupportedExtensions() []string {
	return []string{
		".py", ".js", ".ts", ".jsx", ".tsx", ".java", ".cpp", ".c", ".h", ".hpp",
		".go", ".rs", ".php", ".rb", ".swift", ".kt", ".scala", ".cs",
		".sh", ".bash", ".zsh", ".fish", ".ps1",
		".json", ".yaml", ".yml", ".toml", ".xml",
		".html", ".css", ".scss", ".sass", ".less",
		".sql", ".r", ".m", ".pl", ".lua", ".vim",
	}
}

// detectLanguage detects the programming language
func (e *CodeExtractor) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	filename := strings.ToLower(filepath.Base(filePath))
	
	// Extension mapping
	extLangMap := map[string]string{
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".jsx":  "javascript",
		".tsx":  "typescript",
		".java": "java",
		".cpp":  "cpp",
		".c":    "c",
		".h":    "c",
		".hpp":  "cpp",
		".go":   "go",
		".rs":   "rust",
		".php":  "php",
		".rb":   "ruby",
		".swift": "swift",
		".kt":   "kotlin",
		".scala": "scala",
		".cs":   "csharp",
		".sh":   "shell",
		".bash": "bash",
		".zsh":  "zsh",
		".fish": "fish",
		".ps1":  "powershell",
		".json": "json",
		".yaml": "yaml",
		".yml":  "yaml",
		".toml": "toml",
		".xml":  "xml",
		".html": "html",
		".css":  "css",
		".scss": "scss",
		".sass": "sass",
		".less": "less",
		".sql":  "sql",
		".r":    "r",
		".m":    "objective-c",
		".pl":   "perl",
		".lua":  "lua",
		".vim":  "vim",
	}
	
	if lang, ok := extLangMap[ext]; ok {
		return lang
	}
	
	// Filename patterns
	if strings.Contains(filename, "dockerfile") {
		return "dockerfile"
	}
	if strings.Contains(filename, "makefile") {
		return "makefile"
	}
	
	return "text"
}

// addLanguageMetadata adds language-specific metadata
func (e *CodeExtractor) addLanguageMetadata(metadata map[string]interface{}, content, language string) {
	lines := strings.Split(content, "\n")
	
	// Count comment lines
	commentCount := 0
	emptyLineCount := 0
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			emptyLineCount++
		} else if e.isCommentLine(trimmed, language) {
			commentCount++
		}
	}
	
	metadata["comment_lines"] = commentCount
	metadata["empty_lines"] = emptyLineCount
	metadata["code_lines"] = len(lines) - commentCount - emptyLineCount
	
	// Add language-specific features
	switch language {
	case "python":
		metadata["imports"] = e.countPythonImports(content)
		metadata["functions"] = e.countPythonFunctions(content)
		metadata["classes"] = e.countPythonClasses(content)
	case "javascript", "typescript":
		metadata["imports"] = e.countJSImports(content)
		metadata["functions"] = e.countJSFunctions(content)
		metadata["classes"] = e.countJSClasses(content)
	case "go":
		metadata["packages"] = e.countGoPackages(content)
		metadata["functions"] = e.countGoFunctions(content)
		metadata["structs"] = e.countGoStructs(content)
	}
}

// isCommentLine checks if a line is a comment
func (e *CodeExtractor) isCommentLine(line, language string) bool {
	commentPrefixes := map[string][]string{
		"python":     {"#"},
		"javascript": {"//", "/*"},
		"typescript": {"//", "/*"},
		"java":       {"//", "/*"},
		"cpp":        {"//", "/*"},
		"c":          {"//", "/*"},
		"go":         {"//", "/*"},
		"rust":       {"//", "/*"},
		"php":        {"//", "/*", "#"},
		"ruby":       {"#"},
		"swift":      {"//", "/*"},
		"kotlin":     {"//", "/*"},
		"scala":      {"//", "/*"},
		"csharp":     {"//", "/*"},
		"shell":      {"#"},
		"bash":       {"#"},
		"sql":        {"--", "/*"},
		"r":          {"#"},
		"lua":        {"--"},
		"vim":        {"\""},
	}
	
	prefixes, ok := commentPrefixes[language]
	if !ok {
		return false
	}
	
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	
	return false
}

// parseCodeStructure parses code into structural sections
func (e *CodeExtractor) parseCodeStructure(content, language string) []SectionContent {
	lines := strings.Split(content, "\n")
	
	switch language {
	case "python":
		return e.parsePythonStructure(lines)
	case "javascript", "typescript":
		return e.parseJSStructure(lines)
	case "go":
		return e.parseGoStructure(lines)
	default:
		return e.parseGenericCodeStructure(lines, language)
	}
}

// parseGenericCodeStructure provides basic code structure parsing
func (e *CodeExtractor) parseGenericCodeStructure(lines []string, language string) []SectionContent {
	var sections []SectionContent
	var currentSection []string
	var currentType = "code"
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Detect function-like patterns
		if e.isFunctionDeclaration(trimmed, language) {
			// Save previous section
			if len(currentSection) > 0 {
				sections = append(sections, SectionContent{
					Type:     currentType,
					Text:     strings.Join(currentSection, "\n"),
					Language: language,
				})
			}
			
			// Start new function section
			currentType = "function"
			currentSection = []string{line}
		} else if e.isClassDeclaration(trimmed, language) {
			// Save previous section
			if len(currentSection) > 0 {
				sections = append(sections, SectionContent{
					Type:     currentType,
					Text:     strings.Join(currentSection, "\n"),
					Language: language,
				})
			}
			
			// Start new class section
			currentType = "class"
			currentSection = []string{line}
		} else {
			currentSection = append(currentSection, line)
		}
	}
	
	// Save last section
	if len(currentSection) > 0 {
		sections = append(sections, SectionContent{
			Type:     currentType,
			Text:     strings.Join(currentSection, "\n"),
			Language: language,
		})
	}
	
	return sections
}

// Helper functions for language-specific parsing
func (e *CodeExtractor) isFunctionDeclaration(line, language string) bool {
	patterns := map[string][]string{
		"python":     {"def "},
		"javascript": {"function ", "const ", "let ", "var "},
		"typescript": {"function ", "const ", "let ", "var "},
		"java":       {"public ", "private ", "protected "},
		"cpp":        {"void ", "int ", "bool ", "string "},
		"c":          {"void ", "int ", "bool ", "char "},
		"go":         {"func "},
		"rust":       {"fn "},
	}
	
	if p, ok := patterns[language]; ok {
		for _, pattern := range p {
			if strings.Contains(line, pattern) && strings.Contains(line, "(") {
				return true
			}
		}
	}
	
	return false
}

func (e *CodeExtractor) isClassDeclaration(line, language string) bool {
	patterns := map[string][]string{
		"python":     {"class "},
		"javascript": {"class "},
		"typescript": {"class ", "interface "},
		"java":       {"class ", "interface "},
		"cpp":        {"class ", "struct "},
		"c":          {"struct "},
		"go":         {"type ", "struct "},
		"rust":       {"struct ", "impl ", "trait "},
	}
	
	if p, ok := patterns[language]; ok {
		for _, pattern := range p {
			if strings.HasPrefix(line, pattern) {
				return true
			}
		}
	}
	
	return false
}

// Language-specific counting functions (simplified versions)
func (e *CodeExtractor) countPythonImports(content string) int {
	return strings.Count(content, "import ") + strings.Count(content, "from ")
}

func (e *CodeExtractor) countPythonFunctions(content string) int {
	return strings.Count(content, "def ")
}

func (e *CodeExtractor) countPythonClasses(content string) int {
	return strings.Count(content, "class ")
}

func (e *CodeExtractor) countJSImports(content string) int {
	return strings.Count(content, "import ") + strings.Count(content, "require(")
}

func (e *CodeExtractor) countJSFunctions(content string) int {
	return strings.Count(content, "function ") + strings.Count(content, "=> ")
}

func (e *CodeExtractor) countJSClasses(content string) int {
	return strings.Count(content, "class ")
}

func (e *CodeExtractor) countGoPackages(content string) int {
	return strings.Count(content, "package ")
}

func (e *CodeExtractor) countGoFunctions(content string) int {
	return strings.Count(content, "func ")
}

func (e *CodeExtractor) countGoStructs(content string) int {
	return strings.Count(content, "struct {")
}

// Simplified structure parsing functions
func (e *CodeExtractor) parsePythonStructure(lines []string) []SectionContent {
	return e.parseGenericCodeStructure(lines, "python")
}

func (e *CodeExtractor) parseJSStructure(lines []string) []SectionContent {
	return e.parseGenericCodeStructure(lines, "javascript")
}

func (e *CodeExtractor) parseGoStructure(lines []string) []SectionContent {
	return e.parseGenericCodeStructure(lines, "go")
}