package domain

import "time"

type BotProfile struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Personality  string    `json:"personality"`
	Gender       string    `json:"gender"`
	AvatarURL    string    `json:"avatarUrl"`
	ModelType    string    `json:"modelType"`
	SystemPrompt string    `json:"systemPrompt"`
	GrowthScore  int       `json:"growthScore"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ChatMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type ConversationTurn struct {
	UserMessage      ChatMessage `json:"userMessage"`
	AssistantMessage ChatMessage `json:"assistantMessage"`
}

type TrainingSample struct {
	ID              string             `json:"id"`
	BotID           string             `json:"botId"`
	Summary         string             `json:"summary"`
	Turns           []ConversationTurn `json:"turns"`
	Tags            []string           `json:"tags"`
	StoredOn0G      bool               `json:"storedOn0G"`
	StorageRef      string             `json:"storageRef"`
	TxHash          string             `json:"txHash,omitempty"`
	RootHash        string             `json:"rootHash,omitempty"`
	ExplorerTxURL   string             `json:"explorerTxUrl,omitempty"`
	UploadPending   bool               `json:"uploadPending,omitempty"`
	UploadCompleted bool               `json:"uploadCompleted,omitempty"`
	PublishedAt     time.Time          `json:"publishedAt,omitempty"`
	CreatedAt       time.Time          `json:"createdAt"`
}

type Skill struct {
	ID          string `json:"id"`
	BotID       string `json:"botId"`
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
	SizeBytes   int    `json:"sizeBytes"`

	StoredOn0G bool   `json:"storedOn0G"`
	StorageRef string `json:"storageRef"`
	TxHash     string `json:"txHash,omitempty"`
	RootHash   string `json:"rootHash,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PublishResult struct {
	BotID            string    `json:"botId"`
	SampleCount      int       `json:"sampleCount"`
	StorageReference string    `json:"storageReference"`
	Mode             string    `json:"mode"`
	TxHash           string    `json:"txHash,omitempty"`
	RootHash         string    `json:"rootHash,omitempty"`
	ExplorerTxURL    string    `json:"explorerTxUrl,omitempty"`
	IndexerRPC       string    `json:"indexerRpc,omitempty"`
	EvmRPC           string    `json:"evmRpc,omitempty"`
	FileLocations    []string  `json:"fileLocations,omitempty"`
	TxMined          bool      `json:"txMined,omitempty"`
	TxSuccess        bool      `json:"txSuccess,omitempty"`
	UploadPending    bool      `json:"uploadPending,omitempty"`
	UploadCompleted  bool      `json:"uploadCompleted,omitempty"`
	PublishedAt      time.Time `json:"publishedAt"`
}

type DistilledSkillDraft struct {
	Name     string `json:"name"`
	Filename string `json:"filename,omitempty"`
	Content  string `json:"content"`
}

type MemoryDistillationResult struct {
	BotID           string                `json:"botId"`
	Source          string                `json:"source"`
	Model           string                `json:"model"`
	SampleCount     int                   `json:"sampleCount"`
	SampleIDs       []string              `json:"sampleIds,omitempty"`
	MemorySummary   string                `json:"memorySummary"`
	UserProfile     map[string]string     `json:"userProfile,omitempty"`
	StableRules     []string              `json:"stableRules,omitempty"`
	CandidateSkills []DistilledSkillDraft `json:"candidateSkills,omitempty"`
	GeneratedAt     time.Time             `json:"generatedAt"`
}

// X402ToolResult represents the result of a frontend-executed x402 paid HTTP call.
// It is sent to the backend along with a chat message so the backend can include it
// as tool context in the LLM prompt.
type X402ToolResult struct {
	SkillID   string            `json:"skillId"`
	Filename  string            `json:"filename,omitempty"`
	URL       string            `json:"url,omitempty"`
	Method    string            `json:"method,omitempty"`
	OK        bool              `json:"ok"`
	Status    int               `json:"status,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
	Error     string            `json:"error,omitempty"`
	StartedAt string            `json:"startedAt,omitempty"`
	EndedAt   string            `json:"endedAt,omitempty"`
}

// TransferToolResult represents the result of a frontend-executed wallet transfer.
// It is sent with chat requests so the assistant can reference transaction outcomes.
type TransferToolResult struct {
	SkillID   string `json:"skillId"`
	Type      string `json:"type,omitempty"` // native | erc20
	ChainID   int    `json:"chainId,omitempty"`
	To        string `json:"to,omitempty"`
	Token     string `json:"token,omitempty"`
	Amount    string `json:"amount,omitempty"`
	AmountWei string `json:"amountWei,omitempty"`
	TxHash    string `json:"txHash,omitempty"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	StartedAt string `json:"startedAt,omitempty"`
	EndedAt   string `json:"endedAt,omitempty"`
}
