package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/x402pay"
)

// x402SkillSpec is a minimal skill schema that describes a paid HTTP request.
//
// Skill content example (JSON):
// {
//   "type": "x402_fetch",
//   "url": "https://api.example.com/search?q={input}",
//   "method": "GET",
//   "headers": { "accept": "application/json" }
// }
type x402SkillSpec struct {
	SkillID string
	Name    string
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func parseX402SkillSpec(skillID, skillName, raw string) (x402SkillSpec, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return x402SkillSpec{}, false
	}
	var tmp struct {
		Type string `json:"type"`
		x402SkillSpec
	}
	if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
		return x402SkillSpec{}, false
	}
	t := strings.ToLower(strings.TrimSpace(tmp.Type))
	if t != "x402_fetch" && t != "x402" {
		return x402SkillSpec{}, false
	}
	spec := tmp.x402SkillSpec
	spec.SkillID = strings.TrimSpace(skillID)
	spec.Name = strings.TrimSpace(skillName)
	spec.Method = strings.ToUpper(strings.TrimSpace(spec.Method))
	if spec.Method == "" {
		spec.Method = http.MethodGet
	}
	spec.URL = strings.TrimSpace(spec.URL)
	if spec.URL == "" {
		return x402SkillSpec{}, false
	}
	return spec, true
}

func (s *ChatService) runX402Skills(ctx context.Context, specs []x402SkillSpec, userMessage string) (string, error) {
	if len(specs) == 0 {
		return "", nil
	}
	// Hard cap to avoid accidental draining.
	if len(specs) > 3 {
		return "", errors.New("too many x402 skills enabled (max 3)")
	}

	client, err := x402pay.NewFromEnv()
	if err != nil {
		return "", err
	}

	var out strings.Builder
	for _, spec := range specs {
		line, err := s.runOneX402Skill(ctx, client, spec, userMessage)
		if err != nil {
			return "", err
		}
		if line == "" {
			continue
		}
		out.WriteString(line)
		out.WriteString("\n\n")
	}
	return strings.TrimSpace(out.String()), nil
}

func (s *ChatService) runOneX402Skill(ctx context.Context, client *x402pay.Client, spec x402SkillSpec, userMessage string) (string, error) {
	u := strings.ReplaceAll(spec.URL, "{input}", url.QueryEscape(userMessage))
	parsed, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("x402 skill %s: invalid url", spec.SkillID)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("x402 skill %s: url must be http(s)", spec.SkillID)
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0" {
		return "", fmt.Errorf("x402 skill %s: localhost is not allowed", spec.SkillID)
	}

	req, err := http.NewRequestWithContext(ctx, spec.Method, u, nil)
	if err != nil {
		return "", fmt.Errorf("x402 skill %s: failed to build request", spec.SkillID)
	}
	for k, v := range spec.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}

	// Give x402 requests a bit more time; 402 challenge + retry can take a while.
	ctx2, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := client.DoContext(ctx2, req)
	if err != nil {
		return "", fmt.Errorf("x402 skill %s: %v", spec.SkillID, err)
	}
	defer resp.Body.Close()

	const maxBody = 64 << 10
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	body := strings.TrimSpace(string(bytes.TrimSpace(b)))

	title := spec.Name
	if title == "" {
		title = spec.SkillID
	}
	if resp.StatusCode >= 400 {
		if body == "" {
			body = resp.Status
		}
		return "", fmt.Errorf("x402 skill %s (%s) failed: http %d: %s", spec.SkillID, title, resp.StatusCode, body)
	}
	if body == "" {
		body = "(empty response)"
	}

	// Include PAYMENT-RESPONSE as a debugging aid (seller may encode receipt/metadata).
	payResp := strings.TrimSpace(resp.Header.Get("PAYMENT-RESPONSE"))
	if payResp != "" {
		payResp = truncateText(payResp, 512)
	}

	var out strings.Builder
	out.WriteString("### ")
	out.WriteString(title)
	out.WriteString("\n")
	out.WriteString("url: ")
	out.WriteString(u)
	out.WriteString("\n")
	out.WriteString("httpStatus: ")
	out.WriteString(fmt.Sprintf("%d", resp.StatusCode))
	out.WriteString("\n")
	if payResp != "" {
		out.WriteString("paymentResponse: ")
		out.WriteString(payResp)
		out.WriteString("\n")
	}
	out.WriteString("body:\n")
	out.WriteString(truncateText(body, 6000))
	return out.String(), nil
}

func truncateText(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

