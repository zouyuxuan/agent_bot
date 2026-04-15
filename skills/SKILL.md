---
name: metamask-transfer
description: 调用 MetaMask 插件钱包进行 ETH 或 ERC-20 代币转账的 Skill。支持连接钱包、获取账户、发送交易等功能。
version: 1.0.0
author: Assistant
tags: [web3, blockchain, metamask, ethereum, transfer]
---

# MetaMask 转账 Skill

## 功能概述
这个 Skill 提供完整的 MetaMask 钱包集成能力，支持：
- 连接/断开 MetaMask 钱包
- 获取当前账户和网络信息
- 转账 ETH（原生代币）
- 转账 ERC-20 代币
- 查询交易状态
- 监听账户/网络切换事件

## 前置要求
1. 用户浏览器已安装 MetaMask 扩展插件
2. 网页环境（HTTPS 或 localhost）
3. 引入 ethers.js 库（用于简化交易操作）

## 代码实现

### 1. HTML 基础结构

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MetaMask 转账工具</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            justify-content: center;
            align-items: center;
            padding: 20px;
        }
        
        .container {
            background: white;
            border-radius: 20px;
            padding: 40px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            max-width: 500px;
            width: 100%;
        }
        
        h1 {
            color: #333;
            margin-bottom: 10px;
            font-size: 28px;
        }
        
        .subtitle {
            color: #666;
            margin-bottom: 30px;
            font-size: 14px;
        }
        
        .info-card {
            background: #f5f5f5;
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 20px;
        }
        
        .info-row {
            display: flex;
            justify-content: space-between;
            margin-bottom: 12px;
            font-size: 14px;
        }
        
        .info-label {
            color: #666;
            font-weight: 500;
        }
        
        .info-value {
            color: #333;
            word-break: break-all;
            text-align: right;
            max-width: 60%;
        }
        
        .input-group {
            margin-bottom: 20px;
        }
        
        label {
            display: block;
            margin-bottom: 8px;
            color: #333;
            font-weight: 500;
            font-size: 14px;
        }
        
        input, select {
            width: 100%;
            padding: 12px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-size: 14px;
            transition: border-color 0.3s;
        }
        
        input:focus, select:focus {
            outline: none;
            border-color: #667eea;
        }
        
        button {
            width: 100%;
            padding: 14px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
            margin-bottom: 12px;
        }
        
        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 5px 15px rgba(102, 126, 234, 0.4);
        }
        
        button:disabled {
            opacity: 0.6;
            cursor: not-allowed;
            transform: none;
        }
        
        .status {
            margin-top: 20px;
            padding: 12px;
            border-radius: 8px;
            font-size: 14px;
            display: none;
        }
        
        .status.success {
            background: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
            display: block;
        }
        
        .status.error {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
            display: block;
        }
        
        .status.info {
            background: #d1ecf1;
            color: #0c5460;
            border: 1px solid #bee5eb;
            display: block;
        }
        
        .balance {
            font-size: 24px;
            font-weight: bold;
            color: #667eea;
            margin-top: 10px;
        }
        
        .network-badge {
            display: inline-block;
            padding: 4px 12px;
            background: #667eea;
            color: white;
            border-radius: 20px;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🦊 MetaMask 转账工具</h1>
        <div class="subtitle">安全的以太坊交易助手</div>
        
        <div class="info-card">
            <div class="info-row">
                <span class="info-label">网络状态：</span>
                <span class="info-value" id="networkStatus">未连接</span>
            </div>
            <div class="info-row">
                <span class="info-label">当前账户：</span>
                <span class="info-value" id="accountAddress">-</span>
            </div>
            <div class="info-row">
                <span class="info-label">ETH 余额：</span>
                <span class="info-value" id="ethBalance">-</span>
            </div>
        </div>
        
        <div class="input-group">
            <label>转账类型：</label>
            <select id="transferType">
                <option value="eth">ETH 转账</option>
                <option value="erc20">ERC-20 代币转账</option>
            </select>
        </div>
        
        <div class="input-group" id="tokenAddressGroup" style="display: none;">
            <label>代币合约地址：</label>
            <input type="text" id="tokenAddress" placeholder="0x..." />
        </div>
        
        <div class="input-group">
            <label>接收地址：</label>
            <input type="text" id="recipientAddress" placeholder="0x..." />
        </div>
        
        <div class="input-group">
            <label>转账金额：</label>
            <input type="number" id="amount" placeholder="例如: 0.1" step="any" />
        </div>
        
        <button id="connectBtn">🔌 连接钱包</button>
        <button id="transferBtn" disabled>💸 确认转账</button>
        
        <div id="status" class="status"></div>
    </div>
    
    <script src="https://cdn.jsdelivr.net/npm/ethers@5.7.2/dist/ethers.umd.min.js"></script>
    <script src="transfer.js"></script>
</body>
</html>