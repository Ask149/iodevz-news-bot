// internal/poster/twitter.go
package poster

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	twitterAPIURL = "https://api.twitter.com/2/tweets"
)

// TwitterPoster posts tweets via Twitter API v2 with OAuth 1.0a.
type TwitterPoster struct {
	apiURL         string
	consumerKey    string
	consumerSecret string
	accessToken    string
	accessSecret   string
	client         *http.Client
}

// NewTwitterPoster creates a poster from environment variables.
func NewTwitterPoster() *TwitterPoster {
	return &TwitterPoster{
		apiURL:         twitterAPIURL,
		consumerKey:    os.Getenv("TWITTER_API_KEY"),
		consumerSecret: os.Getenv("TWITTER_API_SECRET"),
		accessToken:    os.Getenv("TWITTER_ACCESS_TOKEN"),
		accessSecret:   os.Getenv("TWITTER_ACCESS_SECRET"),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// IsConfigured returns true if all Twitter credentials are set.
func (p *TwitterPoster) IsConfigured() bool {
	return p.consumerKey != "" && p.consumerSecret != "" &&
		p.accessToken != "" && p.accessSecret != ""
}

// Post sends a tweet and returns the tweet ID.
func (p *TwitterPoster) Post(ctx context.Context, text string) (string, error) {
	payload := map[string]string{"text": text}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Sign with OAuth 1.0a if credentials are available.
	if p.consumerKey != "" {
		p.signRequest(req)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("twitter request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return "", fmt.Errorf("twitter API error: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse twitter response: %w", err)
	}

	log.Printf("[poster] posted tweet %s: %s", result.Data.ID, truncateLog(text, 50))
	return result.Data.ID, nil
}

// signRequest adds OAuth 1.0a Authorization header.
func (p *TwitterPoster) signRequest(req *http.Request) {
	nonce := generateNonce()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	params := map[string]string{
		"oauth_consumer_key":     p.consumerKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        timestamp,
		"oauth_token":            p.accessToken,
		"oauth_version":          "1.0",
	}

	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var paramParts []string
	for _, k := range keys {
		paramParts = append(paramParts, fmt.Sprintf("%s=%s",
			url.QueryEscape(k), url.QueryEscape(params[k])))
	}
	paramString := strings.Join(paramParts, "&")

	baseString := fmt.Sprintf("POST&%s&%s",
		url.QueryEscape(req.URL.String()),
		url.QueryEscape(paramString))

	signingKey := fmt.Sprintf("%s&%s",
		url.QueryEscape(p.consumerSecret),
		url.QueryEscape(p.accessSecret))

	mac := hmac.New(sha1.New, []byte(signingKey))
	mac.Write([]byte(baseString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	params["oauth_signature"] = signature

	var authParts []string
	for k, v := range params {
		authParts = append(authParts, fmt.Sprintf(`%s="%s"`, k, url.QueryEscape(v)))
	}
	req.Header.Set("Authorization", "OAuth "+strings.Join(authParts, ", "))
}

func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func truncateLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
