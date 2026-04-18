package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"
	"ai-bot-chain/backend/internal/llm"
)

var (
	ErrZeroGComputeNotConfigured  = errors.New("0G Compute 未配置，请设置 ZERO_G_COMPUTE_API_KEY 和 ZERO_G_COMPUTE_SERVICE_URL")
	ErrNoTrainingSamplesToDistill = errors.New("no training samples to distill")
)

const (
	defaultDistillMaxSamples = 12
	maxDistillSamples        = 20
	defaultZeroGComputeModel = "THUDM/GLM-5-FP8"
)

func (s *ChatService) DistillTrainingMemories(ctx context.Context, botID string, sampleIDs []string, skillIDs []string, maxSamples int) (domain.MemoryDistillationResult, error) {
	bot, err := s.store.GetBot(botID)
	if err != nil {
		return domain.MemoryDistillationResult{}, err
	}
	samples, err := s.store.ListTrainingSamples(botID)
	if err != nil {
		return domain.MemoryDistillationResult{}, err
	}

	selected := selectTrainingSamplesForDistill(samples, sampleIDs, maxSamples)
	if len(selected) == 0 {
		return domain.MemoryDistillationResult{}, ErrNoTrainingSamplesToDistill
	}
	if s.llm == nil {
		return domain.MemoryDistillationResult{}, errors.New("0G Compute client unavailable")
	}

	cfg, err := zeroGComputeLLMConfigFromEnv()
	if err != nil {
		return domain.MemoryDistillationResult{}, err
	}

	skillContext, err := s.buildSkillContext(botID, skillIDs)
	if err != nil {
		return domain.MemoryDistillationResult{}, err
	}

	raw, err := s.llm.Chat(ctx, cfg, []llm.Message{
		{Role: "system", Content: buildMemoryDistillationSystemPrompt()},
		{Role: "user", Content: buildMemoryDistillationUserPrompt(bot, selected, skillContext)},
	})
	if err != nil {
		return domain.MemoryDistillationResult{}, fmt.Errorf("0G Compute distillation failed: %w", err)
	}

	out, err := parseMemoryDistillationResult(raw)
	if err != nil {
		return domain.MemoryDistillationResult{}, err
	}
	out.BotID = botID
	out.Source = "0G Compute"
	out.Model = strings.TrimSpace(cfg.Model)
	out.SampleCount = len(selected)
	out.SampleIDs = collectTrainingSampleIDs(selected)
	out.GeneratedAt = time.Now()
	return out, nil
}

func (s *ChatService) SaveDistilledSkills(botID string, drafts []domain.DistilledSkillDraft) ([]domain.Skill, error) {
	if _, err := s.store.GetBot(botID); err != nil {
		return nil, err
	}
	if len(drafts) == 0 {
		return nil, errors.New("missing distilled skills")
	}

	existing, err := s.store.ListSkills(botID)
	if err != nil {
		return nil, err
	}

	existingFilenames := make(map[string]struct{}, len(existing))
	existingNames := make(map[string]int, len(existing))
	for _, sk := range existing {
		if filename := normalizeDistilledSkillFilename(sk.Filename); filename != "" {
			existingFilenames[filename] = struct{}{}
		}
		if name := strings.TrimSpace(sk.Name); name != "" {
			existingNames[name]++
		}
	}

	seenFilenames := make(map[string]struct{})
	seenNames := make(map[string]int)
	created := make([]domain.Skill, 0, len(drafts))

	for i, draft := range drafts {
		content := strings.TrimSpace(draft.Content)
		if content == "" {
			continue
		}

		name := strings.TrimSpace(draft.Name)
		if name == "" {
			name = fmt.Sprintf("memory-distilled-skill-%02d", i+1)
		}
		name = makeUniqueDistilledSkillName(name, existingNames, seenNames)

		filename := normalizeDistilledSkillFilename(draft.Filename)
		if filename == "" {
			filename = "distilled/" + slugifyDistilledValue(name) + ".md"
		}
		filename = makeUniqueDistilledSkillFilename(filename, existingFilenames, seenFilenames)

		skill, err := s.CreateSkill(botID, domain.Skill{
			Name:        name,
			Filename:    filename,
			ContentType: "text/markdown",
			Content:     content,
			SizeBytes:   len([]byte(content)),
		})
		if err != nil {
			return nil, err
		}
		created = append(created, skill)
	}

	if len(created) == 0 {
		return nil, errors.New("no valid distilled skills to save")
	}
	return created, nil
}

func zeroGComputeLLMConfigFromEnv() (llm.Config, error) {
	apiKey := strings.TrimSpace(os.Getenv("ZERO_G_COMPUTE_API_KEY"))
	baseURL := strings.TrimSpace(os.Getenv("ZERO_G_COMPUTE_SERVICE_URL"))
	if apiKey == "" || baseURL == "" {
		return llm.Config{}, ErrZeroGComputeNotConfigured
	}

	baseURL = strings.TrimRight(baseURL, "/")
	switch {
	case strings.HasSuffix(baseURL, "/chat/completions"):
		// explicit endpoint, keep as-is
	case strings.Contains(baseURL, "/v1/proxy"):
		// proxy path already included
	case strings.HasSuffix(baseURL, "/v1") || strings.Contains(baseURL, "/v1/"):
		// already points to an OpenAI-compatible base
	default:
		baseURL = baseURL + "/v1/proxy"
	}

	model := strings.TrimSpace(os.Getenv("ZERO_G_COMPUTE_MODEL"))
	if model == "" {
		model = defaultZeroGComputeModel
	}

	timeoutMS := 90000
	if raw := strings.TrimSpace(os.Getenv("ZERO_G_COMPUTE_TIMEOUT_MS")); raw != "" {
		if n, err := parseInt(raw); err == nil && n > 0 {
			timeoutMS = n
		}
	}

	return llm.Config{
		Provider:    llm.ProviderOpenAICompat,
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       model,
		Temperature: 0.2,
		MaxTokens:   1800,
		TimeoutMS:   timeoutMS,
	}, nil
}

func selectTrainingSamplesForDistill(samples []domain.TrainingSample, sampleIDs []string, maxSamples int) []domain.TrainingSample {
	if len(samples) == 0 {
		return nil
	}
	if maxSamples <= 0 {
		maxSamples = defaultDistillMaxSamples
	}
	if maxSamples > maxDistillSamples {
		maxSamples = maxDistillSamples
	}

	if len(sampleIDs) > 0 {
		idSet := make(map[string]struct{}, len(sampleIDs))
		for _, id := range sampleIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			idSet[id] = struct{}{}
		}
		if len(idSet) > 0 {
			selected := make([]domain.TrainingSample, 0, len(idSet))
			for _, sample := range samples {
				if _, ok := idSet[strings.TrimSpace(sample.ID)]; ok {
					selected = append(selected, sample)
				}
			}
			if len(selected) > maxSamples {
				selected = selected[len(selected)-maxSamples:]
			}
			return selected
		}
	}

	if len(samples) <= maxSamples {
		return append([]domain.TrainingSample(nil), samples...)
	}
	return append([]domain.TrainingSample(nil), samples[len(samples)-maxSamples:]...)
}

func collectTrainingSampleIDs(samples []domain.TrainingSample) []string {
	out := make([]string, 0, len(samples))
	for _, sample := range samples {
		if id := strings.TrimSpace(sample.ID); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func buildMemoryDistillationSystemPrompt() string {
	return strings.TrimSpace(`
你是一个 AI Agent Memory Distillation Engine。
你的任务是把原始对话记忆蒸馏成结构化长期记忆与可复用 Skills。

你必须严格输出 JSON，对象字段如下：
{
  "memory_summary": "字符串，概括用户当前阶段目标、偏好与上下文",
  "user_profile": {
    "key": "value"
  },
  "stable_rules": ["规则1", "规则2"],
  "candidate_skills": [
    {
      "name": "Skill 名称",
      "filename": "distilled/skill-name.md",
      "content": "可直接保存为 Skill 的正文内容"
    }
  ]
}

约束：
- 只能输出 JSON，不要输出 markdown，不要输出解释文字。
- user_profile 的 value 必须是简短字符串，不要嵌套对象。
- stable_rules 输出 3 到 8 条长期稳定规则，必须可执行。
- candidate_skills 输出 1 到 4 个技能草稿。
- Skill content 必须是可复用的技能说明，适合在后续对话中被模型参考。
- 不要编造记忆中不存在的事实。
- 优先提炼长期偏好、表达风格、项目上下文、工作方式，而不是一次性细节。
`)
}

func buildMemoryDistillationUserPrompt(bot domain.BotProfile, samples []domain.TrainingSample, skillContext string) string {
	var b strings.Builder
	b.WriteString("机器人资料：\n")
	if v := strings.TrimSpace(bot.Name); v != "" {
		b.WriteString("name: ")
		b.WriteString(v)
		b.WriteString("\n")
	}
	if v := strings.TrimSpace(bot.Personality); v != "" {
		b.WriteString("personality: ")
		b.WriteString(v)
		b.WriteString("\n")
	}
	if v := strings.TrimSpace(bot.SystemPrompt); v != "" {
		b.WriteString("system_prompt: ")
		b.WriteString(truncate(v, 500))
		b.WriteString("\n")
	}

	b.WriteString("\n训练记忆样本：\n")
	for i, sample := range samples {
		b.WriteString("\n### sample ")
		b.WriteString(fmt.Sprintf("%d", i+1))
		b.WriteString("\n")
		if id := strings.TrimSpace(sample.ID); id != "" {
			b.WriteString("id: ")
			b.WriteString(id)
			b.WriteString("\n")
		}
		if summary := strings.TrimSpace(sample.Summary); summary != "" {
			b.WriteString("summary: ")
			b.WriteString(truncate(summary, 220))
			b.WriteString("\n")
		}
		if len(sample.Tags) > 0 {
			b.WriteString("tags: ")
			b.WriteString(strings.Join(sample.Tags, ", "))
			b.WriteString("\n")
		}
		for j, turn := range sample.Turns {
			b.WriteString("turn ")
			b.WriteString(fmt.Sprintf("%d", j+1))
			b.WriteString(" user: ")
			b.WriteString(truncate(strings.TrimSpace(turn.UserMessage.Content), 420))
			b.WriteString("\n")
			b.WriteString("turn ")
			b.WriteString(fmt.Sprintf("%d", j+1))
			b.WriteString(" assistant: ")
			b.WriteString(truncate(strings.TrimSpace(turn.AssistantMessage.Content), 420))
			b.WriteString("\n")
		}
	}

	if strings.TrimSpace(skillContext) != "" {
		b.WriteString("\n已选 Skills（蒸馏时必须一并参考）：\n")
		b.WriteString(skillContext)
		b.WriteString("\n")
	}

	b.WriteString("\n请基于这些记忆，输出一个适合长期 AI Agent 使用的 JSON distillation 结果。")
	return b.String()
}

func parseMemoryDistillationResult(raw string) (domain.MemoryDistillationResult, error) {
	payload := extractJSONObject(raw)
	if payload == "" {
		return domain.MemoryDistillationResult{}, errors.New("0G Compute returned no JSON object")
	}

	var parsed struct {
		MemorySummary   string                       `json:"memory_summary"`
		UserProfile     map[string]string            `json:"user_profile"`
		StableRules     []string                     `json:"stable_rules"`
		CandidateSkills []domain.DistilledSkillDraft `json:"candidate_skills"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return domain.MemoryDistillationResult{}, fmt.Errorf("invalid distillation json: %w", err)
	}

	out := domain.MemoryDistillationResult{
		MemorySummary:   strings.TrimSpace(parsed.MemorySummary),
		UserProfile:     normalizeUserProfile(parsed.UserProfile),
		StableRules:     normalizeStringList(parsed.StableRules, 8),
		CandidateSkills: normalizeDistilledDrafts(parsed.CandidateSkills),
	}

	if out.MemorySummary == "" {
		return domain.MemoryDistillationResult{}, errors.New("distillation result missing memory_summary")
	}
	if len(out.CandidateSkills) == 0 && len(out.StableRules) > 0 {
		out.CandidateSkills = []domain.DistilledSkillDraft{buildFallbackDistilledSkill(out.MemorySummary, out.StableRules)}
	}
	return out, nil
}

func extractJSONObject(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(s[start : end+1])
}

func normalizeUserProfile(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(v)
		if key == "" || val == "" {
			continue
		}
		out[key] = truncate(val, 180)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeStringList(in []string, limit int) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, truncate(item, 180))
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func normalizeDistilledDrafts(in []domain.DistilledSkillDraft) []domain.DistilledSkillDraft {
	out := make([]domain.DistilledSkillDraft, 0, len(in))
	for i, draft := range in {
		content := strings.TrimSpace(draft.Content)
		if content == "" {
			continue
		}
		name := strings.TrimSpace(draft.Name)
		if name == "" {
			name = fmt.Sprintf("memory-distilled-skill-%02d", i+1)
		}
		filename := normalizeDistilledSkillFilename(draft.Filename)
		if filename == "" {
			filename = "distilled/" + slugifyDistilledValue(name) + ".md"
		}
		out = append(out, domain.DistilledSkillDraft{
			Name:     truncate(name, 80),
			Filename: filename,
			Content:  content,
		})
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func buildFallbackDistilledSkill(summary string, rules []string) domain.DistilledSkillDraft {
	var b strings.Builder
	b.WriteString("# UserMemoryProfile\n\n")
	b.WriteString("这是一份从长期对话中蒸馏出的用户记忆画像。回答时应按需参考，不要逐字照搬。\n\n")
	b.WriteString("## Memory Summary\n")
	b.WriteString(strings.TrimSpace(summary))
	b.WriteString("\n\n## Stable Rules\n")
	for _, rule := range rules {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(rule))
		b.WriteString("\n")
	}
	return domain.DistilledSkillDraft{
		Name:     "UserMemoryProfile",
		Filename: "distilled/user-memory-profile.md",
		Content:  strings.TrimSpace(b.String()),
	}
}

func normalizeDistilledSkillFilename(filename string) string {
	name := strings.TrimSpace(filename)
	if name == "" {
		return ""
	}
	name = filepath.ToSlash(filepath.Clean(name))
	for strings.HasPrefix(name, "./") {
		name = strings.TrimPrefix(name, "./")
	}
	name = strings.TrimLeft(name, "/")
	if name == "." || name == "" {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		name += ".md"
	} else if ext != ".md" && ext != ".txt" && ext != ".json" && ext != ".yaml" && ext != ".yml" {
		name = strings.TrimSuffix(name, ext) + ".md"
	}
	return name
}

func makeUniqueDistilledSkillFilename(filename string, existing map[string]struct{}, batch map[string]struct{}) string {
	filename = normalizeDistilledSkillFilename(filename)
	if filename == "" {
		filename = "distilled/memory-distilled-skill.md"
	}

	dir := filepath.ToSlash(filepath.Dir(filename))
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	baseName := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ".md"
	}
	if dir == "." {
		dir = ""
	}

	candidate := filename
	for n := 2; ; n++ {
		if _, ok := existing[candidate]; !ok {
			if _, ok := batch[candidate]; !ok {
				batch[candidate] = struct{}{}
				return candidate
			}
		}
		next := fmt.Sprintf("%s-%d%s", baseName, n, ext)
		if dir != "" {
			candidate = dir + "/" + next
		} else {
			candidate = next
		}
	}
}

func makeUniqueDistilledSkillName(name string, existing map[string]int, batch map[string]int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "memory-distilled-skill"
	}
	base := name
	count := existing[base] + batch[base]
	if count == 0 {
		batch[base]++
		return base
	}
	for n := count + 1; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if existing[candidate] == 0 && batch[candidate] == 0 {
			batch[candidate]++
			return candidate
		}
	}
}

func slugifyDistilledValue(value string) string {
	s := strings.ToLower(strings.TrimSpace(value))
	if s == "" {
		return "memory-distilled-skill"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "memory-distilled-skill"
	}
	return out
}
