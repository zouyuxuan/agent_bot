package service

import (
	"encoding/json"
	"strings"

	"ai-bot-chain/backend/internal/domain"
)

func looksLikeTransferSkillContent(content string) bool {
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
	return t == "evm_transfer" || t == "metamask_transfer"
}

// buildTransferContextFromFrontend converts frontend-executed transfer tool results
// into a compact prompt block.
func buildTransferContextFromFrontend(results []domain.TransferToolResult) string {
	if len(results) == 0 {
		return ""
	}
	if len(results) > 3 {
		results = results[:3]
	}

	var b strings.Builder
	for _, r := range results {
		title := strings.TrimSpace(r.SkillID)
		if title == "" {
			title = "transfer"
		}
		b.WriteString("### ")
		b.WriteString(title)
		b.WriteString("\n")

		t := strings.TrimSpace(r.Type)
		if t != "" {
			b.WriteString("type: ")
			b.WriteString(t)
			b.WriteString("\n")
		}
		if r.ChainID != 0 {
			b.WriteString("chainId: ")
			b.WriteString(intToString(r.ChainID))
			b.WriteString("\n")
		}
		if to := strings.TrimSpace(r.To); to != "" {
			b.WriteString("to: ")
			b.WriteString(to)
			b.WriteString("\n")
		}
		if token := strings.TrimSpace(r.Token); token != "" {
			b.WriteString("token: ")
			b.WriteString(token)
			b.WriteString("\n")
		}
		if amt := strings.TrimSpace(r.Amount); amt != "" {
			b.WriteString("amount: ")
			b.WriteString(amt)
			b.WriteString("\n")
		}
		if amtWei := strings.TrimSpace(r.AmountWei); amtWei != "" {
			b.WriteString("amountWei: ")
			b.WriteString(amtWei)
			b.WriteString("\n")
		}
		if tx := strings.TrimSpace(r.TxHash); tx != "" {
			b.WriteString("txHash: ")
			b.WriteString(tx)
			b.WriteString("\n")
		}
		b.WriteString("ok: ")
		if r.OK {
			b.WriteString("true\n")
		} else {
			b.WriteString("false\n")
		}
		if errMsg := strings.TrimSpace(r.Error); errMsg != "" {
			b.WriteString("error: ")
			b.WriteString(trunc(errMsg, 400))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
