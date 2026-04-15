package zerog

import "fmt"

// ErrNeedZgsNodes indicates indexer is unavailable and direct upload nodes are required.
// The handler maps this to HTTP 503 so the frontend can guide the user.
type ErrNeedZgsNodes struct {
	IndexerRPC string
}

func (e *ErrNeedZgsNodes) Error() string {
	if e.IndexerRPC == "" {
		return "0G indexer is unavailable (503). Provide ZGS nodes to bypass indexer"
	}
	return fmt.Sprintf("0G indexer is unavailable (503): %s. Provide ZGS nodes to bypass indexer", e.IndexerRPC)
}

// ErrIndexerUnavailable indicates the indexer RPC itself is down/unreachable (typically HTTP 503).
// In "indexer required" mode, there is no bypass; callers should surface this as HTTP 503.
type ErrIndexerUnavailable struct {
	IndexerRPC string
	Cause      error
}

func (e *ErrIndexerUnavailable) Error() string {
	if e == nil {
		return "0G indexer RPC is unavailable (503)"
	}
	if e.IndexerRPC == "" {
		if e.Cause != nil {
			return fmt.Sprintf("0G indexer RPC is unavailable (503): %v", e.Cause)
		}
		return "0G indexer RPC is unavailable (503)"
	}
	if e.Cause != nil {
		return fmt.Sprintf("0G indexer RPC is unavailable (503): %s (%v)", e.IndexerRPC, e.Cause)
	}
	return fmt.Sprintf("0G indexer RPC is unavailable (503): %s", e.IndexerRPC)
}

func (e *ErrIndexerUnavailable) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
