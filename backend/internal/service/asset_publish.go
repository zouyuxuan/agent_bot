package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/0gfoundation/0g-storage-client/contract"
	"github.com/0gfoundation/0g-storage-client/core"
	"github.com/0gfoundation/0g-storage-client/transfer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	pkgerrors "github.com/pkg/errors"
)

func (s *ChatService) prepareAssetPublish(ctx context.Context, botID, walletAddress string, payload []byte, kind, skillID string, skillIDs []string, inftID string, zgsNodes []string) (WalletTxRequest, error) {
	if len(payload) == 0 {
		return WalletTxRequest{}, errors.New("empty asset payload")
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
		Kind:      kind,
		SkillID:   skillID,
		SkillIDs:  append([]string(nil), skillIDs...),
		INFTID:    inftID,
		Root:      tree.Root(),
		Payload:   append([]byte(nil), payload...),
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

func (s *ChatService) finalizeAssetPublish(ctx context.Context, botID, publishID, txHash, rootHash string, zgsNodes []string, updateFn func(root common.Hash, txHash, ref string) error) (string, error) {
	if strings.TrimSpace(rootHash) == "" {
		return "", errors.New("missing rootHash")
	}
	root := common.HexToHash(rootHash)
	if root == (common.Hash{}) {
		return "", errors.New("invalid rootHash")
	}
	ss, err := s.getSnapshot(strings.TrimSpace(publishID), 15*time.Minute)
	if err != nil {
		return "", err
	}
	if ss.BotID != botID {
		return "", errors.New("publishId does not match botId (please prepare again)")
	}
	payload := append([]byte(nil), ss.Payload...)

	if len(zgsNodes) == 0 {
		nodes, err := discoverZgsNodesViaIndexer(ctx)
		if err != nil {
			return "", err
		}
		zgsNodes = nodes
	}

	dataObj, err := core.NewDataInMemory(payload)
	if err != nil {
		return "", err
	}
	tree, err := core.MerkleTree(dataObj)
	if err != nil {
		return "", err
	}
	if tree.Root() != root {
		return "", errors.New("rootHash mismatch (payload changed; please prepare+send tx again)")
	}

	zgsClients, reachable, unreachable, err := reachableZgsClients(ctx, zgsNodes)
	if err != nil {
		return "", pkgerrors.WithMessagef(err, "no reachable ZGS nodes (reachable=%v unreachable=%v)", reachable, unreachable)
	}
	for _, c := range zgsClients {
		defer c.Close()
	}

	fileInfo, err := waitFileInfo(ctx, zgsClients, root, 2*time.Minute)
	if err != nil {
		return "", err
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
		return "", err
	}

	ref := "0g://storage/root/" + root.Hex() + "?tx=" + strings.TrimSpace(txHash)
	if updateFn != nil {
		if err := updateFn(root, strings.TrimSpace(txHash), ref); err != nil {
			return "", err
		}
	}
	s.deleteSnapshot(strings.TrimSpace(publishID))
	return ref, nil
}
