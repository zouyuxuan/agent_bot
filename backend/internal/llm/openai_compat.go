package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Provider string

const (
	ProviderOpenAICompat Provider = "openai_compat"
)

type Config struct {
	Provider     Provider `json:"provider"`
	APIKey       string   `json:"apiKey"`
	BaseURL      string   `json:"baseUrl"`
	Model        string   `json:"model"`
	Temperature  float64  `json:"temperature"`
	MaxTokens    int      `json:"maxTokens"`
	TimeoutMS    int      `json:"timeoutMs"`
	ForceDisable bool     `json:"forceDisable"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAICompatClient struct {
	http *http.Client
}

func NewOpenAICompatClient() *OpenAICompatClient {
	return &OpenAICompatClient{
		http: &http.Client{},
	}
}

func (c *OpenAICompatClient) Chat(ctx context.Context, cfg Config, messages []Message) (string, error) {
	if cfg.ForceDisable {
		return "", errors.New("llm disabled")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return "", errors.New("missing apiKey")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return "", errors.New("missing model")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}
	endpoint := baseURL + "/chat/completions"

	type requestBody struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		Temperature float64   `json:"temperature,omitempty"`
		MaxTokens   int       `json:"max_tokens,omitempty"`
	}

	body := requestBody{
		Model:       cfg.Model,
		Messages:    messages,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	callCtx := ctx
	if cfg.TimeoutMS > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, time.Duration(cfg.TimeoutMS)*time.Millisecond)
		defer cancel()
	} else {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	type responseBody struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	var parsed responseBody
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			return "", fmt.Errorf("llm http %d: %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("llm http %d", resp.StatusCode)
	}

	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return "", errors.New(parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("llm returned empty choices")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("llm returned empty content")
	}
	return content, nil
}
