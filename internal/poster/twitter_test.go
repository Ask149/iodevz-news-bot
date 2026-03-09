// internal/poster/twitter_test.go
package poster

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostTweet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}

		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		json.Unmarshal(body, &req)
		if req["text"] == "" {
			t.Error("missing tweet text")
		}

		resp := map[string]interface{}{
			"data": map[string]string{
				"id":   "1234567890",
				"text": req["text"],
			},
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	poster := &TwitterPoster{
		apiURL:         server.URL,
		consumerKey:    "test-key",
		consumerSecret: "test-secret",
		accessToken:    "test-token",
		accessSecret:   "test-access-secret",
		client:         http.DefaultClient,
	}

	tweetID, err := poster.Post(context.Background(), "Test tweet content")
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if tweetID != "1234567890" {
		t.Errorf("tweet ID: got %q, want %q", tweetID, "1234567890")
	}
}

func TestPostTweetHandlesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"detail": "forbidden"}`))
	}))
	defer server.Close()

	poster := &TwitterPoster{
		apiURL: server.URL,
		client: http.DefaultClient,
	}

	_, err := poster.Post(context.Background(), "Test")
	if err == nil {
		t.Fatal("expected error on 403")
	}
}
