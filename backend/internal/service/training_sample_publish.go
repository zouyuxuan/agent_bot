package service

import (
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"
)

type trainingSamplePublishState struct {
	StoredOn0G      bool
	StorageRef      string
	TxHash          string
	RootHash        string
	ExplorerTxURL   string
	UploadPending   bool
	UploadCompleted bool
	PublishedAt     time.Time
}

func applyTrainingSamplePublishState(sample *domain.TrainingSample, state trainingSamplePublishState) {
	if sample == nil {
		return
	}
	sample.StoredOn0G = state.StoredOn0G
	if v := strings.TrimSpace(state.StorageRef); v != "" {
		sample.StorageRef = v
	}
	if v := strings.TrimSpace(state.TxHash); v != "" {
		sample.TxHash = v
	}
	if v := strings.TrimSpace(state.RootHash); v != "" {
		sample.RootHash = v
	}
	if v := strings.TrimSpace(state.ExplorerTxURL); v != "" {
		sample.ExplorerTxURL = v
	}
	sample.UploadPending = state.UploadPending
	sample.UploadCompleted = state.UploadCompleted
	if !state.PublishedAt.IsZero() {
		sample.PublishedAt = state.PublishedAt
	}
}

func (s *ChatService) updateTrainingSamplesByID(botID string, sampleIDs []string, apply func(*domain.TrainingSample)) error {
	if apply == nil {
		return nil
	}
	current, err := s.store.ListTrainingSamples(botID)
	if err != nil {
		return err
	}
	if len(current) == 0 {
		return nil
	}

	idSet := make(map[string]struct{}, len(sampleIDs))
	for _, id := range sampleIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		idSet[id] = struct{}{}
	}

	for i := range current {
		if len(idSet) > 0 {
			if _, ok := idSet[strings.TrimSpace(current[i].ID)]; !ok {
				continue
			}
		}
		apply(&current[i])
	}
	return s.store.UpdateTrainingSamples(botID, current)
}
