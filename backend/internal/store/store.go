package store

import "ai-bot-chain/backend/internal/domain"

// Store is the persistence interface for bots, memories, and datasets.
// Implementations must be concurrency-safe.
type Store interface {
	SaveBot(bot domain.BotProfile) (domain.BotProfile, error)
	ListBots() []domain.BotProfile
	GetBot(id string) (domain.BotProfile, error)

	AppendMemory(botID string, turn domain.ConversationTurn) error
	ListMemories(botID string) ([]domain.ConversationTurn, error)

	SaveTrainingSample(botID string, sample domain.TrainingSample) error
	UpdateTrainingSamples(botID string, samples []domain.TrainingSample) error
	ListTrainingSamples(botID string) ([]domain.TrainingSample, error)

	SaveSkill(botID string, skill domain.Skill) (domain.Skill, error)
	GetSkill(botID, skillID string) (domain.Skill, error)
	ListSkills(botID string) ([]domain.Skill, error)
	UpdateSkills(botID string, skills []domain.Skill) error

	SaveINFT(botID string, inft domain.INFTAsset) (domain.INFTAsset, error)
	GetINFT(botID, inftID string) (domain.INFTAsset, error)
	ListINFTs(botID string) ([]domain.INFTAsset, error)
	UpdateINFTs(botID string, infts []domain.INFTAsset) error
}
