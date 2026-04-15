package zerog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	zg_common "github.com/0gfoundation/0g-storage-client/common"
	"github.com/0gfoundation/0g-storage-client/common/blockchain"
	"github.com/0gfoundation/0g-storage-client/core"
	"github.com/0gfoundation/0g-storage-client/indexer"
	"github.com/0gfoundation/0g-storage-client/node"
	"github.com/0gfoundation/0g-storage-client/transfer"
	eth_common "github.com/ethereum/go-ethereum/common"
	providers "github.com/openweb3/go-rpc-provider/provider_wrapper"
	"github.com/openweb3/web3go"
	"github.com/sirupsen/logrus"
)

// Client uploads training payload to 0G Storage if credentials are configured.
// Otherwise, it falls back to a local simulated reference so the project can run end-to-end.
type Client struct {
	mode string

	// sim mode
	simEndpoint string

	// sdk mode
	privateKey string
	evmRPC     string
	indexerRPC string
	explorer   string
	method     string
	replicas   uint
	zgsNodes   []string
	rpcTimeout time.Duration
	probeTO    time.Duration
}

func NewClientFromEnv() *Client {
	privateKey := strings.TrimSpace(os.Getenv("ZERO_G_PRIVATE_KEY"))
	if privateKey == "" {
		return &Client{
			mode:        "sim",
			simEndpoint: strings.TrimSpace(os.Getenv("ZERO_G_ENDPOINT")),
		}
	}

	return &Client{
		mode:       "sdk",
		privateKey: privateKey,
		evmRPC:     envOr("ZERO_G_EVM_RPC", "https://evmrpc-testnet.0g.ai"),
		indexerRPC: envOr("ZERO_G_INDEXER_RPC", "https://indexer-storage-testnet-standard.0g.ai"),
		explorer:   envOr("ZERO_G_EXPLORER_BASE", "https://chainscan-galileo.0g.ai"),
		method:     envOr("ZERO_G_UPLOAD_METHOD", "min"),
		replicas:   envUintOr("ZERO_G_REPLICAS", 1),
		zgsNodes:   envCSV("ZERO_G_ZGS_NODES"),
		rpcTimeout: envDurationMSOr("ZERO_G_RPC_TIMEOUT_MS", 30000),
		probeTO:    envDurationMSOr("ZERO_G_ZGS_PROBE_TIMEOUT_MS", 3500),
	}
}

func (c *Client) StoreTrainingPayload(ctx context.Context, payload []byte, zgsNodesOverride []string) (PublishInfo, error) {
	switch c.mode {
	case "sdk":
		return c.storeViaSDK(ctx, payload, zgsNodesOverride)
	default:
		ref := c.storeViaSim(payload)
		return PublishInfo{
			Mode:      "sim",
			Reference: ref,
		}, nil
	}
}

func (c *Client) storeViaSim(payload []byte) string {
	sum := sha256.Sum256(payload)
	ref := hex.EncodeToString(sum[:12])
	if strings.TrimSpace(c.simEndpoint) == "" {
		return fmt.Sprintf("0g://local-sim/%s", ref)
	}
	return fmt.Sprintf("%s/%s", strings.TrimRight(c.simEndpoint, "/"), ref)
}

func (c *Client) storeViaSDK(ctx context.Context, payload []byte, zgsNodesOverride []string) (PublishInfo, error) {
	w3client := blockchain.MustNewWeb3(c.evmRPC, c.privateKey)
	defer w3client.Close()

	data, err := core.NewDataInMemory(payload)
	if err != nil {
		return PublishInfo{}, err
	}

	tree, err := core.MerkleTree(data)
	if err != nil {
		return PublishInfo{}, err
	}
	root := tree.Root()

	override := normalizeURLs(zgsNodesOverride)
	nodes := c.zgsNodes
	if len(override) > 0 {
		nodes = override
	}

	// Prefer direct upload when ZGS nodes are provided to avoid indexer dependency.
	// This is the most reliable path when the public indexer endpoint is returning 503.
	if len(nodes) > 0 {
		if info, err := c.uploadDirect(ctx, w3client, data, root, nodes); err == nil {
			return info, nil
		}
		// If direct upload fails, fall back to indexer-based upload (may still work for some users).
	}

	// Indexer-based upload. When indexer is down (503), surface HTTP 503 to the caller.
	info, err := c.tryUploadViaIndexer(ctx, w3client, data, root)
	if err != nil {
		if isTemporaryIndexerFailure(err) {
			return PublishInfo{}, &ErrIndexerUnavailable{IndexerRPC: c.indexerRPC, Cause: err}
		}
		return PublishInfo{}, err
	}
	return info, nil
}

func (c *Client) tryUploadViaIndexer(ctx context.Context, w3client *web3go.Client, data core.IterableData, rootHash eth_common.Hash) (PublishInfo, error) {
	// Default go-rpc-provider retry count is 0, which turns a transient 503 into
	// an immediate failure ("failed after 0 retries"). Configure retries here.
	opt := providers.Option{}
	opt.RequestTimeout = c.rpcTimeout
	opt.RetryCount = envIntOr("ZERO_G_INDEXER_RETRY_COUNT", 2)
	opt.RetryInterval = envDurationMSOr("ZERO_G_INDEXER_RETRY_INTERVAL_MS", 750)

	indexerClient, err := indexer.NewClient(c.indexerRPC, indexer.IndexerClientOption{
		ProviderOption: opt,
		LogOption:      zg_common.LogOption{Logger: logrus.StandardLogger()},
		FullTrusted:    true,
	})
	if err != nil {
		return PublishInfo{}, err
	}

	// Use splitable upload to match the latest SDK; for small payloads this returns a single tx/root.
	txHashes, roots, err := indexerClient.SplitableUpload(ctx, w3client, data, 4*1024*1024*1024, transfer.UploadOption{
		FinalityRequired: transfer.FileFinalized,
		Method:           c.method,
		FullTrusted:      true,
		ExpectedReplica:  c.replicas,
		NRetries:         3,
	})
	if err != nil {
		return PublishInfo{}, err
	}

	txHash := eth_common.Hash{}
	if len(txHashes) > 0 {
		txHash = txHashes[0]
	}
	// Root may differ for split uploads; prefer the SDK returned root when present.
	if len(roots) > 0 && roots[0] != (eth_common.Hash{}) {
		rootHash = roots[0]
	}

	locations := make([]string, 0)
	if locs, err := indexerClient.GetFileLocations(ctx, rootHash.Hex()); err == nil {
		for _, l := range locs {
			if strings.TrimSpace(l.URL) != "" {
				locations = append(locations, l.URL)
			}
		}
	}

	explorer := strings.TrimRight(strings.TrimSpace(c.explorer), "/")
	explorerTx := ""
	if explorer != "" && txHash.Hex() != "0x0000000000000000000000000000000000000000000000000000000000000000" {
		explorerTx = fmt.Sprintf("%s/tx/%s", explorer, txHash.Hex())
	}

	ref := fmt.Sprintf("0g://storage/root/%s?tx=%s&ts=%d", rootHash.Hex(), txHash.Hex(), time.Now().Unix())
	return PublishInfo{
		Mode:          "sdk-indexer",
		Reference:     ref,
		TxHash:        txHash.Hex(),
		RootHash:      rootHash.Hex(),
		ExplorerTxURL: explorerTx,
		IndexerRPC:    c.indexerRPC,
		EvmRPC:        c.evmRPC,
		FileLocations: locations,
	}, nil
}

func (c *Client) uploadDirect(ctx context.Context, w3client *web3go.Client, data core.IterableData, rootHash eth_common.Hash, nodes []string) (PublishInfo, error) {
	zgsClients, reachable, unreachable := c.buildReachableZgsClients(ctx, nodes)
	for _, client := range zgsClients {
		defer client.Close()
	}
	if len(zgsClients) == 0 {
		return PublishInfo{}, fmt.Errorf("no reachable ZGS nodes (timeout). reachable=%v unreachable=%v", reachable, unreachable)
	}

	uploader, err := transfer.NewUploaderWithContractConfig(ctx, w3client, &transfer.SelectedNodes{Trusted: zgsClients}, transfer.UploaderConfig{
		LogOption: zg_common.LogOption{Logger: logrus.StandardLogger()},
	})
	if err != nil {
		return PublishInfo{}, err
	}

	txHash, _, err := uploader.Upload(ctx, data, transfer.UploadOption{
		FinalityRequired: transfer.FileFinalized,
		ExpectedReplica:  c.replicas,
		Method:           c.method,
		FullTrusted:      true,
		NRetries:         2,
	})
	if err != nil {
		return PublishInfo{}, err
	}

	explorer := strings.TrimRight(strings.TrimSpace(c.explorer), "/")
	explorerTx := ""
	if explorer != "" && txHash.Hex() != "0x0000000000000000000000000000000000000000000000000000000000000000" {
		explorerTx = fmt.Sprintf("%s/tx/%s", explorer, txHash.Hex())
	}

	ref := fmt.Sprintf("0g://storage/root/%s?tx=%s&ts=%d", rootHash.Hex(), txHash.Hex(), time.Now().Unix())
	return PublishInfo{
		Mode:          "sdk-direct",
		Reference:     ref,
		TxHash:        txHash.Hex(),
		RootHash:      rootHash.Hex(),
		ExplorerTxURL: explorerTx,
		IndexerRPC:    c.indexerRPC,
		EvmRPC:        c.evmRPC,
		FileLocations: append([]string{}, reachable...),
	}, nil
}

func (c *Client) buildReachableZgsClients(ctx context.Context, nodes []string) ([]*node.ZgsClient, []string, []string) {
	// Use a shorter probe timeout to avoid long hangs when a node is down/unreachable.
	probeCtx, cancel := context.WithTimeout(ctx, c.probeTO)
	defer cancel()

	opt := providers.Option{}
	opt.RequestTimeout = c.rpcTimeout
	opt.RetryCount = 0

	reachable := make([]string, 0, len(nodes))
	unreachable := make([]string, 0)
	clients := make([]*node.ZgsClient, 0)

	for _, raw := range nodes {
		cands := candidateURLs(raw)
		var picked *node.ZgsClient
		var pickedURL string
		for _, url := range cands {
			client, err := node.NewZgsClient(url, nil, opt)
			if err != nil {
				continue
			}

			// Minimal probe: zgs_getStatus.
			if _, err := client.GetStatus(probeCtx); err != nil {
				client.Close()
				continue
			}

			picked = client
			pickedURL = url
			break
		}

		if picked == nil {
			if len(cands) > 0 {
				unreachable = append(unreachable, cands[0])
			} else {
				unreachable = append(unreachable, strings.TrimSpace(raw))
			}
			continue
		}

		reachable = append(reachable, pickedURL)
		clients = append(clients, picked)
	}

	return clients, reachable, unreachable
}

func isTemporaryIndexerFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	if strings.Contains(msg, "503") || strings.Contains(low, "service temporarily unavailable") {
		return true
	}
	// Common pattern from go-rpc-provider: "failed after 0 retries: 503 ..."
	if strings.Contains(low, "failed after") && strings.Contains(low, "retries") && strings.Contains(low, "503") {
		return true
	}
	return false
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envUintOr(key string, fallback uint) uint {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return fallback
	}
	if n == 0 {
		return fallback
	}
	return uint(n)
}

func envIntOr(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envCSV(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envDurationMSOr(key string, fallbackMS int) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return time.Duration(fallbackMS) * time.Millisecond
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return time.Duration(fallbackMS) * time.Millisecond
	}
	return time.Duration(n) * time.Millisecond
}

func normalizeURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	// Many ZGS URLs are given as host:port. Assume http in that case.
	return "http://" + s
}

func candidateURLs(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		// Try https first; if cert mismatch it will fail quickly and fall back to http.
		return []string{"https://" + s, "http://" + s}
	}
	if strings.HasPrefix(s, "http://") {
		return []string{s, "https://" + strings.TrimPrefix(s, "http://")}
	}
	return []string{s, "http://" + strings.TrimPrefix(s, "https://")}
}

func normalizeURLs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, normalizeURL(v))
	}
	return out
}
