package service

import (
	"strings"

	"ai-bot-chain/backend/internal/domain"
)

func buildReplyFromFrontendToolResults(x402Results []domain.X402ToolResult, transferResults []domain.TransferToolResult) (string, bool) {
	if reply := buildTransferToolReply(transferResults); strings.TrimSpace(reply) != "" {
		return reply, true
	}
	return "", false
}

func buildTransferToolReply(results []domain.TransferToolResult) string {
	if len(results) == 0 {
		return ""
	}

	r := results[0]
	var b strings.Builder

	if r.OK {
		b.WriteString("已为你发起链上转账。\n")
		if amt := strings.TrimSpace(r.Amount); amt != "" {
			b.WriteString("金额：")
			b.WriteString(amt)
			if token := strings.TrimSpace(r.Token); token != "" && token != "native" {
				b.WriteString(" ")
				b.WriteString(token)
			}
			b.WriteString("\n")
		}
		if to := strings.TrimSpace(r.To); to != "" {
			b.WriteString("收款地址：")
			b.WriteString(to)
			b.WriteString("\n")
		}
		if r.ChainID != 0 {
			b.WriteString("链 ID：")
			b.WriteString(intToString(r.ChainID))
			b.WriteString("\n")
		}
		if tx := strings.TrimSpace(r.TxHash); tx != "" {
			b.WriteString("交易哈希：")
			b.WriteString(tx)
			b.WriteString("\n")
		}
		b.WriteString("说明：当前表示钱包已提交交易请求并返回交易哈希，最终是否上链成功请以区块浏览器或钱包状态为准。")
		return strings.TrimSpace(b.String())
	}

	b.WriteString("这次转账未完成。")
	if errMsg := strings.TrimSpace(r.Error); errMsg != "" {
		b.WriteString("\n原因：")
		b.WriteString(errMsg)
	}
	if to := strings.TrimSpace(r.To); to != "" || strings.TrimSpace(r.Amount) != "" {
		b.WriteString("\n请检查转账地址、金额、链 ID 与钱包网络后重试。")
	}
	return strings.TrimSpace(b.String())
}
