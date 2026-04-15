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
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	pkgerrors "github.com/pkg/errors"
)

func (s *ChatService) PrepareSkillPublish(ctx context.Context, botID, skillID, walletAddress string, zgsNodes []string) (WalletTxRequest, error) {
	sk, err := s.store.GetSkill(botID, skillID)
	if err != nil {
		return WalletTxRequest{}, err
	}
	if strings.TrimSpace(sk.Content) == "" {
		return WalletTxRequest{}, errors.New("empty skill content")
	}

	exportedAt := time.Now().UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(map[string]any{
		"botId":         botID,
		"skillId":       sk.ID,
		"name":          sk.Name,
		"filename":      sk.Filename,
		"contentType":   sk.ContentType,
		"content":       sk.Content,
		"exportedAt":    exportedAt,
		"storageTarget": "0G",
	})
	if err != nil {
		return WalletTxRequest{}, err
	}

	flowAddr := common.Address{}
	if v := strings.TrimSpace(os.Getenv("ZERO_G_FLOW_CONTRACT_ADDRESS")); v != "" {
		if common.IsHexAddress(v) {
			flowAddr = common.HexToAddress(v)
		} else {
			return WalletTxRequest{}, errors.New("invalid ZERO_G_FLOW_CONTRACT_ADDRESS")
		}
	}
	if flowAddr == (common.Address{}) {
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
	s.putSnapshot(walletPublishSnapshot{
		ID:        publishID,
		BotID:     botID,
		Wallet:    walletAddress,
		Kind:      "skill",
		SkillID:   sk.ID,
		Root:      tree.Root(),
		Payload:   payload,
		CreatedAt: time.Now(),
	})

	return WalletTxRequest{
		PublishID: publishID,
		ChainIDHex: "0x" + chainID.Text(16),
		ChainIDDec: chainID.String(),
		From:       walletAddress,
		To:         flowAddr.Hex(),
		Data:       "0x" + common.Bytes2Hex(calldata),
		Value:      "0x" + fee.Text(16),
		RootHash:   tree.Root().Hex(),
	}, nil
}

func (s *ChatService) FinalizeSkillPublish(ctx context.Context, botID, skillID, publishID, txHash, rootHash string, zgsNodes []string) (domain.Skill, error) {
	publishID = strings.TrimSpace(publishID)
	if publishID == "" {
		return domain.Skill{}, errors.New("missing publishId (please prepare again)")
	}
	if strings.TrimSpace(rootHash) == "" {
		return domain.Skill{}, errors.New("missing rootHash")
	}
	root := common.HexToHash(rootHash)
	if root == (common.Hash{}) {
		return domain.Skill{}, errors.New("invalid rootHash")
	}

	ss, err := s.getSnapshot(publishID, 15*time.Minute)
	if err != nil {
		return domain.Skill{}, err
	}
	if ss.BotID != botID || ss.Kind != "skill" || ss.SkillID != skillID {
		return domain.Skill{}, errors.New("publishId does not match this skill (please prepare again)")
	}
	if ss.Root != root {
		return domain.Skill{}, errors.New("rootHash mismatch (payload changed; please prepare+send tx again)")
	}
	payload := ss.Payload

	if len(zgsNodes) == 0 {
		nodes, err := discoverZgsNodesViaIndexer(ctx)
		if err != nil {
			return domain.Skill{}, err
		}
		zgsNodes = nodes
	}

	dataObj, err := core.NewDataInMemory(payload)
	if err != nil {
		return domain.Skill{}, err
	}
	tree, err := core.MerkleTree(dataObj)
	if err != nil {
		return domain.Skill{}, err
	}
	if tree.Root() != root {
		return domain.Skill{}, errors.New("rootHash mismatch (payload changed; please prepare+send tx again)")
	}

	zgsClients, reachable, unreachable, err := reachableZgsClients(ctx, zgsNodes)
	if err != nil {
		return domain.Skill{}, pkgerrors.WithMessagef(err, "no reachable ZGS nodes (reachable=%v unreachable=%v)", reachable, unreachable)
	}
	for _, c := range zgsClients {
		defer c.Close()
	}

	fileInfo, err := waitFileInfo(ctx, zgsClients, root, 2*time.Minute)
	if err != nil {
		return domain.Skill{}, err
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
		return domain.Skill{}, err
	}

	ref := "0g://storage/root/" + root.Hex() + "?tx=" + strings.TrimSpace(txHash)

	skills, err := s.store.ListSkills(botID)
	if err != nil {
		return domain.Skill{}, err
	}
	var updated domain.Skill
	found := false
	for i := range skills {
		if skills[i].ID == skillID {
			skills[i].StoredOn0G = true
			skills[i].StorageRef = ref
			skills[i].TxHash = strings.TrimSpace(txHash)
			skills[i].RootHash = root.Hex()
			skills[i].UpdatedAt = time.Now()
			updated = skills[i]
			found = true
			break
		}
	}
	if !found {
		return domain.Skill{}, errors.New("skill not found")
	}
	_ = s.store.UpdateSkills(botID, skills)
	s.deleteSnapshot(publishID)

	_ = reachable
	return updated, nil
}
