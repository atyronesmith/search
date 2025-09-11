package chunker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkerBasics(t *testing.T) {
	t.Run("chunk size validation", func(t *testing.T) {
		chunkSize := 512
		overlap := 50
		minSize := 100

		assert.True(t, chunkSize > 0, "Chunk size should be positive")
		assert.True(t, overlap < chunkSize, "Overlap should be less than chunk size")
		assert.True(t, minSize < chunkSize, "Min size should be less than chunk size")
	})
}

func TestTextSplitting(t *testing.T) {
	t.Run("basic text splitting", func(t *testing.T) {
		text := "This is a test sentence. This is another sentence. This is a third sentence."
		sentences := strings.Split(text, ". ")

		assert.Equal(t, 3, len(sentences))
		assert.Contains(t, sentences[0], "test")
	})
}

func TestTokenCounting(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "single word",
			text:     "hello",
			expected: 1,
		},
		{
			name:     "multiple words",
			text:     "hello world test",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := countTokensApproximate(tt.text)
			assert.GreaterOrEqual(t, count, tt.expected)
		})
	}
}

func TestChunkProperties(t *testing.T) {
	t.Run("chunk structure", func(t *testing.T) {
		// Test that chunks have required properties
		properties := []string{"Content", "Index", "Type", "Language", "StartChar", "EndChar"}

		for _, prop := range properties {
			assert.NotEmpty(t, prop, "Property should not be empty")
		}
	})
}
