package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/auth"
	"ai-bot-chain/backend/internal/domain"
	"ai-bot-chain/backend/internal/llm"
	"ai-bot-chain/backend/internal/service"
	"ai-bot-chain/backend/internal/store"
	"ai-bot-chain/backend/internal/zerog"

	"github.com/ethereum/go-ethereum/ethclient"
)

type Server struct {
	service *service.ChatService
	auth    *auth.Service
	mux     *http.ServeMux
}

func NewServer() (*Server, error) {
	st := store.Store(store.NewMemoryStore())
	if p := strings.TrimSpace(os.Getenv("STORE_PATH")); p != "" {
		if fs, err := store.NewFileStore(p); err == nil {
			st = fs
		} else {
			log.Printf("failed to init file store (%s): %v; falling back to memory store", p, err)
		}
	}

	pubs := store.NewNoopPublishLog()
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		pl, err := store.NewPgPublishLog(context.Background(), dsn)
		if err != nil {
			return nil, err
		}
		pubs = pl
	}

	svc := service.NewChatService(st, pubs, zerog.NewClientFromEnv(), llm.NewOpenAICompatClient())
	server := &Server{
		service: svc,
		auth:    auth.NewService(),
		mux:     http.NewServeMux(),
	}
	server.registerRoutes()
	return server, nil
}

func (s *Server) Routes() http.Handler {
	return withCORS(s.mux)
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/auth/nonce", s.handleAuthNonce)
	s.mux.HandleFunc("/api/auth/verify", s.handleAuthVerify)
	s.mux.HandleFunc("/api/zerog/config", s.handleZeroGConfig)
	s.mux.HandleFunc("/api/zerog/chaininfo", s.handleZeroGChainInfo)
	s.mux.HandleFunc("/api/zerog/tx_status", s.handleZeroGTxStatus)
	s.mux.HandleFunc("/api/x402/fetch", s.handleX402Proxy)
	s.mux.HandleFunc("/api/x402/proxy", s.handleX402ProxyFetch)
	s.mux.HandleFunc("/api/bots", s.handleBots)
	s.mux.HandleFunc("/api/bots/", s.handleBotRoutes)

	frontendDir := resolveFrontendDir()
	if stat, err := os.Stat(frontendDir); err == nil && stat.IsDir() {
		fileServer := http.FileServer(http.Dir(frontendDir))
		s.mux.Handle("/", fileServer)
	}
}

func (s *Server) handleZeroGConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Expose only non-secret config to help the frontend prefill.
	// ZERO_G_PRIVATE_KEY is never returned.
	writeJSON(w, http.StatusOK, map[string]any{
		"hasPrivateKey": os.Getenv("ZERO_G_PRIVATE_KEY") != "",
		"evmRpc":        os.Getenv("ZERO_G_EVM_RPC"),
		"indexerRpc":    os.Getenv("ZERO_G_INDEXER_RPC"),
		"explorerBase":  os.Getenv("ZERO_G_EXPLORER_BASE"),
		"zgsNodes":      strings.TrimSpace(os.Getenv("ZERO_G_ZGS_NODES")),
	})
}

func (s *Server) handleZeroGChainInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	evmRPC := strings.TrimSpace(os.Getenv("ZERO_G_EVM_RPC"))
	if evmRPC == "" {
		evmRPC = "https://evmrpc-testnet.0g.ai"
	}
	flowAddr := strings.TrimSpace(os.Getenv("ZERO_G_FLOW_CONTRACT_ADDRESS"))

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	ec, err := ethclient.DialContext(ctx, evmRPC)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	defer ec.Close()

	chainID, err := ec.ChainID(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"evmRpc":              evmRPC,
		"chainIdDec":          chainID.String(),
		"chainIdHex":          "0x" + chainID.Text(16),
		"flowContractAddress": flowAddr,
		"explorerBase":        strings.TrimSpace(os.Getenv("ZERO_G_EXPLORER_BASE")),
	})
}

func (s *Server) handleAuthNonce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	chainID := strings.TrimSpace(r.URL.Query().Get("chainId"))
	origin := requestOrigin(r)
	chal, err := s.auth.NewChallenge(address, chainID, origin)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, chal)
}

func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var input struct {
		Address   string `json:"address"`
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sess, err := s.auth.Verify(r.Context(), input.Address, input.Message, input.Signature)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func resolveFrontendDir() string {
	candidates := []string{"frontend", "../frontend"}
	for _, dir := range candidates {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return dir
		}
	}
	return filepath.Join(".", "frontend")
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) handleBots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.service.ListBots())
	case http.MethodPost:
		var input struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Personality  string `json:"personality"`
			Gender       string `json:"gender"`
			AvatarURL    string `json:"avatarUrl"`
			ModelType    string `json:"modelType"`
			SystemPrompt string `json:"systemPrompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if input.ID == "" {
			input.ID = strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
		}
		if input.ID == "" {
			input.ID = "bot-" + time.Now().Format("20060102150405")
		}
		bot := s.service.UpsertBot(domain.BotProfile{
			ID:           input.ID,
			Name:         input.Name,
			Personality:  input.Personality,
			Gender:       input.Gender,
			AvatarURL:    input.AvatarURL,
			ModelType:    input.ModelType,
			SystemPrompt: input.SystemPrompt,
		})
		writeJSON(w, http.StatusCreated, bot)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleBotRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/bots/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	botID := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		bot, err := s.service.GetBot(botID)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, bot)
		return
	}

	if len(parts) != 2 {
		// Allow nested routes such as /skills/{id}/publish_prepare
		if len(parts) >= 2 && parts[1] == "skills" {
			s.handleSkills(w, r, botID, parts[2:])
			return
		}
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	switch parts[1] {
	case "chat":
		s.handleChat(w, r, botID)
	case "memories":
		s.handleMemories(w, r, botID)
	case "datasets":
		s.handleDatasets(w, r, botID)
	case "publish":
		s.handlePublish(w, r, botID)
	case "publish_prepare":
		s.handlePublishPrepare(w, r, botID)
	case "publish_finalize":
		s.handlePublishFinalize(w, r, botID)
	case "skills":
		s.handleSkills(w, r, botID, nil)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) requireWallet(w http.ResponseWriter, r *http.Request) (string, bool) {
	addr, ok := s.auth.Authenticate(bearerToken(r.Header.Get("Authorization")))
	if !ok {
		writeError(w, http.StatusUnauthorized, "wallet authorization required (connect wallet first)")
		return "", false
	}
	return addr, true
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var input struct {
		Message string                  `json:"message"`
		LLM     *llm.Config             `json:"llm"`
		Skills  []string                `json:"skills"`
		X402    []domain.X402ToolResult `json:"x402"`
		Debug   *struct {
			SkillsUsed bool `json:"skillsUsed"`
		} `json:"debug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	turn, bot, llmUsed, err := s.service.Chat(r.Context(), botID, input.Message, input.LLM, input.Skills, input.X402)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	meta := map[string]any{
		"llmUsed": llmUsed,
	}
	if input.Debug != nil && input.Debug.SkillsUsed {
		meta["skillsUsed"] = s.buildSkillsUsed(botID, input.Skills)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"turn": turn,
		"bot":  bot,
		"meta": meta,
	})
}

func (s *Server) buildSkillsUsed(botID string, skillIDs []string) []map[string]any {
	out := make([]map[string]any, 0, len(skillIDs))
	seen := map[string]bool{}
	for _, id := range skillIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		sk, err := s.service.GetSkill(botID, id)
		if err != nil {
			continue
		}
		kind := "text"
		if looksLikeX402Skill(sk.Content) {
			kind = "x402_fetch"
		}
		out = append(out, map[string]any{
			"id":       sk.ID,
			"name":     sk.Name,
			"filename": sk.Filename,
			"kind":     kind,
		})
	}
	return out
}

func looksLikeX402Skill(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" || !strings.HasPrefix(content, "{") {
		return false
	}
	var v struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		return false
	}
	t := strings.ToLower(strings.TrimSpace(v.Type))
	return t == "x402_fetch" || t == "x402"
}

func (s *Server) handleMemories(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	memories, err := s.service.ListMemories(botID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func (s *Server) handleDatasets(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	samples, err := s.service.ListTrainingSamples(botID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	_, ok := s.requireWallet(w, r)
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

	result, err := s.service.PublishTrainingData(r.Context(), botID, nodes)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePublishPrepare(w http.ResponseWriter, r *http.Request, botID string) {
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
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	nodes := parseCSV(input.ZgsNodes)

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	out, err := s.service.PrepareWalletPublish(ctx, botID, walletAddr, nodes)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePublishFinalize(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	walletAddr, ok := s.requireWallet(w, r)
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
	nodes := parseCSV(input.ZgsNodes)
	if strings.TrimSpace(input.TxHash) == "" || strings.TrimSpace(input.RootHash) == "" {
		writeError(w, http.StatusBadRequest, "txHash and rootHash are required")
		return
	}
	if strings.TrimSpace(input.PublishID) == "" {
		// Backward compatible: older frontend omits publishId (JSON.stringify drops undefined).
		if id, err := s.service.ResolvePublishID(botID, walletAddr, input.RootHash); err == nil {
			input.PublishID = id
		} else {
			writeError(w, http.StatusBadRequest, "publishId is required (please prepare again)")
			return
		}
	}

	result, err := s.service.FinalizeWalletPublish(r.Context(), botID, input.PublishID, input.TxHash, input.RootHash, nodes)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrBotNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var idxDown *zerog.ErrIndexerUnavailable
	if errors.As(err, &idxDown) {
		writeError(w, http.StatusServiceUnavailable, idxDown.Error())
		return
	}
	var needNodes *zerog.ErrNeedZgsNodes
	if errors.As(err, &needNodes) {
		// Service is temporarily unavailable unless the client provides direct ZGS nodes.
		writeError(w, http.StatusServiceUnavailable, needNodes.Error())
		return
	}

	if strings.Contains(strings.ToLower(err.Error()), "not uploaded to 0g yet") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Treat transient network failures as 503 so the frontend can show a retryable error.
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "timed out") || strings.Contains(low, "timeout") || strings.Contains(low, "no reachable") {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	log.Printf("handler error: %v", err)
	if os.Getenv("APP_DEBUG") == "1" || strings.EqualFold(os.Getenv("APP_DEBUG"), "true") {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func bearerToken(header string) string {
	h := strings.TrimSpace(header)
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(h) >= len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
