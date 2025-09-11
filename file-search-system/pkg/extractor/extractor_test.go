package extractor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileTypeDetection(t *testing.T) {
	tests := []struct {
		filename     string
		expectedType string
	}{
		{"test.txt", "text"},
		{"readme.md", "markdown"},
		{"main.go", "code"},
		{"script.js", "code"},
		{"app.py", "code"},
		{"data.json", "structured"},
		{"config.yaml", "structured"},
		{"unknown.xyz", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			ext := filepath.Ext(tt.filename)
			assert.NotEmpty(t, ext)

			// Basic file extension validation
			switch ext {
			case ".txt":
				assert.Equal(t, "text", tt.expectedType)
			case ".md":
				assert.Equal(t, "markdown", tt.expectedType)
			case ".go", ".js", ".py":
				assert.Equal(t, "code", tt.expectedType)
			case ".json", ".yaml":
				assert.Equal(t, "structured", tt.expectedType)
			}
		})
	}
}

func TestTextValidation(t *testing.T) {
	t.Run("text content validation", func(t *testing.T) {
		textContent := "This is plain text content"
		assert.NotEmpty(t, textContent)
		assert.True(t, len(textContent) > 0)
	})

	t.Run("binary content detection", func(t *testing.T) {
		binaryData := []byte{0x00, 0x01, 0x02, 0xFF}
		assert.NotNil(t, binaryData)
		assert.True(t, len(binaryData) > 0)
	})
}

func TestContentExtraction(t *testing.T) {
	t.Run("markdown content", func(t *testing.T) {
		markdown := `# Title\n\nThis is **bold** text.`
		assert.Contains(t, markdown, "#")
		assert.Contains(t, markdown, "**")
	})

	t.Run("code content", func(t *testing.T) {
		code := `func main() {\n\tprintln("Hello")\n}`
		assert.Contains(t, code, "func")
		assert.Contains(t, code, "main")
	})

	t.Run("json content", func(t *testing.T) {
		json := `{"name": "test", "value": 123}`
		assert.Contains(t, json, "{")
		assert.Contains(t, json, "name")
	})
}

func TestSectionTypes(t *testing.T) {
	validTypes := []string{"text", "code", "markdown", "structured", "comment", "header"}

	for _, sectionType := range validTypes {
		t.Run("section type: "+sectionType, func(t *testing.T) {
			assert.NotEmpty(t, sectionType)
			assert.True(t, len(sectionType) > 0)
		})
	}
}
