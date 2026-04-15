package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"
)

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request, botID string, rest []string) {
	// /api/bots/{botID}/skills
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			skills, err := s.service.ListSkills(botID)
			if err != nil {
				handleStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, skills)
			return
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
	}

	// /api/bots/{botID}/skills/upload
	if rest[0] == "upload" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleSkillUpload(w, r, botID)
		return
	}

	// /api/bots/{botID}/skills/import_github
	if rest[0] == "import_github" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if _, ok := s.requireWallet(w, r); !ok {
			return
		}
		var input struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		out, err := s.service.ImportSkillsFromGitHub(ctx, botID, input.URL)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
		return
	}

	// /api/bots/{botID}/skills/publish_bundle_prepare|publish_bundle_finalize
	if rest[0] == "publish_bundle_prepare" || rest[0] == "publish_bundle_finalize" {
		switch rest[0] {
		case "publish_bundle_prepare":
			s.handleSkillsBundlePublishPrepare(w, r, botID)
		case "publish_bundle_finalize":
			s.handleSkillsBundlePublishFinalize(w, r, botID)
		}
		return
	}

	// /api/bots/{botID}/skills/{skillID}/publish_prepare|publish_finalize
	if len(rest) == 2 && (rest[1] == "publish_prepare" || rest[1] == "publish_finalize") {
		switch rest[1] {
		case "publish_prepare":
			s.handleSkillPublishPrepare(w, r, botID, rest[0])
		case "publish_finalize":
			s.handleSkillPublishFinalize(w, r, botID, rest[0])
		}
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

func (s *Server) handleSkillsBundlePublishPrepare(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	walletAddr, ok := s.requireWallet(w, r)
	if !ok {
		return
	}
	var input struct {
		ZgsNodes string   `json:"zgsNodes"`
		SkillIDs []string `json:"skillIds"`
	}
	if r.Body != nil && strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}
	nodes := parseCSV(input.ZgsNodes)

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	out, count, err := s.service.PrepareSkillsBundlePublish(ctx, botID, walletAddr, input.SkillIDs, nodes)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	// include count so UI can confirm what's being published
	writeJSON(w, http.StatusOK, map[string]any{
		"publishId": out.PublishID,
		"chainIdHex": out.ChainIDHex,
		"chainIdDec": out.ChainIDDec,
		"from": out.From,
		"to": out.To,
		"data": out.Data,
		"value": out.Value,
		"rootHash": out.RootHash,
		"skillCount": count,
	})
}

func (s *Server) handleSkillsBundlePublishFinalize(w http.ResponseWriter, r *http.Request, botID string) {
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

	result, err := s.service.FinalizeSkillsBundlePublish(r.Context(), botID, input.PublishID, input.TxHash, input.RootHash, nodes)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSkillUpload(w http.ResponseWriter, r *http.Request, botID string) {
	const maxBytes = 20 << 20 // 20MB (folder upload can contain multiple files)
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	// New: folder upload uses "files" (multiple). Old clients use "file" (single).
	var headers []*multipart.FileHeader
	if r.MultipartForm != nil && len(r.MultipartForm.File["files"]) > 0 {
		headers = append(headers, r.MultipartForm.File["files"]...)
	} else if r.MultipartForm != nil && len(r.MultipartForm.File["file"]) > 0 {
		// Backward compatibility.
		headers = append(headers, r.MultipartForm.File["file"]...)
	}
	if len(headers) == 0 {
		writeError(w, http.StatusBadRequest, "missing files")
		return
	}

	created := make([]domain.Skill, 0, len(headers))
	seen := map[string]int{}

	for _, hdr := range headers {
		if hdr == nil {
			continue
		}
		filename := strings.TrimSpace(hdr.Filename)
		if filename == "" {
			continue
		}
		// Ignore obvious system junk.
		base := filepath.Base(filename)
		if strings.HasPrefix(base, ".") || base == "" {
			continue
		}

		ext := strings.ToLower(filepath.Ext(filename))
		if ext == "" {
			ext = ".txt"
		}
		if ext != ".txt" && ext != ".md" && ext != ".json" && ext != ".yaml" && ext != ".yml" {
			writeError(w, http.StatusBadRequest, "unsupported skill file type: "+filename+" (use .txt/.md/.json/.yml)")
			return
		}

		f, err := hdr.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to open file: "+filename)
			return
		}
		raw, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read file: "+filename)
			return
		}
		content := strings.TrimSpace(string(raw))
		if content == "" {
			// Skip empty files silently; folders often contain README stubs or blanks.
			continue
		}

		// Use path-without-ext as default name to preserve folder structure in UI (and keep names stable).
		name := strings.TrimSpace(strings.TrimSuffix(filename, filepath.Ext(filename)))
		if name == "" {
			name = strings.TrimSuffix(base, filepath.Ext(base))
		}
		if name == "" {
			name = "skill-" + time.Now().Format("20060102150405")
		}
		if n := seen[name]; n > 0 {
			seen[name] = n + 1
			name = name + "-" + fmt.Sprintf("%d", n+1)
		} else {
			seen[name] = 1
		}

		skill, err := s.service.CreateSkill(botID, domain.Skill{
			Name:        name,
			Filename:    filename,
			ContentType: hdr.Header.Get("Content-Type"),
			Content:     content,
			SizeBytes:   len(raw),
		})
		if err != nil {
			handleStoreError(w, err)
			return
		}
		created = append(created, skill)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"created": created,
		"count":   len(created),
	})
}

func (s *Server) handleSkillPublishPrepare(w http.ResponseWriter, r *http.Request, botID, skillID string) {
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
	if r.Body != nil && strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}
	nodes := parseCSV(input.ZgsNodes)

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	out, err := s.service.PrepareSkillPublish(ctx, botID, skillID, walletAddr, nodes)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSkillPublishFinalize(w http.ResponseWriter, r *http.Request, botID, skillID string) {
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

	result, err := s.service.FinalizeSkillPublish(r.Context(), botID, skillID, input.PublishID, input.TxHash, input.RootHash, nodes)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
