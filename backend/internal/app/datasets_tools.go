package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"
	"ai-bot-chain/backend/internal/service"
	"ai-bot-chain/backend/internal/store"
)

func (s *Server) handleDatasetsTools(w http.ResponseWriter, r *http.Request, botID string, rest []string) {
	if len(rest) == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if rest[0] == "export_skills" {
		s.handleDatasetsExportSkills(w, r, botID)
		return
	}

	if rest[0] == "distill" {
		if len(rest) == 1 {
			s.handleDatasetsDistill(w, r, botID)
			return
		}
		if len(rest) == 2 && rest[1] == "save" {
			s.handleDatasetsDistillSave(w, r, botID)
			return
		}
	}

	writeError(w, http.StatusNotFound, "not found")
}

func (s *Server) handleDatasetsDistill(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var input struct {
		SampleIDs  []string `json:"sampleIds"`
		Skills     []string `json:"skills"`
		MaxSamples int      `json:"maxSamples"`
	}
	if err := decodeOptionalJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Second)
	defer cancel()

	out, err := s.service.DistillTrainingMemories(ctx, botID, input.SampleIDs, input.Skills, input.MaxSamples)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrBotNotFound):
			handleStoreError(w, err)
		case errors.Is(err, service.ErrNoTrainingSamplesToDistill):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrZeroGComputeNotConfigured):
			writeError(w, http.StatusServiceUnavailable, err.Error())
		case strings.Contains(strings.ToLower(err.Error()), "invalid distillation json"):
			writeError(w, http.StatusBadGateway, err.Error())
		case strings.Contains(strings.ToLower(err.Error()), "0g compute"):
			writeError(w, http.StatusBadGateway, err.Error())
		default:
			handleStoreError(w, err)
		}
		return
	}

	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDatasetsDistillSave(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var input struct {
		Skills []domain.DistilledSkillDraft `json:"skills"`
	}
	if err := decodeOptionalJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := s.service.SaveDistilledSkills(botID, input.Skills)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrBotNotFound):
			handleStoreError(w, err)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"count":   len(created),
		"created": created,
	})
}
