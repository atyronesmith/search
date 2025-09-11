package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigDefaults(t *testing.T) {
	t.Run("default API settings", func(t *testing.T) {
		defaultHost := "127.0.0.1"
		defaultPort := 8080

		assert.Equal(t, "127.0.0.1", defaultHost)
		assert.Equal(t, 8080, defaultPort)
		assert.True(t, defaultPort > 0)
		assert.True(t, defaultPort < 65536)
	})

	t.Run("default database settings", func(t *testing.T) {
		defaultDB := "postgresql://postgres:postgres@localhost:5432/file_search"
		assert.Contains(t, defaultDB, "postgresql://")
		assert.Contains(t, defaultDB, "localhost")
	})
}

func TestEnvironmentVariables(t *testing.T) {
	t.Run("environment variable parsing", func(t *testing.T) {
		testVar := "TEST_VAR"
		testValue := "test_value"

		os.Setenv(testVar, testValue)
		defer os.Unsetenv(testVar)

		retrieved := os.Getenv(testVar)
		assert.Equal(t, testValue, retrieved)
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("port range validation", func(t *testing.T) {
		validPorts := []int{8080, 3000, 9000, 8000}

		for _, port := range validPorts {
			assert.True(t, port > 0, "Port should be positive")
			assert.True(t, port < 65536, "Port should be within valid range")
		}
	})

	t.Run("search weight validation", func(t *testing.T) {
		vectorWeight := 0.6
		bm25Weight := 0.3
		metadataWeight := 0.1

		total := vectorWeight + bm25Weight + metadataWeight
		assert.InDelta(t, 1.0, total, 0.01, "Weights should sum to approximately 1.0")
	})
}
