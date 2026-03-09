// internal/ranker/test_helpers_test.go
package ranker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// testLLMClient mocks the LLM for testing.
type testLLMClient struct {
	serverURL string
}

func newTestLLMClient(serverURL string) *testLLMClient {
	return &testLLMClient{serverURL: serverURL}
}

func (c *testLLMClient) ChatJSON(ctx context.Context, systemPrompt, userMessage string, v interface{}) error {
	resp, err := http.Post(c.serverURL, "application/json", nil)
	if err != nil {
		return err
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
		return err
	}

	return json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), v)
}
