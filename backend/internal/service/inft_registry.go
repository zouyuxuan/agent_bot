package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const memoryRegistryABI = `[
  {
    "type":"function",
    "name":"previewAssetId",
    "stateMutability":"pure",
    "inputs":[
      {"name":"owner","type":"address"},
      {"name":"kind","type":"uint8"},
      {"name":"rootHash","type":"bytes32"},
      {"name":"storageRef","type":"string"},
      {"name":"name","type":"string"},
      {"name":"parentAssetId","type":"bytes32"}
    ],
    "outputs":[{"name":"","type":"bytes32"}]
  },
  {
    "type":"function",
    "name":"registerAsset",
    "stateMutability":"nonpayable",
    "inputs":[
      {"name":"kind","type":"uint8"},
      {"name":"rootHash","type":"bytes32"},
      {"name":"storageRef","type":"string"},
      {"name":"name","type":"string"},
      {"name":"parentAssetId","type":"bytes32"}
    ],
    "outputs":[{"name":"assetId","type":"bytes32"}]
  }
]`

func (s *ChatService) PrepareINFTRegister(ctx context.Context, botID, inftID, walletAddress string) (WalletTxRequest, error) {
	inft, err := s.store.GetINFT(botID, inftID)
	if err != nil {
		return WalletTxRequest{}, err
	}
	if !inft.StoredOn0G || strings.TrimSpace(inft.RootHash) == "" || strings.TrimSpace(inft.StorageRef) == "" {
		return WalletTxRequest{}, errors.New("inft must be published to 0G before registry registration")
	}
	if inft.RegistryRegistered {
		return WalletTxRequest{}, errors.New("inft already registered on chain")
	}

	registryAddr, err := memoryRegistryAddressFromEnv()
	if err != nil {
		return WalletTxRequest{}, err
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
	code, err := ec.CodeAt(ctx, registryAddr, nil)
	if err != nil {
		return WalletTxRequest{}, err
	}
	if len(code) == 0 {
		return WalletTxRequest{}, errors.New("no contract code at given address (check ZERO_G_MEMORY_REGISTRY_ADDRESS)")
	}

	registryKind := uint8(resolveINFTRegistryKind(inft))
	parentAssetID := common.Hash{}
	if parentID := strings.TrimSpace(inft.ParentINFTID); parentID != "" {
		parent, err := s.store.GetINFT(botID, parentID)
		if err != nil {
			return WalletTxRequest{}, err
		}
		if !parent.RegistryRegistered || strings.TrimSpace(parent.RegistryAssetID) == "" {
			return WalletTxRequest{}, errors.New("parent inft must be registered on chain first")
		}
		parentAssetID = common.HexToHash(strings.TrimSpace(parent.RegistryAssetID))
	}

	parsedABI, err := abi.JSON(strings.NewReader(memoryRegistryABI))
	if err != nil {
		return WalletTxRequest{}, err
	}
	predictedAssetID, err := previewMemoryRegistryAssetID(ctx, ec, parsedABI, registryAddr, common.HexToAddress(walletAddress), registryKind, common.HexToHash(inft.RootHash), strings.TrimSpace(inft.StorageRef), strings.TrimSpace(inft.Name), parentAssetID)
	if err != nil {
		return WalletTxRequest{}, err
	}
	calldata, err := parsedABI.Pack("registerAsset", registryKind, common.HexToHash(inft.RootHash), strings.TrimSpace(inft.StorageRef), strings.TrimSpace(inft.Name), parentAssetID)
	if err != nil {
		return WalletTxRequest{}, err
	}

	publishID, err := newSnapshotID()
	if err != nil {
		return WalletTxRequest{}, err
	}
	s.putSnapshot(walletPublishSnapshot{
		ID:              publishID,
		BotID:           botID,
		Wallet:          walletAddress,
		Kind:            "inft_register",
		INFTID:          inft.ID,
		RegistryAssetID: predictedAssetID.Hex(),
		Root:            common.HexToHash(inft.RootHash),
		CreatedAt:       time.Now(),
	})

	return WalletTxRequest{
		PublishID:  publishID,
		ChainIDHex: "0x" + chainID.Text(16),
		ChainIDDec: chainID.String(),
		From:       walletAddress,
		To:         registryAddr.Hex(),
		Data:       "0x" + common.Bytes2Hex(calldata),
		Value:      "0x0",
		RootHash:   common.HexToHash(inft.RootHash).Hex(),
	}, nil
}

func (s *ChatService) FinalizeINFTRegister(ctx context.Context, botID, inftID, publishID, txHash string) (domain.INFTAsset, error) {
	publishID = strings.TrimSpace(publishID)
	if publishID == "" {
		return domain.INFTAsset{}, errors.New("missing publishId (please prepare again)")
	}
	ss, err := s.getSnapshot(publishID, 15*time.Minute)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	if ss.BotID != botID || ss.Kind != "inft_register" || ss.INFTID != inftID {
		return domain.INFTAsset{}, errors.New("publishId does not match this inft registration")
	}
	if strings.TrimSpace(txHash) == "" {
		return domain.INFTAsset{}, errors.New("missing txHash")
	}

	mined, success, err := checkTxReceipt(ctx, strings.TrimSpace(txHash))
	if err != nil {
		return domain.INFTAsset{}, err
	}
	if !mined {
		return domain.INFTAsset{}, errors.New("transaction not mined yet")
	}
	if !success {
		return domain.INFTAsset{}, errors.New("transaction failed on-chain")
	}

	infts, err := s.store.ListINFTs(botID)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	explorerBase := strings.TrimRight(strings.TrimSpace(os.Getenv("ZERO_G_EXPLORER_BASE")), "/")
	if explorerBase == "" {
		explorerBase = "https://chainscan-galileo.0g.ai"
	}
	var updated domain.INFTAsset
	found := false
	for i := range infts {
		if infts[i].ID == inftID {
			infts[i].RegistryAssetID = strings.TrimSpace(ss.RegistryAssetID)
			infts[i].RegistryTxHash = strings.TrimSpace(txHash)
			infts[i].RegistryContract, _ = memoryRegistryAddressHexFromEnv()
			infts[i].RegistryRegistered = true
			infts[i].RegistryRegisteredAt = time.Now()
			infts[i].RegistryExplorerTxURL = explorerBase + "/tx/" + strings.TrimSpace(txHash)
			infts[i].UpdatedAt = time.Now()
			updated = infts[i]
			found = true
			break
		}
	}
	if !found {
		return domain.INFTAsset{}, errors.New("inft not found")
	}
	if err := s.store.UpdateINFTs(botID, infts); err != nil {
		return domain.INFTAsset{}, err
	}
	s.deleteSnapshot(publishID)
	return updated, nil
}

func resolveINFTRegistryKind(inft domain.INFTAsset) int {
	if strings.TrimSpace(inft.Kind) == "distilled_memory" {
		return 1
	}
	return 0
}

func memoryRegistryAddressFromEnv() (common.Address, error) {
	raw := strings.TrimSpace(os.Getenv("ZERO_G_MEMORY_REGISTRY_ADDRESS"))
	if raw == "" {
		return common.Address{}, errors.New("missing ZERO_G_MEMORY_REGISTRY_ADDRESS")
	}
	if !common.IsHexAddress(raw) {
		return common.Address{}, errors.New("invalid ZERO_G_MEMORY_REGISTRY_ADDRESS")
	}
	return common.HexToAddress(raw), nil
}

func memoryRegistryAddressHexFromEnv() (string, error) {
	addr, err := memoryRegistryAddressFromEnv()
	if err != nil {
		return "", err
	}
	return addr.Hex(), nil
}

func previewMemoryRegistryAssetID(ctx context.Context, ec *ethclient.Client, parsedABI abi.ABI, registry common.Address, owner common.Address, kind uint8, rootHash common.Hash, storageRef string, name string, parentAssetID common.Hash) (common.Hash, error) {
	callData, err := parsedABI.Pack("previewAssetId", owner, kind, rootHash, storageRef, name, parentAssetID)
	if err != nil {
		return common.Hash{}, err
	}
	out, err := ec.CallContract(ctx, ethereum.CallMsg{
		To:   &registry,
		Data: callData,
	}, nil)
	if err == nil && len(out) > 0 {
		values, unpackErr := parsedABI.Unpack("previewAssetId", out)
		if unpackErr == nil && len(values) == 1 {
			switch v := values[0].(type) {
			case [32]byte:
				return common.BytesToHash(v[:]), nil
			case common.Hash:
				return v, nil
			}
		}
	}

	// Fallback to off-chain deterministic computation matching abi.encode(owner, kind, rootHash, storageRef, name, parentAssetId).
	args := abi.Arguments{
		{Type: mustABIType("address")},
		{Type: mustABIType("uint8")},
		{Type: mustABIType("bytes32")},
		{Type: mustABIType("string")},
		{Type: mustABIType("string")},
		{Type: mustABIType("bytes32")},
	}
	packed, packErr := args.Pack(owner, kind, rootHash, storageRef, name, parentAssetID)
	if packErr != nil {
		return common.Hash{}, packErr
	}
	return crypto.Keccak256Hash(packed), nil
}

func mustABIType(typeName string) abi.Type {
	t, err := abi.NewType(typeName, "", nil)
	if err != nil {
		panic(err)
	}
	return t
}
