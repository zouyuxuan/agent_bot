package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/x402pay"
)

// Minimal buyer-side proxy so the "robot" can make paid HTTP requests via x402.
// This is intentionally generic; higher-level "autonomous trading" logic should be
// implemented as skills/tools on top.
func (s *Server) handleX402Proxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Optional: require wallet auth for *using* the robot, even though payment signing
	// is server-side (prevents open proxy abuse).
	if _, ok := s.requireWallet(w, r); !ok {
		return
	}

	var input struct {
		URL     string            `json:"url"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		Body    any               `json:"body"`
		TimeoutMS int             `json:"timeoutMs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u := strings.TrimSpace(input.URL)
	if u == "" || (!strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://")) {
		writeError(w, http.StatusBadRequest, "url must be http(s)")
		return
	}
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if input.Body != nil && method != http.MethodGet && method != http.MethodHead {
		raw, err := json.Marshal(input.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		bodyReader = bytes.NewReader(raw)
	}

	ctx := r.Context()
	if input.TimeoutMS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(input.TimeoutMS)*time.Millisecond)
		defer cancel()
	} else {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to build request")
		return
	}
	for k, v := range input.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	if input.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client, err := x402pay.NewFromEnv()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	resp, err := client.DoContext(ctx, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	// Limit response size to avoid memory blowups.
	const maxResp = 1 << 20 // 1MB
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxResp))

	// Only pass through a small set of headers plus x402 settle header.
	outHeaders := map[string]string{}
	if v := resp.Header.Get("Content-Type"); v != "" {
		outHeaders["content-type"] = v
	}
	if v := resp.Header.Get("PAYMENT-RESPONSE"); v != "" {
		outHeaders["payment-response"] = v
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  resp.StatusCode,
		"headers": outHeaders,
		"body":    string(b),
	})
}

