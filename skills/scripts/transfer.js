/**
 * MetaMask 转账 Skill
 * 支持 ETH 和 ERC-20 代币转账
 */

class MetaMaskTransfer {
    constructor() {
        this.provider = null;
        this.signer = null;
        this.currentAccount = null;
        this.currentChainId = null;
        this.isConnected = false;

        // ERC-20 ABI (简化版，只包含转账和余额查询)
        this.erc20ABI = [
            "function transfer(address to, uint256 amount) returns (bool)",
            "function balanceOf(address owner) view returns (uint256)",
            "function decimals() view returns (uint8)",
            "function symbol() view returns (string)"
        ];

        this.init();
    }

    /**
     * 初始化事件监听
     */
    init() {
        // 监听账户切换
        if (window.ethereum) {
            window.ethereum.on('accountsChanged', (accounts) => {
                this.handleAccountChange(accounts);
            });

            // 监听网络切换
            window.ethereum.on('chainChanged', (chainId) => {
                this.handleChainChange(chainId);
            });

            // 监听断开连接
            window.ethereum.on('disconnect', (error) => {
                console.log('MetaMask 断开连接', error);
                this.isConnected = false;
                this.updateUIForDisconnect();
            });
        }
    }

    /**
     * 检查 MetaMask 是否安装
     */
    checkMetaMask() {
        if (typeof window.ethereum === 'undefined') {
            throw new Error('请先安装 MetaMask 插件！');
        }
        return true;
    }

    /**
     * 连接钱包
     */
    async connect() {
        try {
            this.checkMetaMask();

            // 请求账户授权
            const accounts = await window.ethereum.request({
                method: 'eth_requestAccounts'
            });

            if (accounts.length === 0) {
                throw new Error('未获取到账户信息');
            }

            // 初始化 ethers provider
            this.provider = new ethers.providers.Web3Provider(window.ethereum);
            this.signer = this.provider.getSigner();
            this.currentAccount = accounts[0];
            this.isConnected = true;

            // 获取网络信息
            const network = await this.provider.getNetwork();
            this.currentChainId = network.chainId;

            // 获取 ETH 余额
            const balance = await this.provider.getBalance(this.currentAccount);
            const ethBalance = ethers.utils.formatEther(balance);

            // 更新 UI
            this.updateUIAfterConnect(accounts[0], ethBalance, this.getNetworkName());

            this.showStatus('success', `✅ 连接成功！当前网络: ${this.getNetworkName()}`);

            return {
                success: true,
                account: this.currentAccount,
                chainId: this.currentChainId,
                balance: ethBalance
            };

        } catch (error) {
            console.error('连接失败:', error);
            this.showStatus('error', `❌ 连接失败: ${error.message}`);
            throw error;
        }
    }

    /**
     * 转账 ETH
     */
    async transferETH(to, amount) {
        try {
            if (!this.isConnected) {
                throw new Error('请先连接钱包');
            }

            if (!to || !ethers.utils.isAddress(to)) {
                throw new Error('无效的接收地址');
            }

            const amountWei = ethers.utils.parseEther(amount.toString());
            const balance = await this.provider.getBalance(this.currentAccount);

            if (balance.lt(amountWei)) {
                throw new Error('余额不足');
            }

            // 发送交易
            const tx = await this.signer.sendTransaction({
                to: to,
                value: amountWei,
                gasLimit: 21000 // ETH 转账标准 gas 限制
            });

            this.showStatus('info', '⏳ 交易已提交，等待确认...');

            // 等待交易确认
            const receipt = await tx.wait();

            this.showStatus('success', `✅ 转账成功！交易哈希: ${tx.hash}`);

            // 更新余额
            const newBalance = await this.provider.getBalance(this.currentAccount);
            const newEthBalance = ethers.utils.formatEther(newBalance);
            document.getElementById('ethBalance').innerHTML = `${parseFloat(newEthBalance).toFixed(6)} ETH`;

            return {
                success: true,
                txHash: tx.hash,
                receipt: receipt
            };

        } catch (error) {
            console.error('ETH 转账失败:', error);
            this.showStatus('error', `❌ 转账失败: ${error.message}`);
            throw error;
        }
    }

    /**
     * 转账 ERC-20 代币
     */
    async transferERC20(tokenAddress, to, amount) {
        try {
            if (!this.isConnected) {
                throw new Error('请先连接钱包');
            }

            if (!tokenAddress || !ethers.utils.isAddress(tokenAddress)) {
                throw new Error('无效的代币合约地址');
            }

            if (!to || !ethers.utils.isAddress(to)) {
                throw new Error('无效的接收地址');
            }

            // 创建代币合约实例
            const tokenContract = new ethers.Contract(
                tokenAddress,
                this.erc20ABI,
                this.signer
            );

            // 获取代币信息
            const decimals = await tokenContract.decimals();
            const symbol = await tokenContract.symbol();

            // 转换金额（考虑小数位数）
            const amountWithDecimals = ethers.utils.parseUnits(amount.toString(), decimals);

            // 检查余额
            const balance = await tokenContract.balanceOf(this.currentAccount);
            if (balance.lt(amountWithDecimals)) {
                throw new Error(`余额不足，当前余额: ${ethers.utils.formatUnits(balance, decimals)} ${symbol}`);
            }

            this.showStatus('info', `⏳ 正在转账 ${amount} ${symbol}...`);

            // 发送交易
            const tx = await tokenContract.transfer(to, amountWithDecimals);

            this.showStatus('info', '⏳ 交易已提交，等待确认...');

            // 等待交易确认
            const receipt = await tx.wait();

            this.showStatus('success', `✅ ${symbol} 转账成功！交易哈希: ${tx.hash}`);

            return {
                success: true,
                txHash: tx.hash,
                receipt: receipt,
                symbol: symbol,
                amount: amount
            };

        } catch (error) {
            console.error('ERC-20 转账失败:', error);
            this.showStatus('error', `❌ 转账失败: ${error.message}`);
            throw error;
        }
    }

    /**
     * 获取网络名称
     */
    getNetworkName() {
        const networks = {
            1: 'Ethereum Mainnet',
            5: 'Goerli Testnet',
            11155111: 'Sepolia Testnet',
            56: 'BSC Mainnet',
            97: 'BSC Testnet',
            137: 'Polygon Mainnet',
            80001: 'Mumbai Testnet'
        };
        return networks[this.currentChainId] || `Chain ID: ${this.currentChainId}`;
    }

    /**
     * 处理账户切换
     */
    handleAccountChange(accounts) {
        if (accounts.length === 0) {
            // 用户断开了连接
            this.isConnected = false;
            this.updateUIForDisconnect();
            this.showStatus('info', '⚠️ 钱包已断开连接');
        } else {
            // 切换了账户
            this.currentAccount = accounts[0];
            this.updateAccountInfo();
            this.showStatus('success', `🔄 账户已切换: ${this.formatAddress(accounts[0])}`);
        }
    }

    /**
     * 处理网络切换
     */
    async handleChainChange(chainId) {
        this.currentChainId = parseInt(chainId, 16);
        this.provider = new ethers.providers.Web3Provider(window.ethereum);
        this.signer = this.provider.getSigner();
        await this.updateAccountInfo();
        this.showStatus('info', `🔄 网络已切换至: ${this.getNetworkName()}`);
    }

    /**
     * 更新账户信息
     */
    async updateAccountInfo() {
        if (!this.isConnected || !this.currentAccount) return;

        try {
            const balance = await this.provider.getBalance(this.currentAccount);
            const ethBalance = ethers.utils.formatEther(balance);

            document.getElementById('accountAddress').innerHTML = this.formatAddress(this.currentAccount);
            document.getElementById('ethBalance').innerHTML = `${parseFloat(ethBalance).toFixed(6)} ETH`;
            document.getElementById('networkStatus').innerHTML = `<span class="network-badge">${this.getNetworkName()}</span>`;
        } catch (error) {
            console.error('更新账户信息失败:', error);
        }
    }

    /**
     * 更新连接后的 UI
     */
    updateUIAfterConnect(account, balance, network) {
        document.getElementById('accountAddress').innerHTML = this.formatAddress(account);
        document.getElementById('ethBalance').innerHTML = `${parseFloat(balance).toFixed(6)} ETH`;
        document.getElementById('networkStatus').innerHTML = `<span class="network-badge">${network}</span>`;
        document.getElementById('connectBtn').innerHTML = '✅ 已连接';
        document.getElementById('connectBtn').disabled = true;
        document.getElementById('transferBtn').disabled = false;
    }

    /**
     * 更新断开连接的 UI
     */
    updateUIForDisconnect() {
        document.getElementById('accountAddress').innerHTML = '-';
        document.getElementById('ethBalance').innerHTML = '-';
        document.getElementById('networkStatus').innerHTML = '未连接';
        document.getElementById('connectBtn').innerHTML = '🔌 连接钱包';
        document.getElementById('connectBtn').disabled = false;
        document.getElementById('transferBtn').disabled = true;
        this.isConnected = false;
    }

    /**
     * 格式化地址显示
     */
    formatAddress(address) {
        if (!address) return '-';
        return `${address.slice(0, 6)}...${address.slice(-4)}`;
    }

    /**
     * 显示状态消息
     */
    showStatus(type, message) {
        const statusDiv = document.getElementById('status');
        statusDiv.className = `status ${type}`;
        statusDiv.innerHTML = message;

        // 5秒后自动清除成功/信息消息
        if (type !== 'error') {
            setTimeout(() => {
                if (statusDiv.className === `status ${type}`) {
                    statusDiv.style.display = 'none';
                }
            }, 5000);
        }
    }
}

// 初始化应用
document.addEventListener('DOMContentLoaded', () => {
    const metamask = new MetaMaskTransfer();

    // 获取 DOM 元素
    const connectBtn = document.getElementById('connectBtn');
    const transferBtn = document.getElementById('transferBtn');
    const transferType = document.getElementById('transferType');
    const tokenAddressGroup = document.getElementById('tokenAddressGroup');

    // 切换转账类型显示
    transferType.addEventListener('change', (e) => {
        if (e.target.value === 'erc20') {
            tokenAddressGroup.style.display = 'block';
        } else {
            tokenAddressGroup.style.display = 'none';
        }
    });

    // 连接钱包
    connectBtn.addEventListener('click', async () => {
        await metamask.connect();
    });

    // 执行转账
    transferBtn.addEventListener('click', async () => {
        const type = transferType.value;
        const recipient = document.getElementById('recipientAddress').value.trim();
        const amount = document.getElementById('amount').value;

        if (!recipient) {
            metamask.showStatus('error', '请输入接收地址');
            return;
        }

        if (!amount || amount <= 0) {
            metamask.showStatus('error', '请输入有效的转账金额');
            return;
        }

        // 确认对话框
        const confirmed = confirm(`确认转账 ${amount} ${type === 'eth' ? 'ETH' : '代币'} 到地址 ${recipient.slice(0, 10)}...？`);
        if (!confirmed) return;

        if (type === 'eth') {
            await metamask.transferETH(recipient, amount);
        } else {
            const tokenAddress = document.getElementById('tokenAddress').value.trim();
            if (!tokenAddress) {
                metamask.showStatus('error', '请输入代币合约地址');
                return;
            }
            await metamask.transferERC20(tokenAddress, recipient, amount);
        }

        // 清空表单
        document.getElementById('recipientAddress').value = '';
        document.getElementById('amount').value = '';
        if (type === 'erc20') {
            document.getElementById('tokenAddress').value = '';
        }
    });
});