package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Provider string

const (
	ProviderOpenAICompat Provider = "openai_compat"
	ProviderAnthropic    Provider = "anthropic"
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
	if cfg.Provider == "" {
		cfg.Provider = ProviderOpenAICompat
	}
	switch cfg.Provider {
	case ProviderAnthropic:
		return c.chatAnthropic(ctx, cfg, messages)
	default:
		return c.chatOpenAICompat(ctx, cfg, messages)
	}
}

func (c *OpenAICompatClient) chatOpenAICompat(ctx context.Context, cfg Config, messages []Message) (string, error) {
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
		baseURL = "https://api.openai.com/v1"
	}
	endpoint := strings.TrimRight(baseURL, "/")
	switch {
	case strings.HasSuffix(endpoint, "/chat/completions"):
		// explicit endpoint
	case strings.HasSuffix(endpoint, "/v1"),
		strings.Contains(endpoint, "/compatible-mode/v1"),
		strings.Contains(endpoint, "/v1beta/openai"),
		strings.Contains(endpoint, "/api/paas/v4"):
		endpoint = endpoint + "/chat/completions"
	default:
		endpoint = endpoint + "/v1/chat/completions"
	}

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

func (c *OpenAICompatClient) chatAnthropic(ctx context.Context, cfg Config, messages []Message) (string, error) {
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
		baseURL = "https://api.anthropic.com/v1"
	}
	endpoint := strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(endpoint, "/messages") {
		endpoint = endpoint + "/messages"
	}

	system := ""
	anthropicMessages := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if role == "system" {
			if system != "" {
				system += "\n\n"
			}
			system += content
			continue
		}
		if role != "assistant" {
			role = "user"
		}
		anthropicMessages = append(anthropicMessages, map[string]any{
			"role": role,
			"content": []map[string]string{
				{"type": "text", "text": content},
			},
		})
	}
	if len(anthropicMessages) == 0 {
		return "", errors.New("anthropic request has no messages")
	}

	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	body := map[string]any{
		"model":      cfg.Model,
		"messages":   anthropicMessages,
		"max_tokens": maxTokens,
	}
	if strings.TrimSpace(system) != "" {
		body["system"] = system
	}
	if cfg.Temperature > 0 {
		body["temperature"] = cfg.Temperature
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
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
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

	var out strings.Builder
	for _, item := range parsed.Content {
		if strings.TrimSpace(item.Type) != "text" {
			continue
		}
		if out.Len() > 0 {
			out.WriteString("\n")
		}
		out.WriteString(strings.TrimSpace(item.Text))
	}
	content := strings.TrimSpace(out.String())
	if content == "" {
		return "", errors.New("llm returned empty content")
	}
	return content, nil
}
