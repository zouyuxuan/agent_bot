package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"
	"ai-bot-chain/backend/internal/zerog"

	zg_common "github.com/0gfoundation/0g-storage-client/common"
	"github.com/0gfoundation/0g-storage-client/contract"
	"github.com/0gfoundation/0g-storage-client/core"
	"github.com/0gfoundation/0g-storage-client/core/merkle"
	"github.com/0gfoundation/0g-storage-client/indexer"
	"github.com/0gfoundation/0g-storage-client/node"
	"github.com/0gfoundation/0g-storage-client/transfer"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	providers "github.com/openweb3/go-rpc-provider/provider_wrapper"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type WalletTxRequest struct {
	PublishID  string `json:"publishId"`
	ChainIDHex string `json:"chainIdHex"`
	ChainIDDec string `json:"chainIdDec"`
	From       string `json:"from"`
	To         string `json:"to"`
	Data       string `json:"data"`
	Value      string `json:"value"`
	RootHash   string `json:"rootHash"`
}

func (s *ChatService) PrepareWalletPublish(ctx context.Context, botID string, walletAddress string, zgsNodes []string) (WalletTxRequest, error) {
	samples, err := s.store.ListTrainingSamples(botID)
	if err != nil {
		return WalletTxRequest{}, err
	}
	if len(samples) == 0 {
		return WalletTxRequest{}, errors.New("no training samples to publish")
	}

	exportedAt := time.Now().UTC().Format(time.RFC3339Nano)
	type exportPayload struct {
		BotID         string                  `json:"botId"`
		ExportedAt    string                  `json:"exportedAt"`
		TrainingSet   []domain.TrainingSample `json:"trainingSet"`
		SampleCount   int                     `json:"sampleCount"`
		StorageTarget string                  `json:"storageTarget"`
	}
	payload, err := json.Marshal(exportPayload{
		BotID:         botID,
		ExportedAt:    exportedAt,
		TrainingSet:   samples,
		SampleCount:   len(samples),
		StorageTarget: "0G",
	})
	if err != nil {
		return WalletTxRequest{}, err
	}

	flowAddr := common.Address{}
	// Fast path: allow configuring Flow contract address to avoid any ZGS node dependency in "prepare".
	if v := strings.TrimSpace(os.Getenv("ZERO_G_FLOW_CONTRACT_ADDRESS")); v != "" {
		if common.IsHexAddress(v) {
			flowAddr = common.HexToAddress(v)
		} else {
			return WalletTxRequest{}, errors.New("invalid ZERO_G_FLOW_CONTRACT_ADDRESS")
		}
	}
	if flowAddr == (common.Address{}) {
		// Need at least one reachable ZGS node to discover Flow contract address.
		// If caller didn't provide nodes, discover via indexer (indexer-required flow).
		if len(zgsNodes) == 0 {
			nodes, err := discoverZgsNodesViaIndexer(ctx)
			if err != nil {
				return WalletTxRequest{}, err
			}
			zgsNodes = nodes
		}
		zgsClient, reachable, unreachable, err := firstReachableZgs(ctx, zgsNodes)
		if err != nil {
			return WalletTxRequest{}, pkgerrors.WithMessagef(err, "no reachable ZGS nodes (reachable=%v unreachable=%v)", reachable, unreachable)
		}
		defer zgsClient.Close()

		status, err := zgsClient.GetStatus(ctx)
		if err != nil {
			return WalletTxRequest{}, err
		}
		flowAddr = status.NetworkIdentity.FlowContractAddress
	}

	evmRPC := strings.TrimSpace(os.Getenv("ZERO_G_EVM_RPC"))
	if evmRPC == "" {
		evmRPC = "https://evmrpc-testnet.0g.ai"
	}
	ec, err := ethclient.DialContext(ctx, evmRPC)
	if err != nil {
		return WalletTxRequest{}, err
	}
	defer ec.Close()

	chainID, err := ec.ChainID(ctx)
	if err != nil {
		return WalletTxRequest{}, err
	}

	// Validate flow contract code early; wrong address causes a confusing "no contract code at given address".
	if flowAddr != (common.Address{}) {
		code, err := ec.CodeAt(ctx, flowAddr, nil)
		if err != nil {
			return WalletTxRequest{}, err
		}
		if len(code) == 0 {
			return WalletTxRequest{}, errors.New("no contract code at given address (check ZERO_G_FLOW_CONTRACT_ADDRESS and ZERO_G_EVM_RPC chainId=" + chainID.String() + ", addr=" + flowAddr.Hex() + ")")
		}
	}

	dataObj, err := core.NewDataInMemory(payload)
	if err != nil {
		return WalletTxRequest{}, err
	}
	tree, err := core.MerkleTree(dataObj)
	if err != nil {
		return WalletTxRequest{}, err
	}

	flow := core.NewFlow(dataObj, nil)
	submission, err := flow.CreateSubmission(common.HexToAddress(walletAddress))
	if err != nil {
		return WalletTxRequest{}, err
	}

	flowCaller, err := contract.NewFlow(flowAddr, ec)
	if err != nil {
		return WalletTxRequest{}, err
	}
	marketAddr, err := flowCaller.Market(nil)
	if err != nil {
		return WalletTxRequest{}, err
	}
	market, err := contract.NewMarket(marketAddr, ec)
	if err != nil {
		return WalletTxRequest{}, err
	}
	pricePerSector, err := market.PricePerSector(nil)
	if err != nil {
		return WalletTxRequest{}, err
	}
	fee := submission.Fee(pricePerSector)

	flowABI, err := abi.JSON(strings.NewReader(contract.FlowMetaData.ABI))
	if err != nil {
		return WalletTxRequest{}, err
	}
	calldata, err := flowABI.Pack("submit", *submission)
	if err != nil {
		return WalletTxRequest{}, err
	}

	publishID, err := newSnapshotID()
	if err != nil {
		return WalletTxRequest{}, err
	}
	sampleIDs := make([]string, 0, len(samples))
	for _, sm := range samples {
		if strings.TrimSpace(sm.ID) != "" {
			sampleIDs = append(sampleIDs, sm.ID)
		}
	}
	s.putSnapshot(walletPublishSnapshot{
		ID:        publishID,
		BotID:     botID,
		Wallet:    walletAddress,
		Kind:      "training",
		Root:      tree.Root(),
		Payload:   payload,
		SampleIDs: sampleIDs,
		CreatedAt: time.Now(),
	})

	return WalletTxRequest{
		PublishID:  publishID,
		ChainIDHex: "0x" + chainID.Text(16),
		ChainIDDec: chainID.String(),
		From:       walletAddress,
		To:         flowAddr.Hex(),
		Data:       "0x" + common.Bytes2Hex(calldata),
		Value:      "0x" + fee.Text(16),
		RootHash:   tree.Root().Hex(),
	}, nil
}

func (s *ChatService) FinalizeWalletPublish(ctx context.Context, botID string, publishID string, txHash string, rootHash string, zgsNodes []string) (domain.PublishResult, error) {
	publishID = strings.TrimSpace(publishID)
	if publishID == "" {
		return domain.PublishResult{}, errors.New("missing publishId (please prepare again)")
	}

	if strings.TrimSpace(rootHash) == "" {
		return domain.PublishResult{}, errors.New("missing rootHash")
	}
	root := common.HexToHash(rootHash)
	if root == (common.Hash{}) {
		return domain.PublishResult{}, errors.New("invalid rootHash")
	}

	ss, err := s.getSnapshot(publishID, 15*time.Minute)
	if err != nil {
		return domain.PublishResult{}, err
	}
	if ss.BotID != botID {
		return domain.PublishResult{}, errors.New("publishId does not match botId (please prepare again)")
	}
	if ss.Kind != "training" {
		return domain.PublishResult{}, errors.New("publishId is not a training publish (please prepare again)")
	}
	if ss.Root != root {
		return domain.PublishResult{}, errors.New("rootHash mismatch (payload changed; please prepare+send tx again)")
	}
	payload := append([]byte(nil), ss.Payload...)
	sampleIDs := append([]string(nil), ss.SampleIDs...)

	dataObj, err := core.NewDataInMemory(payload)
	if err != nil {
		return domain.PublishResult{}, err
	}
	tree, err := core.MerkleTree(dataObj)
	if err != nil {
		return domain.PublishResult{}, err
	}
	if tree.Root() != root {
		return domain.PublishResult{}, errors.New("rootHash mismatch (payload changed; please prepare+send tx again)")
	}

	// Confirm on-chain transaction success quickly. If it's already successful in the explorer,
	// return immediately and upload segments asynchronously to avoid UI timeouts.
	mined, success, err := checkTxReceipt(ctx, strings.TrimSpace(txHash))
	if err != nil {
		return domain.PublishResult{}, err
	}
	if mined && !success {
		return domain.PublishResult{}, errors.New("transaction failed on-chain")
	}

	// Snapshot isn't needed after this point; async uploader has its own copy.
	s.deleteSnapshot(publishID)

	// Start async upload to storage nodes (can take minutes depending on node sync).
	go s.completeWalletTrainingUpload(botID, strings.TrimSpace(txHash), root, payload, sampleIDs, zgsNodes)

	explorer := strings.TrimRight(strings.TrimSpace(os.Getenv("ZERO_G_EXPLORER_BASE")), "/")
	explorerTx := ""
	if explorer == "" {
		explorer = "https://chainscan-galileo.0g.ai"
	}
	if explorer != "" && strings.TrimSpace(txHash) != "" {
		explorerTx = explorer + "/tx/" + strings.TrimSpace(txHash)
	}
	ref := "0g://storage/root/" + root.Hex() + "?tx=" + strings.TrimSpace(txHash)

	out := domain.PublishResult{
		BotID:            botID,
		SampleCount:      len(sampleIDs),
		StorageReference: ref,
		Mode:             "wallet",
		TxHash:           strings.TrimSpace(txHash),
		RootHash:         root.Hex(),
		ExplorerTxURL:    explorerTx,
		TxMined:          mined,
		TxSuccess:        success,
		UploadPending:    true,
		UploadCompleted:  false,
		PublishedAt:      time.Now(),
	}

	if err := s.pubs.SaveTrainingPublish(ctx, out); err != nil {
		return domain.PublishResult{}, err
	}

	return out, nil
}

func checkTxReceipt(ctx context.Context, txHash string) (mined bool, success bool, err error) {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return false, false, errors.New("missing txHash")
	}
	evmRPC := strings.TrimSpace(os.Getenv("ZERO_G_EVM_RPC"))
	if evmRPC == "" {
		evmRPC = "https://evmrpc-testnet.0g.ai"
	}
	ec, err := ethclient.DialContext(ctx, evmRPC)
	if err != nil {
		return false, false, err
	}
	defer ec.Close()

	h := common.HexToHash(txHash)
	// Wait briefly for receipt (most of the time user clicks finalize after seeing it mined).
	deadline := time.Now().Add(18 * time.Second)
	for {
		receipt, rerr := ec.TransactionReceipt(ctx, h)
		if rerr == nil && receipt != nil {
			return true, receipt.Status == 1, nil
		}
		if errors.Is(rerr, ethereum.NotFound) || strings.Contains(strings.ToLower(rerr.Error()), "not found") {
			if time.Now().After(deadline) {
				return false, false, nil
			}
			time.Sleep(1 * time.Second)
			continue
		}
		if rerr != nil {
			return false, false, rerr
		}
		return false, false, nil
	}
}

func (s *ChatService) completeWalletTrainingUpload(botID, txHash string, root common.Hash, payload []byte, sampleIDs []string, zgsNodes []string) {
	// Give the chain + nodes time to sync.
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	// Discover nodes if needed.
	if len(zgsNodes) == 0 {
		if nodes, err := discoverZgsNodesViaIndexer(ctx); err == nil {
			zgsNodes = nodes
		}
	}
	if len(zgsNodes) == 0 {
		logrus.WithFields(logrus.Fields{"botId": botID, "root": root.Hex()}).Warn("no zgs nodes available for async upload")
		return
	}

	dataObj, err := core.NewDataInMemory(payload)
	if err != nil {
		logrus.WithError(err).Warn("async upload: failed to build data")
		return
	}
	tree, err := core.MerkleTree(dataObj)
	if err != nil {
		logrus.WithError(err).Warn("async upload: failed to build merkle")
		return
	}

	zgsClients, reachable, unreachable, err := reachableZgsClients(ctx, zgsNodes)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"reachable": reachable, "unreachable": unreachable}).Warn("async upload: no reachable zgs nodes")
		return
	}
	for _, c := range zgsClients {
		defer c.Close()
	}

	fileInfo, err := waitFileInfo(ctx, zgsClients, root, 8*time.Minute)
	if err != nil {
		logrus.WithError(err).Warn("async upload: file info not found")
		return
	}

	segments := buildAllSegmentsWithProof(dataObj, tree)
	uploader := transfer.NewFileSegmentUploader(zgsClients)
	if err := uploader.Upload(ctx, transfer.FileSegmentsWithProof{
		FileInfo: fileInfo,
		Segments: segments,
	}, transfer.UploadOption{
		ExpectedReplica: 1,
		TaskSize:        16,
		Method:          "min",
	}); err != nil {
		logrus.WithError(err).Warn("async upload: segment upload failed")
		return
	}

	ref := "0g://storage/root/" + root.Hex() + "?tx=" + strings.TrimSpace(txHash)
	current, err := s.store.ListTrainingSamples(botID)
	if err != nil {
		logrus.WithError(err).Warn("async upload: failed to list samples")
		return
	}
	idSet := map[string]bool{}
	for _, id := range sampleIDs {
		idSet[id] = true
	}
	for i := range current {
		if idSet[current[i].ID] {
			current[i].StoredOn0G = true
			current[i].StorageRef = ref
		}
	}
	_ = s.store.UpdateTrainingSamples(botID, current)
}

func discoverZgsNodesViaIndexer(ctx context.Context) ([]string, error) {
	indexerRPC := strings.TrimSpace(os.Getenv("ZERO_G_INDEXER_RPC"))
	if indexerRPC == "" {
		indexerRPC = "https://indexer-storage-testnet-standard.0g.ai"
	}

	// Retries are important: the default provider option has RetryCount=0 which makes
	// transient upstream issues (503) fail immediately.
	opt := providers.Option{}
	if to := strings.TrimSpace(os.Getenv("ZERO_G_RPC_TIMEOUT_MS")); to != "" {
		if n, err := time.ParseDuration(to + "ms"); err == nil && n > 0 {
			opt.RequestTimeout = n
		}
	}
	if opt.RequestTimeout == 0 {
		opt.RequestTimeout = 25 * time.Second
	}
	opt.RetryCount = 2
	if v := strings.TrimSpace(os.Getenv("ZERO_G_INDEXER_RETRY_COUNT")); v != "" {
		if n, err := parseInt(v); err == nil && n >= 0 {
			opt.RetryCount = n
		}
	}
	opt.RetryInterval = 750 * time.Millisecond
	if v := strings.TrimSpace(os.Getenv("ZERO_G_INDEXER_RETRY_INTERVAL_MS")); v != "" {
		if n, err := parseInt(v); err == nil && n > 0 {
			opt.RetryInterval = time.Duration(n) * time.Millisecond
		}
	}

	ic, err := indexer.NewClient(indexerRPC, indexer.IndexerClientOption{
		ProviderOption: opt,
		LogOption:      zg_common.LogOption{Logger: logrus.StandardLogger()},
		FullTrusted:    true,
	})
	if err != nil {
		return nil, err
	}

	nodes, err := ic.GetShardedNodes(ctx)
	if err != nil {
		// Surface as HTTP 503 so frontend shows a clear retryable error.
		if strings.Contains(err.Error(), "503") || strings.Contains(strings.ToLower(err.Error()), "service temporarily unavailable") {
			return nil, &zerog.ErrIndexerUnavailable{IndexerRPC: indexerRPC, Cause: err}
		}
		return nil, err
	}

	out := make([]string, 0, len(nodes.Trusted)+len(nodes.Discovered))
	seen := map[string]bool{}
	add := func(u string) {
		u = normalizeURL(u)
		if strings.TrimSpace(u) == "" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	for _, n := range nodes.Trusted {
		add(n.URL)
	}
	for _, n := range nodes.Discovered {
		add(n.URL)
	}
	if len(out) == 0 {
		return nil, errors.New("indexer returned no ZGS nodes")
	}
	return out, nil
}

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty")
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errors.New("invalid int")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func firstReachableZgs(ctx context.Context, nodes []string) (*node.ZgsClient, []string, []string, error) {
	probeTO := 9 * time.Second
	if raw := strings.TrimSpace(os.Getenv("ZERO_G_ZGS_PROBE_TIMEOUT_MS")); raw != "" {
		if n, err := parseInt(raw); err == nil && n > 0 {
			probeTO = time.Duration(n) * time.Millisecond
		}
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTO)
	defer cancel()

	opt := providers.Option{}
	opt.RequestTimeout = 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("ZERO_G_RPC_TIMEOUT_MS")); raw != "" {
		if n, err := parseInt(raw); err == nil && n > 0 {
			opt.RequestTimeout = time.Duration(n) * time.Millisecond
		}
	}
	opt.RetryCount = 0

	reachable := make([]string, 0, 1)
	unreachable := make([]string, 0)

	for _, raw := range nodes {
		cands := candidateURLs(raw)
		var lastErr error
		for _, u := range cands {
			c, err := node.NewZgsClient(u, nil, opt)
			if err != nil {
				lastErr = err
				continue
			}
			if _, err := c.GetStatus(probeCtx); err != nil {
				lastErr = err
				c.Close()
				continue
			}
			reachable = append(reachable, u)
			return c, reachable, unreachable, nil
		}
		_ = lastErr
		if len(cands) > 0 {
			unreachable = append(unreachable, cands[0])
		} else {
			unreachable = append(unreachable, strings.TrimSpace(raw))
		}
	}

	return nil, reachable, unreachable, errors.New("no reachable nodes")
}

func reachableZgsClients(ctx context.Context, nodes []string) ([]*node.ZgsClient, []string, []string, error) {
	probeTO := 9 * time.Second
	if raw := strings.TrimSpace(os.Getenv("ZERO_G_ZGS_PROBE_TIMEOUT_MS")); raw != "" {
		if n, err := parseInt(raw); err == nil && n > 0 {
			probeTO = time.Duration(n) * time.Millisecond
		}
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTO)
	defer cancel()

	opt := providers.Option{}
	opt.RequestTimeout = 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("ZERO_G_RPC_TIMEOUT_MS")); raw != "" {
		if n, err := parseInt(raw); err == nil && n > 0 {
			opt.RequestTimeout = time.Duration(n) * time.Millisecond
		}
	}
	opt.RetryCount = 0

	reachable := make([]string, 0)
	unreachable := make([]string, 0)
	clients := make([]*node.ZgsClient, 0)

	for _, raw := range nodes {
		cands := candidateURLs(raw)
		var picked *node.ZgsClient
		var pickedURL string
		var lastErr error
		for _, u := range cands {
			c, err := node.NewZgsClient(u, nil, opt)
			if err != nil {
				lastErr = err
				continue
			}
			if _, err := c.GetStatus(probeCtx); err != nil {
				lastErr = err
				c.Close()
				continue
			}
			picked = c
			pickedURL = u
			break
		}
		if picked == nil {
			// Only record a single representative URL to avoid noise.
			if len(cands) > 0 {
				unreachable = append(unreachable, cands[0])
			} else {
				unreachable = append(unreachable, strings.TrimSpace(raw))
			}
			_ = lastErr
			continue
		}
		reachable = append(reachable, pickedURL)
		clients = append(clients, picked)
	}

	if len(clients) == 0 {
		return nil, reachable, unreachable, errors.New("no reachable nodes")
	}
	return clients, reachable, unreachable, nil
}

func candidateURLs(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	// If scheme is absent, try both https and http (some public nodes expose https).
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return []string{"https://" + s, "http://" + s}
	}
	if strings.HasPrefix(s, "http://") {
		return []string{s, "https://" + strings.TrimPrefix(s, "http://")}
	}
	return []string{s, "http://" + strings.TrimPrefix(s, "https://")}
}

func waitFileInfo(ctx context.Context, clients []*node.ZgsClient, root common.Hash, timeout time.Duration) (*node.FileInfo, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, c := range clients {
			info, err := c.GetFileInfo(ctx, root, true)
			if err == nil && info != nil {
				return info, nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return nil, errors.New("file info not found on ZGS nodes (tx may not be mined or nodes not synced)")
}

func buildAllSegmentsWithProof(data core.IterableData, tree *merkle.Tree) []node.SegmentWithProof {
	numSegments := data.NumSegments()
	segs := make([]node.SegmentWithProof, 0, numSegments)
	for i := uint64(0); i < numSegments; i++ {
		segment, _ := core.ReadAt(data, core.DefaultSegmentSize, int64(i*core.DefaultSegmentSize), data.PaddedSize())
		proof := tree.ProofAt(int(i))
		segs = append(segs, node.SegmentWithProof{
			Root:     tree.Root(),
			Data:     segment,
			Index:    i,
			Proof:    proof,
			FileSize: uint64(data.Size()),
		})
	}
	return segs
}

func normalizeURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	return "http://" + s
}
