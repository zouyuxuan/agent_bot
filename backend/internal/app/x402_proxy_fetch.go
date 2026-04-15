package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// handleX402ProxyFetch is a generic server-side HTTP forwarder used by the browser-side x402 SDK
// to bypass CORS restrictions.
//
// Security: This endpoint requires wallet authorization and includes SSRF defenses.
func (s *Server) handleX402ProxyFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := s.requireWallet(w, r); !ok {
		return
	}

	var input struct {
		URL       string            `json:"url"`
		Method    string            `json:"method"`
		Headers   map[string]string `json:"headers"`
		Body      string            `json:"body"`
		TimeoutMS int               `json:"timeoutMs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	target := strings.TrimSpace(input.URL)
	if target == "" {
		writeError(w, http.StatusBadRequest, "missing url")
		return
	}
	parsed, err := url.Parse(target)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid url")
		return
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		writeError(w, http.StatusBadRequest, "url must be http(s)")
		return
	}

	if err := validatePublicURL(r.Context(), parsed); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if input.Body != "" && method != http.MethodGet && method != http.MethodHead {
		bodyReader = strings.NewReader(input.Body)
	}

	ctx := r.Context()
	timeout := 45 * time.Second
	if input.TimeoutMS > 0 {
		timeout = time.Duration(input.TimeoutMS) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to build request")
		return
	}

	// Copy headers, but block hop-by-hop / dangerous ones.
	for k, v := range input.Headers {
		key := http.CanonicalHeaderKey(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		if isBlockedProxyHeader(key) {
			continue
		}
		req.Header.Set(key, v)
	}

	// If frontend provided a body but no content-type, default to json.
	if input.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute. (We intentionally do not follow redirects to avoid SSRF bypass.)
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("redirects are not allowed")
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	const maxResp = 1 << 20 // 1MB
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxResp))

	outHeaders := map[string]string{}
	// x402 headers are important for the browser-side SDK. Some implementations
	// may add extra metadata headers, so forward all PAYMENT-* and X-PAYMENT-* headers.
	for k, vals := range resp.Header {
		uk := strings.ToUpper(strings.TrimSpace(k))
		if uk == "" {
			continue
		}
		if uk == "SET-COOKIE" {
			continue
		}
		if uk == "CONTENT-TYPE" || uk == "CACHE-CONTROL" || strings.HasPrefix(uk, "PAYMENT-") || strings.HasPrefix(uk, "X-PAYMENT-") {
			// PAYMENT-* headers are base64 values and must not be comma-joined. If an upstream
			// server sends multiple header lines, concatenate them to avoid corrupting base64.
			joined := ""
			if len(vals) == 1 {
				joined = vals[0]
			} else if len(vals) > 1 {
				joined = strings.Join(vals, "")
			}
			joined = strings.TrimSpace(joined)
			if joined == "" {
				continue
			}
			outHeaders[strings.ToLower(k)] = joined
		}
	}

	// Return as JSON; the browser-side proxy fetch will re-wrap this into a Response.
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     resp.StatusCode,
		"statusText": resp.Status,
		"headers":    outHeaders,
		"body":       string(raw),
	})
}

func copyHeader(dst map[string]string, src http.Header, key string) {
	if dst == nil || src == nil {
		return
	}
	v := strings.TrimSpace(src.Get(key))
	if v == "" {
		return
	}
	dst[strings.ToLower(key)] = v
}

func isBlockedProxyHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "host", "connection", "proxy-connection", "keep-alive", "transfer-encoding", "upgrade", "te", "trailer":
		return true
	case "x-forwarded-for", "x-forwarded-host", "x-forwarded-proto":
		return true
	}
	return false
}

func validatePublicURL(ctx context.Context, u *url.URL) error {
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return errors.New("invalid host")
	}
	low := strings.ToLower(host)
	if low == "localhost" || low == "0.0.0.0" || low == "127.0.0.1" || low == "::1" {
		return errors.New("localhost is not allowed")
	}

	// If it's an IP literal, validate directly.
	if addr, err := netip.ParseAddr(host); err == nil {
		if !isPublicIP(addr) {
			return errors.New("private ip is not allowed")
		}
		return nil
	}

	// Resolve and block private ranges (best-effort).
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		// If we can't resolve, let the request fail later; don't leak DNS details.
		return errors.New("failed to resolve host")
	}
	for _, ip := range ips {
		if a, ok := netip.AddrFromSlice(ip); ok {
			if !isPublicIP(a) {
				return errors.New("host resolves to private ip (not allowed)")
			}
		}
	}
	return nil
}

func isPublicIP(a netip.Addr) bool {
	if !a.IsValid() {
		return false
	}
	if a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() {
		return false
	}
	if a.IsMulticast() {
		return false
	}
	// CGNAT 100.64.0.0/10
	if a.Is4() {
		v4 := a.As4()
		if v4[0] == 100 && v4[1]&0b11000000 == 0b01000000 {
			return false
		}
		// 169.254.0.0/16
		if v4[0] == 169 && v4[1] == 254 {
			return false
		}
	}
	return true
}
