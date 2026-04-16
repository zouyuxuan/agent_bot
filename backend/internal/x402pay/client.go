package x402pay

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	x402 "github.com/x402-foundation/x402/go"
	x402http "github.com/x402-foundation/x402/go/http"
	evmexact "github.com/x402-foundation/x402/go/mechanisms/evm/exact/client"
	evmsigners "github.com/x402-foundation/x402/go/signers/evm"
)

// Client wraps a standard http.Client so requests automatically handle 402 challenges
// using the x402 protocol (buyer-side).
//
// NOTE: This uses a server-side EVM private key for signing payment payloads.
// If you want user-authorized payments via MetaMask, the buyer-side flow should
// run in the browser instead.
type Client struct {
	http *http.Client
}

var (
	once   sync.Once
	shared *Client
	initErr error
)

func NewFromEnv() (*Client, error) {
	once.Do(func() {
		pk := strings.TrimSpace(os.Getenv("X402_EVM_PRIVATE_KEY"))
		if pk == "" {
			initErr = errors.New("missing X402_EVM_PRIVATE_KEY (required for x402 buyer payments)")
			return
		}
		signer, err := evmsigners.NewClientSignerFromPrivateKey(pk)
		if err != nil {
			initErr = err
			return
		}

		// Create x402 client and register EVM exact scheme for any eip155 chain.
		c := x402.Newx402Client().
			Register("eip155:*", evmexact.NewExactEvmScheme(signer, nil))

		wrapped := x402http.WrapHTTPClientWithPayment(
			http.DefaultClient,
			x402http.Newx402HTTPClient(c),
		)
		shared = &Client{http: wrapped}
	})
	return shared, initErr
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c == nil || c.http == nil {
		return nil, errors.New("x402 client not initialized")
	}
	return c.http.Do(req)
}

func (c *Client) DoContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}
	return c.Do(req.WithContext(ctx))
}

func (c *Client) DoContextNoRedirect(ctx context.Context, req *http.Request, timeout time.Duration) (*http.Response, error) {
	if c == nil || c.http == nil {
		return nil, errors.New("x402 client not initialized")
	}
	if req == nil {
		return nil, errors.New("nil request")
	}
	cloned := *c.http
	if timeout > 0 {
		cloned.Timeout = timeout
	}
	cloned.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return cloned.Do(req.WithContext(ctx))
}
