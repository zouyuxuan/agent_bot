package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"

	"github.com/0gfoundation/0g-storage-client/core"
	"github.com/0gfoundation/0g-storage-client/transfer"
	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
)

func (s *ChatService) ListINFTs(botID string) ([]domain.INFTAsset, error) {
	return s.store.ListINFTs(botID)
}

func (s *ChatService) GetINFT(botID, inftID string) (domain.INFTAsset, error) {
	return s.store.GetINFT(botID, inftID)
}

func (s *ChatService) CreateTrainingINFT(botID string) (domain.INFTAsset, error) {
	bot, err := s.store.GetBot(botID)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	samples, err := s.store.ListTrainingSamples(botID)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	if len(samples) == 0 {
		return domain.INFTAsset{}, errors.New("no training samples to convert into iNFT")
	}

	now := time.Now()
	name := strings.TrimSpace(bot.Name)
	if name == "" {
		name = bot.ID
	}
	if name == "" {
		name = "Agent"
	}

	sampleIDs := collectTrainingSampleIDs(samples)
	payload := map[string]any{
		"standard":    "0g-agent-inft/v1",
		"assetType":   "iNFT",
		"kind":        "training_memory",
		"name":        fmt.Sprintf("%s Training Memory iNFT", name),
		"description": "A user-owned AI memory asset built from raw training conversations.",
		"bot": map[string]any{
			"id":          bot.ID,
			"name":        bot.Name,
			"personality": bot.Personality,
			"modelType":   bot.ModelType,
		},
		"metadata": map[string]any{
			"createdAt":   now.UTC().Format(time.RFC3339Nano),
			"sampleCount": len(samples),
			"sampleIds":   sampleIDs,
			"tags":        collectTrainingTags(samples),
		},
		"memory": map[string]any{
			"trainingSet": samples,
		},
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return domain.INFTAsset{}, err
	}

	inft := domain.INFTAsset{
		ID:           fmt.Sprintf("%s-inft-%d", botID, now.UnixNano()),
		BotID:        botID,
		Kind:         "training_memory",
		Name:         fmt.Sprintf("%s Training Memory iNFT", name),
		Description:  "A user-owned AI memory asset built from raw training conversations.",
		Filename:     "inft/training-memory.json",
		ContentType:  "application/json",
		Content:      string(raw),
		SizeBytes:    len(raw),
		SampleCount:  len(samples),
		SampleIDs:    sampleIDs,
		Source:       "training",
		RegistryKind: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return s.store.SaveINFT(botID, inft)
}

func (s *ChatService) CreateDistilledINFT(botID string, memorySummary, description string) (domain.INFTAsset, error) {
	bot, err := s.store.GetBot(botID)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	memorySummary = strings.TrimSpace(memorySummary)
	if memorySummary == "" {
		return domain.INFTAsset{}, errors.New("missing distilled memory summary")
	}
	description = strings.TrimSpace(description)
	if description == "" {
		description = "A user-owned distilled AI memory asset summarizing long-term agent memory."
	}

	samples, err := s.store.ListTrainingSamples(botID)
	if err != nil {
		return domain.INFTAsset{}, err
	}

	now := time.Now()
	name := strings.TrimSpace(bot.Name)
	if name == "" {
		name = bot.ID
	}
	if name == "" {
		name = "Agent"
	}

	parentINFTID, _ := s.latestTrainingINFTID(botID)

	sampleIDs := collectTrainingSampleIDs(samples)
	payload := map[string]any{
		"standard":    "0g-agent-inft/v1",
		"assetType":   "iNFT",
		"kind":        "distilled_memory",
		"name":        fmt.Sprintf("%s Distilled Memory iNFT", name),
		"description": description,
		"bot": map[string]any{
			"id":          bot.ID,
			"name":        bot.Name,
			"personality": bot.Personality,
			"modelType":   bot.ModelType,
		},
		"metadata": map[string]any{
			"createdAt":   now.UTC().Format(time.RFC3339Nano),
			"sampleCount": len(samples),
			"sampleIds":   sampleIDs,
			"tags":        collectTrainingTags(samples),
		},
		"memory": map[string]any{
			"summary": memorySummary,
		},
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return domain.INFTAsset{}, err
	}

	inft := domain.INFTAsset{
		ID:           fmt.Sprintf("%s-inft-%d", botID, now.UnixNano()),
		BotID:        botID,
		Kind:         "distilled_memory",
		Name:         fmt.Sprintf("%s Distilled Memory iNFT", name),
		Description:  description,
		Filename:     "inft/distilled-memory.json",
		ContentType:  "application/json",
		Content:      string(raw),
		SizeBytes:    len(raw),
		SampleCount:  len(samples),
		SampleIDs:    sampleIDs,
		Source:       "distillation",
		ParentINFTID: parentINFTID,
		RegistryKind: 1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return s.store.SaveINFT(botID, inft)
}

func (s *ChatService) latestTrainingINFTID(botID string) (string, bool) {
	infts, err := s.store.ListINFTs(botID)
	if err != nil || len(infts) == 0 {
		return "", false
	}
	for i := len(infts) - 1; i >= 0; i-- {
		if strings.TrimSpace(infts[i].Kind) == "training_memory" {
			return strings.TrimSpace(infts[i].ID), true
		}
	}
	return "", false
}

func collectTrainingTags(samples []domain.TrainingSample) []string {
	if len(samples) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	for _, sample := range samples {
		for _, tag := range sample.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			out = append(out, tag)
			if len(out) >= 8 {
				return out
			}
		}
	}
	return out
}

func (s *ChatService) PrepareINFTPublish(ctx context.Context, botID, inftID, walletAddress string, zgsNodes []string) (WalletTxRequest, error) {
	inft, err := s.store.GetINFT(botID, inftID)
	if err != nil {
		return WalletTxRequest{}, err
	}
	if strings.TrimSpace(inft.Content) == "" {
		return WalletTxRequest{}, errors.New("empty inft content")
	}
	return s.prepareAssetPublish(ctx, botID, walletAddress, []byte(inft.Content), "inft", "", nil, inft.ID, zgsNodes)
}

func (s *ChatService) FinalizeINFTPublish(ctx context.Context, botID, inftID, publishID, txHash, rootHash string, zgsNodes []string) (domain.INFTAsset, error) {
	publishID = strings.TrimSpace(publishID)
	if publishID == "" {
		return domain.INFTAsset{}, errors.New("missing publishId (please prepare again)")
	}
	if strings.TrimSpace(rootHash) == "" {
		return domain.INFTAsset{}, errors.New("missing rootHash")
	}
	root := common.HexToHash(rootHash)
	if root == (common.Hash{}) {
		return domain.INFTAsset{}, errors.New("invalid rootHash")
	}

	ss, err := s.getSnapshot(publishID, 15*time.Minute)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	if ss.BotID != botID || ss.Kind != "inft" || ss.INFTID != inftID {
		return domain.INFTAsset{}, errors.New("publishId does not match this inft (please prepare again)")
	}
	if ss.Root != root {
		return domain.INFTAsset{}, errors.New("rootHash mismatch (payload changed; please prepare+send tx again)")
	}
	payload := append([]byte(nil), ss.Payload...)

	dataObj, err := core.NewDataInMemory(payload)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	tree, err := core.MerkleTree(dataObj)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	if tree.Root() != root {
		return domain.INFTAsset{}, errors.New("rootHash mismatch (payload changed; please prepare+send tx again)")
	}

	mined, success, err := checkTxReceipt(ctx, strings.TrimSpace(txHash))
	if err != nil {
		return domain.INFTAsset{}, err
	}
	if mined && !success {
		return domain.INFTAsset{}, errors.New("transaction failed on-chain")
	}

	s.deleteSnapshot(publishID)
	go s.completeWalletINFTUpload(botID, inftID, strings.TrimSpace(txHash), root, payload, zgsNodes)

	asset, err := s.store.GetINFT(botID, inftID)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	explorer := strings.TrimRight(strings.TrimSpace(os.Getenv("ZERO_G_EXPLORER_BASE")), "/")
	if explorer == "" {
		explorer = "https://chainscan-galileo.0g.ai"
	}
	ref := "0g://storage/root/" + root.Hex() + "?tx=" + strings.TrimSpace(txHash)

	infts, err := s.store.ListINFTs(botID)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	found := false
	for i := range infts {
		if infts[i].ID == inftID {
			infts[i].StoredOn0G = true
			infts[i].StorageRef = ref
			infts[i].TxHash = strings.TrimSpace(txHash)
			infts[i].RootHash = root.Hex()
			infts[i].UpdatedAt = time.Now()
			asset = infts[i]
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

	_ = explorer
	_ = mined
	_ = success
	return asset, nil
}

func (s *ChatService) completeWalletINFTUpload(botID, inftID, txHash string, root common.Hash, payload []byte, zgsNodes []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	if len(zgsNodes) == 0 {
		if nodes, err := discoverZgsNodesViaIndexer(ctx); err == nil {
			zgsNodes = nodes
		}
	}
	if len(zgsNodes) == 0 {
		logrus.WithFields(logrus.Fields{"botId": botID, "inftId": inftID, "root": root.Hex()}).Warn("no zgs nodes available for async inft upload")
		return
	}

	dataObj, err := core.NewDataInMemory(payload)
	if err != nil {
		logrus.WithError(err).Warn("async inft upload: failed to build data")
		return
	}
	tree, err := core.MerkleTree(dataObj)
	if err != nil {
		logrus.WithError(err).Warn("async inft upload: failed to build merkle")
		return
	}

	zgsClients, reachable, unreachable, err := reachableZgsClients(ctx, zgsNodes)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"reachable": reachable, "unreachable": unreachable}).Warn("async inft upload: no reachable zgs nodes")
		return
	}
	for _, c := range zgsClients {
		defer c.Close()
	}

	fileInfo, err := waitFileInfo(ctx, zgsClients, root, 8*time.Minute)
	if err != nil {
		logrus.WithError(err).Warn("async inft upload: file info not found")
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
		logrus.WithError(err).Warn("async inft upload: segment upload failed")
		return
	}

	ref := "0g://storage/root/" + root.Hex() + "?tx=" + strings.TrimSpace(txHash)
	infts, err := s.store.ListINFTs(botID)
	if err != nil {
		logrus.WithError(err).Warn("async inft upload: failed to list infts")
		return
	}
	for i := range infts {
		if infts[i].ID == inftID {
			infts[i].StoredOn0G = true
			infts[i].StorageRef = ref
			infts[i].TxHash = strings.TrimSpace(txHash)
			infts[i].RootHash = root.Hex()
			infts[i].UpdatedAt = time.Now()
			break
		}
	}
	_ = s.store.UpdateINFTs(botID, infts)
}
