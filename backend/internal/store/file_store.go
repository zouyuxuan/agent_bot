package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ai-bot-chain/backend/internal/domain"
)

type FileStore struct {
	mu   sync.RWMutex
	path string

	bots     map[string]domain.BotProfile
	memories map[string][]domain.ConversationTurn
	datasets map[string][]domain.TrainingSample
	skills   map[string][]domain.Skill
	infts    map[string][]domain.INFTAsset
}

type persistedState struct {
	Bots     map[string]domain.BotProfile         `json:"bots"`
	Memories map[string][]domain.ConversationTurn `json:"memories"`
	Datasets map[string][]domain.TrainingSample   `json:"datasets"`
	Skills   map[string][]domain.Skill            `json:"skills"`
	INFTs    map[string][]domain.INFTAsset        `json:"infts"`
}

func NewFileStore(path string) (*FileStore, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, errors.New("missing store path")
	}
	if !filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err == nil {
			p = abs
		}
	}

	fs := &FileStore{
		path:     p,
		bots:     make(map[string]domain.BotProfile),
		memories: make(map[string][]domain.ConversationTurn),
		datasets: make(map[string][]domain.TrainingSample),
		skills:   make(map[string][]domain.Skill),
		infts:    make(map[string][]domain.INFTAsset),
	}
	_ = fs.load()
	return fs, nil
}

func (s *FileStore) SaveBot(bot domain.BotProfile) (domain.BotProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bots[bot.ID] = bot
	if err := s.flushLocked(); err != nil {
		return domain.BotProfile{}, err
	}
	return bot, nil
}

func (s *FileStore) ListBots() []domain.BotProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.BotProfile, 0, len(s.bots))
	for _, bot := range s.bots {
		result = append(result, bot)
	}
	return result
}

func (s *FileStore) GetBot(id string) (domain.BotProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bot, ok := s.bots[id]
	if !ok {
		return domain.BotProfile{}, ErrBotNotFound
	}
	return bot, nil
}

func (s *FileStore) AppendMemory(botID string, turn domain.ConversationTurn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return ErrBotNotFound
	}
	s.memories[botID] = append(s.memories[botID], turn)
	return s.flushLocked()
}

func (s *FileStore) ListMemories(botID string) ([]domain.ConversationTurn, error) {
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

func (s *FileStore) SaveTrainingSample(botID string, sample domain.TrainingSample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return ErrBotNotFound
	}
	s.datasets[botID] = append(s.datasets[botID], sample)
	return s.flushLocked()
}

func (s *FileStore) UpdateTrainingSamples(botID string, samples []domain.TrainingSample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return ErrBotNotFound
	}
	s.datasets[botID] = samples
	return s.flushLocked()
}

func (s *FileStore) ListTrainingSamples(botID string) ([]domain.TrainingSample, error) {
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

func (s *FileStore) SaveSkill(botID string, skill domain.Skill) (domain.Skill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return domain.Skill{}, ErrBotNotFound
	}
	s.skills[botID] = append(s.skills[botID], skill)
	if err := s.flushLocked(); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}

func (s *FileStore) GetSkill(botID, skillID string) (domain.Skill, error) {
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

func (s *FileStore) ListSkills(botID string) ([]domain.Skill, error) {
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

func (s *FileStore) UpdateSkills(botID string, skills []domain.Skill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return ErrBotNotFound
	}
	s.skills[botID] = skills
	return s.flushLocked()
}

func (s *FileStore) SaveINFT(botID string, inft domain.INFTAsset) (domain.INFTAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return domain.INFTAsset{}, ErrBotNotFound
	}
	s.infts[botID] = append(s.infts[botID], inft)
	if err := s.flushLocked(); err != nil {
		return domain.INFTAsset{}, err
	}
	return inft, nil
}

func (s *FileStore) GetINFT(botID, inftID string) (domain.INFTAsset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.bots[botID]; !ok {
		return domain.INFTAsset{}, ErrBotNotFound
	}
	for _, inft := range s.infts[botID] {
		if inft.ID == inftID {
			return inft, nil
		}
	}
	return domain.INFTAsset{}, errors.New("inft not found")
}

func (s *FileStore) ListINFTs(botID string) ([]domain.INFTAsset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.bots[botID]; !ok {
		return nil, ErrBotNotFound
	}
	infts := s.infts[botID]
	result := make([]domain.INFTAsset, len(infts))
	copy(result, infts)
	return result, nil
}

func (s *FileStore) UpdateINFTs(botID string, infts []domain.INFTAsset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bots[botID]; !ok {
		return ErrBotNotFound
	}
	s.infts[botID] = infts
	return s.flushLocked()
}

func (s *FileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var st persistedState
	if err := json.NewDecoder(f).Decode(&st); err != nil {
		return err
	}
	if st.Bots != nil {
		s.bots = st.Bots
	}
	if st.Memories != nil {
		s.memories = st.Memories
	}
	if st.Datasets != nil {
		s.datasets = st.Datasets
	}
	if st.Skills != nil {
		s.skills = st.Skills
	}
	if st.INFTs != nil {
		s.infts = st.INFTs
	}
	return nil
}

func (s *FileStore) flushLocked() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(persistedState{
		Bots:     s.bots,
		Memories: s.memories,
		Datasets: s.datasets,
		Skills:   s.skills,
		INFTs:    s.infts,
	})
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, s.path)
}
