package service

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"
)

func (s *ChatService) ExportTrainingSamplesAsSkills(botID string) ([]byte, string, int, error) {
	bot, err := s.store.GetBot(botID)
	if err != nil {
		return nil, "", 0, err
	}
	samples, err := s.store.ListTrainingSamples(botID)
	if err != nil {
		return nil, "", 0, err
	}
	if len(samples) == 0 {
		return nil, "", 0, errors.New("no training samples to export")
	}

	folderName := "training-skills-" + slugifySkillExportName(bot.ID)
	downloadName := folderName + ".zip"

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for i, sample := range samples {
		entryName := fmt.Sprintf("%s/%03d-%s.md", folderName, i+1, buildTrainingSkillFilename(sample, i+1))
		h := &zip.FileHeader{
			Name:   entryName,
			Method: zip.Deflate,
		}
		modTime := sample.CreatedAt
		if modTime.IsZero() {
			modTime = time.Now()
		}
		h.SetModTime(modTime)

		w, err := zw.CreateHeader(h)
		if err != nil {
			_ = zw.Close()
			return nil, "", 0, err
		}
		if _, err := w.Write([]byte(renderTrainingSampleAsSkill(bot, sample))); err != nil {
			_ = zw.Close()
			return nil, "", 0, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, "", 0, err
	}
	return buf.Bytes(), downloadName, len(samples), nil
}

func buildTrainingSkillFilename(sample domain.TrainingSample, index int) string {
	if out := slugifySkillExportName(sample.ID); out != "item" {
		return out
	}
	if out := slugifySkillExportName(sample.Summary); out != "item" {
		return out
	}
	return fmt.Sprintf("sample-%03d", index)
}

func renderTrainingSampleAsSkill(bot domain.BotProfile, sample domain.TrainingSample) string {
	var b strings.Builder
	title := strings.TrimSpace(sample.Summary)
	if title == "" {
		title = "训练样本导出 Skill"
	}
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("来源：用户训练数据导出为 Skill\n")
	if strings.TrimSpace(bot.Name) != "" {
		b.WriteString("机器人：")
		b.WriteString(strings.TrimSpace(bot.Name))
		if strings.TrimSpace(bot.ID) != "" {
			b.WriteString(" (")
			b.WriteString(strings.TrimSpace(bot.ID))
			b.WriteString(")")
		}
		b.WriteString("\n")
	} else if strings.TrimSpace(bot.ID) != "" {
		b.WriteString("机器人 ID：")
		b.WriteString(strings.TrimSpace(bot.ID))
		b.WriteString("\n")
	}
	if strings.TrimSpace(sample.ID) != "" {
		b.WriteString("训练样本 ID：")
		b.WriteString(strings.TrimSpace(sample.ID))
		b.WriteString("\n")
	}
	if !sample.CreatedAt.IsZero() {
		b.WriteString("创建时间：")
		b.WriteString(sample.CreatedAt.Format(time.RFC3339))
		b.WriteString("\n")
	}
	if len(sample.Tags) > 0 {
		b.WriteString("标签：")
		b.WriteString(strings.Join(sample.Tags, ", "))
		b.WriteString("\n")
	}
	if strings.TrimSpace(sample.StorageRef) != "" || strings.TrimSpace(sample.RootHash) != "" || strings.TrimSpace(sample.TxHash) != "" {
		b.WriteString("可验证记忆：是\n")
		if strings.TrimSpace(sample.StorageRef) != "" {
			b.WriteString("Storage Ref：")
			b.WriteString(strings.TrimSpace(sample.StorageRef))
			b.WriteString("\n")
		}
		if strings.TrimSpace(sample.RootHash) != "" {
			b.WriteString("Root Hash：")
			b.WriteString(strings.TrimSpace(sample.RootHash))
			b.WriteString("\n")
		}
		if strings.TrimSpace(sample.TxHash) != "" {
			b.WriteString("Tx Hash：")
			b.WriteString(strings.TrimSpace(sample.TxHash))
			b.WriteString("\n")
		}
		if strings.TrimSpace(sample.ExplorerTxURL) != "" {
			b.WriteString("Explorer：")
			b.WriteString(strings.TrimSpace(sample.ExplorerTxURL))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString("使用方式：当用户意图相似时，把下面的历史对话作为风格、偏好和回答策略参考，按需改写，不要逐字复述。\n")

	if len(sample.Turns) == 0 {
		return strings.TrimSpace(b.String()) + "\n"
	}

	for i, turn := range sample.Turns {
		b.WriteString("\n## 对话 ")
		b.WriteString(fmt.Sprintf("%d", i+1))
		b.WriteString("\n\n### 用户\n")
		user := strings.TrimSpace(turn.UserMessage.Content)
		if user == "" {
			user = "（空）"
		}
		b.WriteString(user)
		b.WriteString("\n\n### 助手\n")
		assistant := strings.TrimSpace(turn.AssistantMessage.Content)
		if assistant == "" {
			assistant = "（空）"
		}
		b.WriteString(assistant)
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String()) + "\n"
}

func slugifySkillExportName(value string) string {
	s := strings.ToLower(strings.TrimSpace(value))
	if s == "" {
		return "item"
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
		case r == '-' || r == '_':
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "item"
	}
	return out
}
