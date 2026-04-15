// internal/generator/test_helpers_test.go
package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type testTextLLMClient struct {
	serverURL string
}

func newTestTextLLMClient(serverURL string) *testTextLLMClient {
	return &testTextLLMClient{serverURL: serverURL}
}

func (c *testTextLLMClient) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	resp, err := http.Post(c.serverURL, "application/json", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, &apiResp); err != nil {
		return "", err
	}

	return apiResp.Choices[0].Message.Content, nil
}

// errorTextLLMClient always returns an error, simulating LLM unavailability.
type errorTextLLMClient struct{}

func (c *errorTextLLMClient) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	return "", fmt.Errorf("LLM unavailable")
}
