package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleINFTs(w http.ResponseWriter, r *http.Request, botID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			infts, err := s.service.ListINFTs(botID)
			if err != nil {
				handleStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, infts)
			return
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
	}

	if rest[0] == "create_training" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		inft, err := s.service.CreateTrainingINFT(botID)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "no training samples") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			handleStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, inft)
		return
	}

	if rest[0] == "create_distilled" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var input struct {
			MemorySummary string `json:"memorySummary"`
		}
		if err := decodeOptionalJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		inft, err := s.service.CreateDistilledINFT(botID, input.MemorySummary)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, inft)
		return
	}

	if len(rest) == 2 && (rest[1] == "publish_prepare" || rest[1] == "publish_finalize") {
		switch rest[1] {
		case "publish_prepare":
			s.handleINFTPublishPrepare(w, r, botID, rest[0])
		case "publish_finalize":
			s.handleINFTPublishFinalize(w, r, botID, rest[0])
		}
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

func (s *Server) handleINFTPublishPrepare(w http.ResponseWriter, r *http.Request, botID, inftID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	walletAddr, ok := s.requireWallet(w, r)
	if !ok {
		return
	}
	var input struct {
		ZgsNodes string `json:"zgsNodes"`
	}
	if err := decodeOptionalJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	nodes := parseCSV(input.ZgsNodes)

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	out, err := s.service.PrepareINFTPublish(ctx, botID, inftID, walletAddr, nodes)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "inft not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleINFTPublishFinalize(w http.ResponseWriter, r *http.Request, botID, inftID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	_, ok := s.requireWallet(w, r)
	if !ok {
		return
	}
	var input struct {
		ZgsNodes  string `json:"zgsNodes"`
		PublishID string `json:"publishId"`
		TxHash    string `json:"txHash"`
		RootHash  string `json:"rootHash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(input.PublishID) == "" || strings.TrimSpace(input.TxHash) == "" || strings.TrimSpace(input.RootHash) == "" {
		writeError(w, http.StatusBadRequest, "publishId, txHash and rootHash are required")
		return
	}
	nodes := parseCSV(input.ZgsNodes)

	inft, err := s.service.FinalizeINFTPublish(r.Context(), botID, inftID, input.PublishID, input.TxHash, input.RootHash, nodes)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "inft not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inft)
}
