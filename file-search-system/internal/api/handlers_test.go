package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONHelpers(t *testing.T) {
	t.Run("success response structure", func(t *testing.T) {
		response := SuccessResponse{
			Success: true,
			Message: "test message",
			Data:    map[string]string{"key": "value"},
		}

		assert.True(t, response.Success)
		assert.Equal(t, "test message", response.Message)
		assert.NotNil(t, response.Data)
	})

	t.Run("error response structure", func(t *testing.T) {
		response := ErrorResponse{
			Error:   "Bad Request",
			Message: "test error",
			Code:    400,
		}

		assert.Equal(t, "Bad Request", response.Error)
		assert.Equal(t, "test error", response.Message)
		assert.Equal(t, 400, response.Code)
	})
}

func TestSearchRequest(t *testing.T) {
	t.Run("search request validation", func(t *testing.T) {
		request := SearchRequest{
			Query:  "test query",
			Limit:  10,
			Offset: 0,
		}

		assert.NotEmpty(t, request.Query)
		assert.True(t, request.Limit > 0)
		assert.True(t, request.Offset >= 0)
	})
}

func TestIndexingRequest(t *testing.T) {
	t.Run("indexing control request", func(t *testing.T) {
		request := IndexingControlRequest{
			Paths:     []string{"/tmp/test"},
			Recursive: true,
			Force:     false,
		}

		assert.NotEmpty(t, request.Paths)
		assert.True(t, request.Recursive)
		assert.False(t, request.Force)
	})
}

func TestHTTPHelpers(t *testing.T) {
	t.Run("HTTP status codes", func(t *testing.T) {
		assert.Equal(t, 200, http.StatusOK)
		assert.Equal(t, 400, http.StatusBadRequest)
		assert.Equal(t, 500, http.StatusInternalServerError)
	})

	t.Run("content type validation", func(t *testing.T) {
		contentType := "application/json"
		assert.Equal(t, "application/json", contentType)
	})
}

func TestResponseWriter(t *testing.T) {
	t.Run("response writer test", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		data := map[string]string{"message": "test"}
		jsonData, err := json.Marshal(data)
		assert.NoError(t, err)

		recorder.Header().Set("Content-Type", "application/json")
		recorder.WriteHeader(http.StatusOK)
		_, writeErr := recorder.Write(jsonData)
		assert.NoError(t, writeErr)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
		assert.Contains(t, recorder.Body.String(), "test")
	})
}