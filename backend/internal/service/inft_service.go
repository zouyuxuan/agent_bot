package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"

	"github.com/ethereum/go-ethereum/common"
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
		ID:          fmt.Sprintf("%s-inft-%d", botID, now.UnixNano()),
		BotID:       botID,
		Kind:        "training_memory",
		Name:        fmt.Sprintf("%s Training Memory iNFT", name),
		Description: "A user-owned AI memory asset built from raw training conversations.",
		Filename:    "inft/training-memory.json",
		ContentType: "application/json",
		Content:     string(raw),
		SizeBytes:   len(raw),
		SampleCount: len(samples),
		SampleIDs:   sampleIDs,
		Source:      "training",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return s.store.SaveINFT(botID, inft)
}

func (s *ChatService) CreateDistilledINFT(botID string, memorySummary string) (domain.INFTAsset, error) {
	bot, err := s.store.GetBot(botID)
	if err != nil {
		return domain.INFTAsset{}, err
	}
	memorySummary = strings.TrimSpace(memorySummary)
	if memorySummary == "" {
		return domain.INFTAsset{}, errors.New("missing distilled memory summary")
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

	sampleIDs := collectTrainingSampleIDs(samples)
	payload := map[string]any{
		"standard":    "0g-agent-inft/v1",
		"assetType":   "iNFT",
		"kind":        "distilled_memory",
		"name":        fmt.Sprintf("%s Distilled Memory iNFT", name),
		"description": "A user-owned distilled AI memory asset summarizing long-term agent memory.",
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
		ID:          fmt.Sprintf("%s-inft-%d", botID, now.UnixNano()),
		BotID:       botID,
		Kind:        "distilled_memory",
		Name:        fmt.Sprintf("%s Distilled Memory iNFT", name),
		Description: "A user-owned distilled AI memory asset summarizing long-term agent memory.",
		Filename:    "inft/distilled-memory.json",
		ContentType: "application/json",
		Content:     string(raw),
		SizeBytes:   len(raw),
		SampleCount: len(samples),
		SampleIDs:   sampleIDs,
		Source:      "distillation",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return s.store.SaveINFT(botID, inft)
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

	if len(zgsNodes) == 0 {
		nodes, err := discoverZgsNodesViaIndexer(ctx)
		if err != nil {
			return domain.INFTAsset{}, err
		}
		zgsNodes = nodes
	}

	asset, err := s.store.GetINFT(botID, inftID)
	if err != nil {
		return domain.INFTAsset{}, err
	}

	_, err = s.finalizeAssetPublish(ctx, botID, publishID, txHash, rootHash, zgsNodes, func(root common.Hash, txHash, ref string) error {
		infts, err := s.store.ListINFTs(botID)
		if err != nil {
			return err
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
			return errors.New("inft not found")
		}
		return s.store.UpdateINFTs(botID, infts)
	})
	if err != nil {
		return domain.INFTAsset{}, err
	}
	return asset, nil
}
