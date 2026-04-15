package service

import (
	"encoding/json"
	"strings"

	"ai-bot-chain/backend/internal/domain"
)

func looksLikeX402SkillContent(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" || !strings.HasPrefix(content, "{") {
		return false
	}
	var v struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		return false
	}
	t := strings.ToLower(strings.TrimSpace(v.Type))
	return t == "x402_fetch" || t == "x402"
}

// buildX402ContextFromFrontend converts frontend-executed x402 tool results into a prompt-friendly block.
// Keep it small to avoid blowing up tokens.
func buildX402ContextFromFrontend(results []domain.X402ToolResult) string {
	if len(results) == 0 {
		return ""
	}
	if len(results) > 3 {
		results = results[:3]
	}
	var b strings.Builder
	for _, r := range results {
		title := strings.TrimSpace(r.Filename)
		if title == "" {
			title = strings.TrimSpace(r.SkillID)
		}
		if title == "" {
			title = "x402"
		}
		b.WriteString("### ")
		b.WriteString(title)
		b.WriteString("\n")
		if strings.TrimSpace(r.Method) != "" {
			b.WriteString("method: ")
			b.WriteString(strings.TrimSpace(r.Method))
			b.WriteString("\n")
		}
		if strings.TrimSpace(r.URL) != "" {
			b.WriteString("url: ")
			b.WriteString(strings.TrimSpace(r.URL))
			b.WriteString("\n")
		}
		if r.Status != 0 {
			b.WriteString("httpStatus: ")
			b.WriteString(strings.TrimSpace(intToString(r.Status)))
			b.WriteString("\n")
		}
		if r.Error != "" && !r.OK {
			b.WriteString("error: ")
			b.WriteString(trunc(strings.TrimSpace(r.Error), 400))
			b.WriteString("\n")
		}
		if r.Headers != nil {
			if v := strings.TrimSpace(r.Headers["payment-response"]); v != "" {
				b.WriteString("paymentResponse: ")
				b.WriteString(trunc(v, 512))
				b.WriteString("\n")
			}
		}
		body := strings.TrimSpace(r.Body)
		if body == "" {
			body = "(empty)"
		}
		b.WriteString("body:\n")
		b.WriteString(trunc(body, 6000))
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

func intToString(v int) string {
	// small helper to avoid fmt import for a single field
	// (keeps the service package a bit lean).
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	var buf [16]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
