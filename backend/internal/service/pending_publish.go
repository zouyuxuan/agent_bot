package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type walletPublishSnapshot struct {
	ID        string
	BotID     string
	Wallet    string
	Kind      string // "training" | "skill" | "skills_bundle"
	SkillID   string
	SkillIDs  []string
	Root      common.Hash
	Payload   []byte
	SampleIDs []string
	CreatedAt time.Time
}

func newSnapshotID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func (s *ChatService) putSnapshot(ss walletPublishSnapshot) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	s.pending[ss.ID] = ss
}

func (s *ChatService) getSnapshot(id string, maxAge time.Duration) (walletPublishSnapshot, error) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	ss, ok := s.pending[id]
	if !ok {
		return walletPublishSnapshot{}, errors.New("publish snapshot not found (please prepare again)")
	}
	if maxAge > 0 && time.Since(ss.CreatedAt) > maxAge {
		delete(s.pending, id)
		return walletPublishSnapshot{}, errors.New("publish snapshot expired (please prepare again)")
	}
	return ss, nil
}

func (s *ChatService) deleteSnapshot(id string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	delete(s.pending, id)
}

func (s *ChatService) resolveSnapshotID(botID, wallet string, root common.Hash, kind string, maxAge time.Duration) (string, error) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	var best walletPublishSnapshot
	found := false
	for _, ss := range s.pending {
		if ss.BotID != botID {
			continue
		}
		if ss.Wallet != wallet {
			continue
		}
		if strings.TrimSpace(kind) != "" && ss.Kind != kind {
			continue
		}
		if ss.Root != root {
			continue
		}
		if maxAge > 0 && time.Since(ss.CreatedAt) > maxAge {
			continue
		}
		if !found || ss.CreatedAt.After(best.CreatedAt) {
			best = ss
			found = true
		}
	}
	if !found {
		return "", errors.New("publish snapshot not found for this rootHash (please prepare again)")
	}
	return best.ID, nil
}
