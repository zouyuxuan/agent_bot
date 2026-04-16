package store

import (
	"errors"
	"sync"

	"ai-bot-chain/backend/internal/domain"
)

var ErrBotNotFound = errors.New("bot not found")

type MemoryStore struct {
	mu       sync.RWMutex
	bots     map[string]domain.BotProfile
	memories map[string][]domain.ConversationTurn
	datasets map[string][]domain.TrainingSample
	skills   map[string][]domain.Skill
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		bots:     make(map[string]domain.BotProfile),
		memories: make(map[string][]domain.ConversationTurn),
		datasets: make(map[string][]domain.TrainingSample),
		skills:   make(map[string][]domain.Skill),
	}
}

func (s *MemoryStore) SaveBot(bot domain.BotProfile) (domain.BotProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bots[bot.ID] = bot
	return bot, nil
}

func (s *MemoryStore) ListBots() []domain.BotProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.BotProfile, 0, len(s.bots))
	for _, bot := range s.bots {
		result = append(result, bot)
	}
	return result
}

func (s *MemoryStore) GetBot(id string) (domain.BotProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bot, ok := s.bots[id]
	if !ok {
		return domain.BotProfile{}, ErrBotNotFound
	}
	return bot, nil
}

func (s *MemoryStore) AppendMemory(botID string, turn domain.ConversationTurn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return ErrBotNotFound
	}
	s.memories[botID] = append(s.memories[botID], turn)
	return nil
}

func (s *MemoryStore) ListMemories(botID string) ([]domain.ConversationTurn, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.bots[botID]; !ok {
		return nil, ErrBotNotFound
	}
	turns := s.memories[botID]
	result := make([]domain.ConversationTurn, len(turns))
	copy(result, turns)
	return result, nil
}

func (s *MemoryStore) SaveTrainingSample(botID string, sample domain.TrainingSample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return ErrBotNotFound
	}
	s.datasets[botID] = append(s.datasets[botID], sample)
	return nil
}

func (s *MemoryStore) UpdateTrainingSamples(botID string, samples []domain.TrainingSample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return ErrBotNotFound
	}
	s.datasets[botID] = samples
	return nil
}

func (s *MemoryStore) ListTrainingSamples(botID string) ([]domain.TrainingSample, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.bots[botID]; !ok {
		return nil, ErrBotNotFound
	}
	samples := s.datasets[botID]
	result := make([]domain.TrainingSample, len(samples))
	copy(result, samples)
	return result, nil
}

func (s *MemoryStore) SaveSkill(botID string, skill domain.Skill) (domain.Skill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return domain.Skill{}, ErrBotNotFound
	}
	s.skills[botID] = append(s.skills[botID], skill)
	return skill, nil
}

func (s *MemoryStore) GetSkill(botID, skillID string) (domain.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.bots[botID]; !ok {
		return domain.Skill{}, ErrBotNotFound
	}
	for _, sk := range s.skills[botID] {
		if sk.ID == skillID {
			return sk, nil
		}
	}
	return domain.Skill{}, errors.New("skill not found")
}

func (s *MemoryStore) ListSkills(botID string) ([]domain.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.bots[botID]; !ok {
		return nil, ErrBotNotFound
	}
	skills := s.skills[botID]
	result := make([]domain.Skill, len(skills))
	copy(result, skills)
	return result, nil
}

func (s *MemoryStore) UpdateSkills(botID string, skills []domain.Skill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return ErrBotNotFound
	}
	s.skills[botID] = skills
	return nil
}
