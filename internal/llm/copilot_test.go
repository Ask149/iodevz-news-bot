// internal/llm/copilot_test.go
package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatParsesResponse(t *testing.T) {
	// Mock Copilot API server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request.
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing Content-Type header")
		}

		resp := map[string]interface{}{
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "Test response from LLM",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 5,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		token:          "test-token",
		completionsURL: server.URL,
		model:          "gpt-4o",
		httpClient:     http.DefaultClient,
	}

	result, err := client.Chat(context.Background(), "system prompt", "user message")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if result != "Test response from LLM" {
		t.Errorf("got %q, want %q", result, "Test response from LLM")
	}
}

func TestChatHandlesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := &Client{
		token:          "test-token",
		completionsURL: server.URL,
		model:          "gpt-4o",
		httpClient:     http.DefaultClient,
	}

	_, err := client.Chat(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error on 500")
	}
}
