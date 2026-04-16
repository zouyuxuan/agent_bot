# AI Bot Chain Studio

一个基于 Go 后端和原生 JavaScript 前端构建的 0G Agent Memory Layer 项目。用户可以自定义机器人性格、系统提示词与技能层；随着持续互动，系统会把对话沉淀为可验证的长期记忆，再导出为 Skills、发布到 0G，并在多 Agent 场景中复用。

## 项目结构

```text
.
├── backend
│   ├── cmd/server
│   └── internal
│       ├── app
│       ├── domain
│       ├── service
│       ├── store
│       └── zerog
└── frontend
```

## 当前能力

- 创建和管理聊天机器人资料
- 聊天过程中累积成长值
- 自动把每轮对话转换为长期记忆样本
- 查看训练样本与样本级别的 0G proof 信息
- 使用 0G Compute 对长期记忆做蒸馏，自动提炼用户画像、稳定规则与 Skills 草稿
- 一键将训练样本导出为 Skills 压缩包，便于复用、分享或重新导入
- 一键将训练样本发布到 0G（已接入 0G Storage Go SDK；未配置密钥时自动降级为本地模拟引用）
- 将训练记忆转化为 Skills，并作为可迁移的 Agent 资产层管理
- 后端可直接托管前端静态页面

## 启动方式

在仓库根目录执行：

```bash
go run ./backend/cmd/server
```

启动后访问 [http://localhost:8080](http://localhost:8080)。

## Go 版本

后端 `go.mod` 已升级到 `go 1.26` 并包含 `toolchain` 指令（会自动下载对应 Go toolchain）。如果你本机/CI 的 `go` 命令无法自动拉取 toolchain，请先升级本机 Go 版本，或设置可用的 `GOPROXY`。

## 配置（.env）

后端启动时会自动读取仓库根目录的 `.env`（没有也不影响启动）。示例配置见 `.env.example`。

`DATABASE_URL` 仅用于把发布记录写入 PostgreSQL，本地开发可留空。若已配置但数据库暂时不可用，后端默认会降级为内存 publish log 并继续启动；如果你希望数据库不可用时直接启动失败，可设置 `DATABASE_REQUIRED=1`。

## 大模型调用

前端页面只需要填写 `API Key`，并在机器人资料里选择/填写“默认模型 ID”。API Key 不会保存到服务器，只会在发送消息时随 `POST /api/bots/:id/chat` 请求传给后端用于调用模型。

后端实现的是 OpenAI Compatible 的 `POST /v1/chat/completions` 方式，并会根据“默认模型 ID”自动推断常见厂商的 Base URL（例如 `deepseek-*`、`MiniMax-*`、`gpt-*` 等）。

## 关键接口

- `GET /api/bots` 获取机器人列表
- `POST /api/bots` 创建或更新机器人
- `GET /api/bots/:id` 获取机器人详情
- `POST /api/bots/:id/chat` 发送消息并记录训练数据
- `GET /api/bots/:id/memories` 查看历史对话
- `GET /api/bots/:id/datasets` 查看训练样本
- `GET /api/bots/:id/datasets/export_skills` 将训练样本导出为 Skills ZIP
- `POST /api/bots/:id/datasets/distill` 使用 0G Compute 蒸馏训练记忆
- `POST /api/bots/:id/datasets/distill/save` 将蒸馏得到的候选 Skills 保存到当前机器人
- `POST /api/bots/:id/publish` 发布训练数据到 0G
- `POST /api/x402/fetch` 通过 x402 协议发起“需要付费”的 HTTP 请求（buyer 侧自动处理 402 支付挑战）

## x402 集成说明

后端已接入 x402 的 Go SDK（buyer 侧），用于让“机器人”能够以按次付费的方式访问 x402 保护的资源（HTTP 402 Payment Required）。

### x402 环境变量

- `X402_EVM_PRIVATE_KEY` 用于签名 x402 付款 payload 的 EVM 私钥（必填）

安全建议：使用专用的小额钱包，严格限制余额与权限；不要复用生产资金钱包。

### /api/x402/fetch

这是一个最小化的代理接口，后端会用 x402 SDK 自动完成 `402 -> 支付签名 -> 重试` 的流程。为了避免变成公共 open proxy，本接口要求先完成钱包登录授权（复用现有的 `/api/auth/*` token）。

请求示例（body）：

```json
{
  "url": "https://api.example.com/paid-endpoint",
  "method": "GET",
  "headers": { "accept": "application/json" },
  "timeoutMs": 45000
}
```

## 0G 集成说明

当前 `backend/internal/zerog/client.go` 使用 `github.com/0gfoundation/0g-storage-client`（0G Storage Go SDK）上传训练数据：

- 配置 `ZERO_G_PRIVATE_KEY` 后：走真实 0G 上传（通过 indexer 选择节点，提交链上日志并传输数据到存储节点）
- 未配置 `ZERO_G_PRIVATE_KEY`：自动降级为本地模拟引用，保证本地开发可跑通

### 0G 环境变量

- `ZERO_G_PRIVATE_KEY` EVM 私钥
- `ZERO_G_EVM_RPC` 默认 `https://evmrpc-testnet.0g.ai`
- `ZERO_G_INDEXER_RPC` 默认 `https://indexer-storage-testnet-standard.0g.ai`
- `ZERO_G_UPLOAD_METHOD` 默认 `min`
- `ZERO_G_REPLICAS` 默认 `1`
- `ZERO_G_ZGS_NODES` 可选，逗号分隔的存储节点 URL 列表（当 indexer 503 时用于直连上传绕过 indexer；可从 StorageScan 的 miners 列表复制 URL）
- `ZERO_G_RPC_TIMEOUT_MS` 默认 `30000`，ZGS/EVM RPC 单次请求超时（毫秒）
- `ZERO_G_ZGS_PROBE_TIMEOUT_MS` 默认 `3500`，发布前探测 ZGS 节点可达性超时（毫秒）

### 0G Compute 环境变量

- `ZERO_G_COMPUTE_API_KEY` 0G Compute 的 API Key
- `ZERO_G_COMPUTE_SERVICE_URL` 0G Compute 的 service URL 或 OpenAI-compatible base URL
- `ZERO_G_COMPUTE_MODEL` 默认 `THUDM/GLM-5-FP8`
- `ZERO_G_COMPUTE_TIMEOUT_MS` 默认 `90000`
