package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"

	"github.com/0gfoundation/0g-storage-client/contract"
	"github.com/0gfoundation/0g-storage-client/core"
	"github.com/0gfoundation/0g-storage-client/transfer"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (s *ChatService) PrepareSkillsBundlePublish(ctx context.Context, botID string, walletAddress string, skillIDs []string, zgsNodes []string) (WalletTxRequest, int, error) {
	skills, err := s.store.ListSkills(botID)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	if len(skills) == 0 {
		return WalletTxRequest{}, 0, errors.New("no skills to publish")
	}

	// Determine which skills to include.
	idAllow := map[string]bool{}
	if len(skillIDs) > 0 {
		for _, id := range skillIDs {
			id = strings.TrimSpace(id)
			if id != "" {
				idAllow[id] = true
			}
		}
	}

	type skillExport struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	outSkills := make([]skillExport, 0, len(skills))
	outIDs := make([]string, 0, len(skills))

	for _, sk := range skills {
		if len(idAllow) > 0 && !idAllow[sk.ID] {
			continue
		}
		if sk.StoredOn0G {
			continue
		}
		if strings.TrimSpace(sk.Content) == "" {
			continue
		}
		outSkills = append(outSkills, skillExport{
			ID:       sk.ID,
			Name:     sk.Name,
			Filename: sk.Filename,
			Content:  sk.Content,
		})
		outIDs = append(outIDs, sk.ID)
	}
	if len(outSkills) == 0 {
		return WalletTxRequest{}, 0, errors.New("no pending skills to publish")
	}

	payload, err := json.Marshal(map[string]any{
		"botId":       botID,
		"exportedAt":  time.Now().UTC().Format(time.RFC3339Nano),
		"kind":        "skills_bundle",
		"skills":      outSkills,
		"skillCount":  len(outSkills),
		"storageTarget": "0G",
	})
	if err != nil {
		return WalletTxRequest{}, 0, err
	}

	flowAddr := common.Address{}
	if v := strings.TrimSpace(os.Getenv("ZERO_G_FLOW_CONTRACT_ADDRESS")); v != "" {
		if common.IsHexAddress(v) {
			flowAddr = common.HexToAddress(v)
		} else {
			return WalletTxRequest{}, 0, errors.New("invalid ZERO_G_FLOW_CONTRACT_ADDRESS")
		}
	}
	if flowAddr == (common.Address{}) {
		if len(zgsNodes) == 0 {
			nodes, err := discoverZgsNodesViaIndexer(ctx)
			if err != nil {
				return WalletTxRequest{}, 0, err
			}
			zgsNodes = nodes
		}
		zgsClient, reachable, unreachable, err := firstReachableZgs(ctx, zgsNodes)
		if err != nil {
			return WalletTxRequest{}, 0, pkgerrors.WithMessagef(err, "no reachable ZGS nodes (reachable=%v unreachable=%v)", reachable, unreachable)
		}
		defer zgsClient.Close()
		status, err := zgsClient.GetStatus(ctx)
		if err != nil {
			return WalletTxRequest{}, 0, err
		}
		flowAddr = status.NetworkIdentity.FlowContractAddress
	}

	evmRPC := strings.TrimSpace(os.Getenv("ZERO_G_EVM_RPC"))
	if evmRPC == "" {
		evmRPC = "https://evmrpc-testnet.0g.ai"
	}
	ec, err := ethclient.DialContext(ctx, evmRPC)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	defer ec.Close()

	chainID, err := ec.ChainID(ctx)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	code, err := ec.CodeAt(ctx, flowAddr, nil)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	if len(code) == 0 {
		return WalletTxRequest{}, 0, errors.New("no contract code at given address (check ZERO_G_FLOW_CONTRACT_ADDRESS and ZERO_G_EVM_RPC chainId=" + chainID.String() + ", addr=" + flowAddr.Hex() + ")")
	}

	dataObj, err := core.NewDataInMemory(payload)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	tree, err := core.MerkleTree(dataObj)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}

	flow := core.NewFlow(dataObj, nil)
	submission, err := flow.CreateSubmission(common.HexToAddress(walletAddress))
	if err != nil {
		return WalletTxRequest{}, 0, err
	}

	flowCaller, err := contract.NewFlow(flowAddr, ec)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	marketAddr, err := flowCaller.Market(nil)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	market, err := contract.NewMarket(marketAddr, ec)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	pricePerSector, err := market.PricePerSector(nil)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	fee := submission.Fee(pricePerSector)

	flowABI, err := abi.JSON(strings.NewReader(contract.FlowMetaData.ABI))
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	calldata, err := flowABI.Pack("submit", *submission)
	if err != nil {
		return WalletTxRequest{}, 0, err
	}

	publishID, err := newSnapshotID()
	if err != nil {
		return WalletTxRequest{}, 0, err
	}
	s.putSnapshot(walletPublishSnapshot{
		ID:       publishID,
		BotID:    botID,
		Wallet:   walletAddress,
		Kind:     "skills_bundle",
		SkillIDs: outIDs,
		Root:     tree.Root(),
		Payload:  payload,
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
	}, len(outIDs), nil
}

func (s *ChatService) FinalizeSkillsBundlePublish(ctx context.Context, botID string, publishID string, txHash string, rootHash string, zgsNodes []string) (domain.PublishResult, error) {
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
	if ss.BotID != botID || ss.Kind != "skills_bundle" {
		return domain.PublishResult{}, errors.New("publishId does not match this skills bundle (please prepare again)")
	}
	if ss.Root != root {
		return domain.PublishResult{}, errors.New("rootHash mismatch (payload changed; please prepare+send tx again)")
	}
	payload := append([]byte(nil), ss.Payload...)
	skillIDs := append([]string(nil), ss.SkillIDs...)

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

	mined, success, err := checkTxReceipt(ctx, strings.TrimSpace(txHash))
	if err != nil {
		return domain.PublishResult{}, err
	}
	if mined && !success {
		return domain.PublishResult{}, errors.New("transaction failed on-chain")
	}

	s.deleteSnapshot(publishID)
	go s.completeWalletSkillsBundleUpload(botID, strings.TrimSpace(txHash), root, payload, skillIDs, zgsNodes)

	explorer := strings.TrimRight(strings.TrimSpace(os.Getenv("ZERO_G_EXPLORER_BASE")), "/")
	if explorer == "" {
		explorer = "https://chainscan-galileo.0g.ai"
	}
	explorerTx := ""
	if explorer != "" && strings.TrimSpace(txHash) != "" {
		explorerTx = explorer + "/tx/" + strings.TrimSpace(txHash)
	}
	ref := "0g://storage/root/" + root.Hex() + "?tx=" + strings.TrimSpace(txHash)

	// Mark skills as "uploaded" immediately after the on-chain tx is confirmed.
	// Segment upload to storage nodes may still be running asynchronously, but for UI/LLM usage
	// we can treat on-chain success as "available" (content is already in our store).
	if len(skillIDs) > 0 {
		if skills, err := s.store.ListSkills(botID); err == nil {
			idSet := map[string]bool{}
			for _, id := range skillIDs {
				idSet[id] = true
			}
			changed := false
			for i := range skills {
				if idSet[skills[i].ID] {
					skills[i].StoredOn0G = true
					skills[i].StorageRef = ref
					skills[i].TxHash = strings.TrimSpace(txHash)
					skills[i].RootHash = root.Hex()
					changed = true
				}
			}
			if changed {
				_ = s.store.UpdateSkills(botID, skills)
			}
		}
	}

	return domain.PublishResult{
		BotID:            botID,
		SampleCount:      len(skillIDs),
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
	}, nil
}

func (s *ChatService) completeWalletSkillsBundleUpload(botID, txHash string, root common.Hash, payload []byte, skillIDs []string, zgsNodes []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	if len(zgsNodes) == 0 {
		if nodes, err := discoverZgsNodesViaIndexer(ctx); err == nil {
			zgsNodes = nodes
		}
	}
	if len(zgsNodes) == 0 {
		logrus.WithFields(logrus.Fields{"botId": botID, "root": root.Hex()}).Warn("no zgs nodes available for async skills upload")
		return
	}

	dataObj, err := core.NewDataInMemory(payload)
	if err != nil {
		logrus.WithError(err).Warn("async skills upload: failed to build data")
		return
	}
	tree, err := core.MerkleTree(dataObj)
	if err != nil {
		logrus.WithError(err).Warn("async skills upload: failed to build merkle")
		return
	}

	zgsClients, reachable, unreachable, err := reachableZgsClients(ctx, zgsNodes)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"reachable": reachable, "unreachable": unreachable}).Warn("async skills upload: no reachable zgs nodes")
		return
	}
	for _, c := range zgsClients {
		defer c.Close()
	}

	fileInfo, err := waitFileInfo(ctx, zgsClients, root, 8*time.Minute)
	if err != nil {
		logrus.WithError(err).Warn("async skills upload: file info not found")
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
		logrus.WithError(err).Warn("async skills upload: segment upload failed")
		return
	}

	ref := "0g://storage/root/" + root.Hex() + "?tx=" + strings.TrimSpace(txHash)
	skills, err := s.store.ListSkills(botID)
	if err != nil {
		logrus.WithError(err).Warn("async skills upload: failed to list skills")
		return
	}
	idSet := map[string]bool{}
	for _, id := range skillIDs {
		idSet[id] = true
	}
	for i := range skills {
		if idSet[skills[i].ID] {
			skills[i].StoredOn0G = true
			skills[i].StorageRef = ref
			skills[i].TxHash = strings.TrimSpace(txHash)
			skills[i].RootHash = root.Hex()
		}
	}
	_ = s.store.UpdateSkills(botID, skills)
}

// A small local helper; avoid importing ethereum receipt code here again.
func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ethereum.NotFound) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
