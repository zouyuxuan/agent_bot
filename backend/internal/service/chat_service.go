package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"ai-bot-chain/backend/internal/domain"
	"ai-bot-chain/backend/internal/llm"
	"ai-bot-chain/backend/internal/store"
	"ai-bot-chain/backend/internal/zerog"

	"github.com/ethereum/go-ethereum/common"
)

type ChatService struct {
	store  store.Store
	pubs   store.PublishLog
	client *zerog.Client
	llm    *llm.OpenAICompatClient

	pendingMu sync.Mutex
	pending   map[string]walletPublishSnapshot
}

func NewChatService(st store.Store, pubs store.PublishLog, client *zerog.Client, llmClient *llm.OpenAICompatClient) *ChatService {
	if pubs == nil {
		pubs = store.NewNoopPublishLog()
	}
	return &ChatService{
		store:   st,
		pubs:    pubs,
		client:  client,
		llm:     llmClient,
		pending: make(map[string]walletPublishSnapshot),
	}
}

func (s *ChatService) UpsertBot(profile domain.BotProfile) (domain.BotProfile, error) {
	now := time.Now()
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now
	return s.store.SaveBot(profile)
}

func (s *ChatService) ListBots() []domain.BotProfile {
	return s.store.ListBots()
}

func (s *ChatService) GetBot(id string) (domain.BotProfile, error) {
	return s.store.GetBot(id)
}

func (s *ChatService) Chat(ctx context.Context, botID, message string, llmCfg *llm.Config, skillIDs []string, x402Results []domain.X402ToolResult, transferResults []domain.TransferToolResult) (domain.ConversationTurn, domain.BotProfile, bool, error) {
	bot, err := s.store.GetBot(botID)
	if err != nil {
		return domain.ConversationTurn{}, domain.BotProfile{}, false, err
	}

	skillCtx, err := s.buildSkillContext(botID, skillIDs)
	if err != nil {
		return domain.ConversationTurn{}, domain.BotProfile{}, false, err
	}

	now := time.Now()
	x402Ctx := buildX402ContextFromFrontend(x402Results)
	transferCtx := buildTransferContextFromFrontend(transferResults)
	reply, llmUsed := buildReplyFromFrontendToolResults(x402Results, transferResults)
	if strings.TrimSpace(reply) == "" {
		reply, llmUsed = s.generateReply(ctx, bot, message, llmCfg, skillCtx, x402Ctx, transferCtx)
	}
	turn := domain.ConversationTurn{
		UserMessage: domain.ChatMessage{
			Role:      "user",
			Content:   message,
			Timestamp: now,
		},
		AssistantMessage: domain.ChatMessage{
			Role:      "assistant",
			Content:   reply,
			Timestamp: now,
		},
	}

	if err := s.store.AppendMemory(botID, turn); err != nil {
		return domain.ConversationTurn{}, domain.BotProfile{}, false, err
	}

	bot.GrowthScore += 1
	bot.UpdatedAt = now
	bot, err = s.store.SaveBot(bot)
	if err != nil {
		return domain.ConversationTurn{}, domain.BotProfile{}, false, err
	}

	sample := domain.TrainingSample{
		ID:        fmt.Sprintf("%s-%d", botID, now.UnixNano()),
		BotID:     botID,
		Summary:   summarizeTurn(message, reply),
		Turns:     []domain.ConversationTurn{turn},
		Tags:      inferTags(bot, message),
		CreatedAt: now,
	}
	if err := s.store.SaveTrainingSample(botID, sample); err != nil {
		return domain.ConversationTurn{}, domain.BotProfile{}, false, err
	}

	return turn, bot, llmUsed, nil
}

func (s *ChatService) ListMemories(botID string) ([]domain.ConversationTurn, error) {
	return s.store.ListMemories(botID)
}

func (s *ChatService) ListTrainingSamples(botID string) ([]domain.TrainingSample, error) {
	return s.store.ListTrainingSamples(botID)
}

func (s *ChatService) ListSkills(botID string) ([]domain.Skill, error) {
	return s.store.ListSkills(botID)
}

func (s *ChatService) GetSkill(botID, skillID string) (domain.Skill, error) {
	return s.store.GetSkill(botID, skillID)
}

func (s *ChatService) CreateSkill(botID string, in domain.Skill) (domain.Skill, error) {
	now := time.Now()
	in.BotID = botID
	if strings.TrimSpace(in.ID) == "" {
		in.ID = fmt.Sprintf("%s-skill-%d", botID, now.UnixNano())
	}
	in.CreatedAt = now
	in.UpdatedAt = now
	return s.store.SaveSkill(botID, in)
}

func (s *ChatService) DeleteSkills(botID string, skillIDs []string) (int, error) {
	current, err := s.store.ListSkills(botID)
	if err != nil {
		return 0, err
	}
	if len(skillIDs) == 0 {
		return 0, nil
	}

	idSet := make(map[string]struct{}, len(skillIDs))
	for _, id := range skillIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		idSet[id] = struct{}{}
	}
	if len(idSet) == 0 {
		return 0, nil
	}

	next := make([]domain.Skill, 0, len(current))
	deleted := 0
	for _, sk := range current {
		if _, ok := idSet[strings.TrimSpace(sk.ID)]; ok {
			deleted++
			continue
		}
		next = append(next, sk)
	}
	if deleted == 0 {
		return 0, nil
	}
	if err := s.store.UpdateSkills(botID, next); err != nil {
		return 0, err
	}
	return deleted, nil
}

func (s *ChatService) PublishTrainingData(ctx context.Context, botID string, zgsNodesOverride []string) (domain.PublishResult, error) {
	samples, err := s.store.ListTrainingSamples(botID)
	if err != nil {
		return domain.PublishResult{}, err
	}

	payload, err := json.MarshalIndent(map[string]any{
		"botId":         botID,
		"exportedAt":    time.Now(),
		"trainingSet":   samples,
		"sampleCount":   len(samples),
		"storageTarget": "0G",
	}, "", "  ")
	if err != nil {
		return domain.PublishResult{}, err
	}

	info, err := s.client.StoreTrainingPayload(ctx, payload, zgsNodesOverride)
	if err != nil {
		return domain.PublishResult{}, err
	}

	publishedAt := time.Now()
	for i := range samples {
		applyTrainingSamplePublishState(&samples[i], trainingSamplePublishState{
			StoredOn0G:      true,
			StorageRef:      info.Reference,
			TxHash:          info.TxHash,
			RootHash:        info.RootHash,
			ExplorerTxURL:   info.ExplorerTxURL,
			UploadPending:   false,
			UploadCompleted: true,
			PublishedAt:     publishedAt,
		})
	}
	if err := s.store.UpdateTrainingSamples(botID, samples); err != nil {
		return domain.PublishResult{}, err
	}

	out := domain.PublishResult{
		BotID:            botID,
		SampleCount:      len(samples),
		StorageReference: info.Reference,
		Mode:             info.Mode,
		TxHash:           info.TxHash,
		RootHash:         info.RootHash,
		ExplorerTxURL:    info.ExplorerTxURL,
		IndexerRPC:       info.IndexerRPC,
		EvmRPC:           info.EvmRPC,
		FileLocations:    info.FileLocations,
		PublishedAt:      publishedAt,
	}

	if err := s.pubs.SaveTrainingPublish(ctx, out); err != nil {
		return domain.PublishResult{}, err
	}

	return out, nil
}

func (s *ChatService) generateReply(ctx context.Context, bot domain.BotProfile, message string, llmCfg *llm.Config, skillContext string, x402Context string, transferContext string) (string, bool) {
	if llmCfg != nil && s.llm != nil && strings.TrimSpace(llmCfg.APIKey) != "" {
		model := strings.TrimSpace(llmCfg.Model)
		if model == "" {
			model = strings.TrimSpace(bot.ModelType)
		}
		if model != "" {
			cfg := *llmCfg
			cfg.Provider = llm.ProviderOpenAICompat
			cfg.Model = model

			system := buildSystemPrompt(bot)
			if strings.TrimSpace(skillContext) != "" {
				system = system + "\n\n已启用 Skills（按需使用，不要逐字照搬）：\n" + skillContext
			}
			if strings.TrimSpace(x402Context) != "" {
				system = system + "\n\nx402 工具返回（可作为事实依据引用，但不要编造不存在的字段）：\n" + x402Context
			}
			if strings.TrimSpace(transferContext) != "" {
				system = system + "\n\n转账工具返回（仅引用已执行结果，不要编造链上状态）：\n" + transferContext
			}
			messages := []llm.Message{{Role: "system", Content: system}}

			if memories, err := s.store.ListMemories(bot.ID); err == nil {
				for _, m := range tailTurns(memories, 8) {
					messages = append(messages, llm.Message{Role: "user", Content: m.UserMessage.Content})
					messages = append(messages, llm.Message{Role: "assistant", Content: m.AssistantMessage.Content})
				}
			}
			messages = append(messages, llm.Message{Role: "user", Content: message})

			if content, err := s.llm.Chat(ctx, cfg, messages); err == nil {
				return content, true
			}
		}
	}

	personality := bot.Personality
	if personality == "" {
		personality = "温和、善于倾听"
	}
	model := bot.ModelType
	if model == "" {
		model = "general-companion"
	}

	var promptBuilder strings.Builder
	promptBuilder.WriteString("我是 ")
	promptBuilder.WriteString(bot.Name)
	if bot.Name == "" {
		promptBuilder.WriteString("你的 AI 伙伴")
	}
	promptBuilder.WriteString("。")
	promptBuilder.WriteString("我会以")
	promptBuilder.WriteString(personality)
	promptBuilder.WriteString("的风格回应你。")
	promptBuilder.WriteString("当前成长模型：")
	promptBuilder.WriteString(model)
	promptBuilder.WriteString("。")
	promptBuilder.WriteString("你刚才说的是：")
	promptBuilder.WriteString(message)
	promptBuilder.WriteString("。")
	promptBuilder.WriteString("基于我们过去的互动，我会持续学习你的偏好，并把高质量对话沉淀为训练样本。")
	if strings.TrimSpace(skillContext) != "" {
		promptBuilder.WriteString("我已启用以下 Skills，可以在回答时参考：\n")
		promptBuilder.WriteString(skillContext)
	}
	if strings.TrimSpace(x402Context) != "" {
		promptBuilder.WriteString("\n\nx402 工具返回（可作为事实依据引用）：\n")
		promptBuilder.WriteString(x402Context)
	}
	if strings.TrimSpace(transferContext) != "" {
		promptBuilder.WriteString("\n\n转账工具返回（仅引用已执行结果）：\n")
		promptBuilder.WriteString(transferContext)
	}
	return promptBuilder.String(), false
}

// ResolvePublishID finds the latest prepare snapshot ID for (botID, wallet, rootHash).
// This is a backward-compatibility path for older frontends that don't send publishId.
func (s *ChatService) ResolvePublishID(botID, walletAddr, rootHash string) (string, error) {
	if strings.TrimSpace(botID) == "" || strings.TrimSpace(walletAddr) == "" || strings.TrimSpace(rootHash) == "" {
		return "", fmt.Errorf("missing botID/walletAddr/rootHash")
	}
	root := common.HexToHash(rootHash)
	if root == (common.Hash{}) {
		return "", fmt.Errorf("invalid rootHash")
	}
	return s.resolveSnapshotID(botID, walletAddr, root, "training", 15*time.Minute)
}

func (s *ChatService) buildSkillContext(botID string, skillIDs []string) (string, error) {
	if len(skillIDs) == 0 {
		return "", nil
	}
	var b strings.Builder
	for _, id := range skillIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		sk, err := s.store.GetSkill(botID, id)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(sk.Content) == "" {
			continue
		}

		// Keep x402 tool specs out of the plain-text skill context; they are executed in the frontend.
		if looksLikeX402SkillContent(sk.Content) || looksLikeTransferSkillContent(sk.Content) {
			continue
		}

		b.WriteString("### ")
		if strings.TrimSpace(sk.Name) != "" {
			b.WriteString(strings.TrimSpace(sk.Name))
		} else {
			b.WriteString(id)
		}
		if strings.TrimSpace(sk.Filename) != "" {
			b.WriteString(" (")
			b.WriteString(strings.TrimSpace(sk.Filename))
			b.WriteString(")")
		}
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(sk.Content))
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String()), nil
}

func buildSystemPrompt(bot domain.BotProfile) string {
	var b strings.Builder
	if strings.TrimSpace(bot.SystemPrompt) != "" {
		b.WriteString(strings.TrimSpace(bot.SystemPrompt))
		b.WriteString("\n")
	}
	if strings.TrimSpace(bot.Name) != "" {
		b.WriteString("你的名字是：")
		b.WriteString(strings.TrimSpace(bot.Name))
		b.WriteString("。\n")
	}
	if strings.TrimSpace(bot.Personality) != "" {
		b.WriteString("你的性格：")
		b.WriteString(strings.TrimSpace(bot.Personality))
		b.WriteString("。\n")
	}
	if strings.TrimSpace(bot.Gender) != "" {
		b.WriteString("你的性别设定：")
		b.WriteString(strings.TrimSpace(bot.Gender))
		b.WriteString("。\n")
	}
	b.WriteString("你是一个在线聊天机器人，需要自然、连贯地与用户交流。回答尽量简洁直接，必要时可以追问澄清。")
	return b.String()
}

func tailTurns(turns []domain.ConversationTurn, n int) []domain.ConversationTurn {
	if n <= 0 || len(turns) == 0 {
		return nil
	}
	if len(turns) <= n {
		return turns
	}
	return turns[len(turns)-n:]
}

func summarizeTurn(message, reply string) string {
	return fmt.Sprintf("用户提问：%s；机器人回应：%s", truncate(message, 32), truncate(reply, 48))
}

func inferTags(bot domain.BotProfile, message string) []string {
	tags := []string{"chat", bot.ModelType}
	if bot.Personality != "" {
		tags = append(tags, "persona")
	}
	if strings.Contains(message, "?") || strings.Contains(message, "？") {
		tags = append(tags, "question")
	}
	return tags
}

func truncate(value string, limit int) string {
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}
