const apiBase = "";

const state = {
  activeBotId: "",
  activeBot: null,
  bots: [],
  skills: [],
  selectedSkillFolders: new Set(),
};

const botForm = document.querySelector("#bot-form");
const botList = document.querySelector("#bot-list");
const refreshBotsBtn = document.querySelector("#refresh-bots");
const chatForm = document.querySelector("#chat-form");
const chatInput = document.querySelector("#chat-input");
const chatStream = document.querySelector("#chat-stream");
const activeBotName = document.querySelector("#active-bot-name");
const growthScore = document.querySelector("#growth-score");
const llmStatus = document.querySelector("#llm-status");
const datasetList = document.querySelector("#dataset-list");
const publishBtn = document.querySelector("#publish-btn");
const publishResult = document.querySelector("#publish-result");
const zgsNodesInput = document.querySelector("#zgs-nodes");
const skillsSummary = document.querySelector("#skills-summary");
const skillsList = document.querySelector("#skills-list");
const skillsGitHubURL = document.querySelector("#skills-github-url");
const skillsGitHubImportPublishBtn = document.querySelector("#skills-github-import-publish");
const skillsPublishBundleBtn = document.querySelector("#skills-publish-bundle");
const llmApiKey = document.querySelector("#llm-apikey");
const llmTemp = document.querySelector("#llm-temp");
const debugSkills = document.querySelector("#debug-skills");
const botModelPreset = document.querySelector("#bot-model-preset");
const botModelCustom = document.querySelector("#bot-model-custom");
const walletConnectBtn = document.querySelector("#wallet-connect");
const walletStatus = document.querySelector("#wallet-status");
const walletBox = document.querySelector("#wallet-box");
const x402Url = document.querySelector("#x402-url");
const x402Method = document.querySelector("#x402-method");
const x402Headers = document.querySelector("#x402-headers");
const x402Body = document.querySelector("#x402-body");
const x402Timeout = document.querySelector("#x402-timeout");
const x402Send = document.querySelector("#x402-send");
const x402Output = document.querySelector("#x402-output");
const x402Box = document.querySelector("#x402-box");
const x402Toggle = document.querySelector("#x402-toggle");
const x402UseProxy = document.querySelector("#x402-use-proxy");

botForm.addEventListener("submit", handleBotSubmit);
refreshBotsBtn.addEventListener("click", loadBots);
botList.addEventListener("change", handleBotSwitch);
chatForm.addEventListener("submit", handleChatSubmit);
publishBtn.addEventListener("click", publishDatasets);
skillsGitHubImportPublishBtn?.addEventListener("click", importAndPublishSkillsFromGitHub);
skillsPublishBundleBtn?.addEventListener("click", publishSkillsBundle);
botModelPreset?.addEventListener("change", syncBotModelPreset);
walletConnectBtn?.addEventListener("click", handleWalletButtonClick);
x402Send?.addEventListener("click", sendX402Test);
x402Toggle?.addEventListener("click", toggleX402Panel);

loadBots();
restoreZgsNodes();
renderWalletStatus();
bindWalletEvents();
prefillZeroGConfig();
syncWalletFromProvider();
restoreX402PanelState();
restoreDebugSettings();

function syncBotModelPreset() {
  if (!botModelPreset || !botModelCustom) return;
  const chosen = botModelPreset.value || "";
  if (chosen) {
    botModelCustom.value = chosen;
  }
}

function inferBaseUrlFromModel(model) {
  const m = String(model || "").trim();
  const low = m.toLowerCase();
  if (!m) return "";
  if (low.startsWith("deepseek-")) return "https://api.deepseek.com";
  if (m.startsWith("MiniMax-") || low.startsWith("minimax-")) return "https://api.minimax.io/v1";
  if (low.startsWith("gpt-") || low.startsWith("o1") || low.startsWith("o3")) return "https://api.openai.com";
  return "https://api.openai.com";
}

async function loadBots() {
  const response = await fetch(`${apiBase}/api/bots`);
  const bots = await response.json();
  state.bots = bots;

  botList.innerHTML = "";
  if (!bots.length) {
    const option = document.createElement("option");
    option.textContent = "暂无机器人";
    option.value = "";
    botList.append(option);
    state.activeBotId = "";
    renderActiveBot();
    renderDatasets([]);
    return;
  }

  bots.forEach((bot) => {
    const option = document.createElement("option");
    option.value = bot.id;
    option.textContent = `${bot.name || bot.id} (${bot.modelType || "default"})`;
    botList.append(option);
  });

  if (!state.activeBotId || !bots.some((bot) => bot.id === state.activeBotId)) {
    state.activeBotId = bots[0].id;
  }
  botList.value = state.activeBotId;
  await refreshActiveBotViews();
}

async function handleBotSubmit(event) {
  event.preventDefault();
  syncBotModelPreset();
  const formData = new FormData(botForm);
  const payload = Object.fromEntries(formData.entries());
  const response = await fetch(`${apiBase}/api/bots`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const bot = await response.json();
  state.activeBotId = bot.id;
  botForm.reset();
  await loadBots();
}

async function handleBotSwitch() {
  state.activeBotId = botList.value;
  await refreshActiveBotViews();
}

async function refreshActiveBotViews() {
  if (!state.activeBotId) {
    renderActiveBot();
    renderDatasets([]);
    renderSkills([]);
    return;
  }

  const [botResponse, memoriesResponse, datasetsResponse, skillsResponse] = await Promise.all([
    fetch(`${apiBase}/api/bots/${state.activeBotId}`),
    fetch(`${apiBase}/api/bots/${state.activeBotId}/memories`),
    fetch(`${apiBase}/api/bots/${state.activeBotId}/datasets`),
    fetch(`${apiBase}/api/bots/${state.activeBotId}/skills`),
  ]);

  const bot = await botResponse.json();
  const memories = await memoriesResponse.json();
  const datasets = await datasetsResponse.json();
  const skills = await skillsResponse.json();

  state.activeBot = bot;
  renderActiveBot(bot);
  renderMemories(memories);
  renderDatasets(datasets);
  state.skills = Array.isArray(skills) ? skills : [];
  renderSkills(state.skills);
}

function renderActiveBot(bot = null) {
  activeBotName.textContent = bot ? bot.name || bot.id : "请选择或创建机器人";
  growthScore.textContent = bot ? bot.growthScore : "0";
  if (llmStatus) {
    llmStatus.textContent = "";
  }
}

function renderMemories(memories) {
  chatStream.innerHTML = "";
  memories.forEach((turn) => {
    appendBubble("user", turn.userMessage.content);
    appendBubble("assistant", turn.assistantMessage.content);
  });
  chatStream.scrollTop = chatStream.scrollHeight;
}

function renderDatasets(samples) {
  datasetList.innerHTML = "";
  samples
    .slice()
    .reverse()
    .forEach((sample) => {
      const div = document.createElement("article");
      div.className = "dataset-item";
      div.innerHTML = `
        <strong>${sample.summary}</strong>
        <small>标签：${(sample.tags || []).join(", ") || "无"} </small>
        <small>状态：${sample.storedOn0G ? `已上链 ${sample.storageRef}` : "待发布到 0G"}</small>
      `;
      datasetList.append(div);
    });
}

function renderSkills(skills) {
  if (!skillsList || !skillsSummary) return;
  const raw = Array.isArray(skills) ? skills : [];
  skillsSummary.textContent = raw.length ? `已上传 ${raw.length} 个 Skills（按文件夹分组）。勾选文件夹后可在对话中启用。` : "暂无 Skills。请先从 GitHub 拉取或上传 Skills。";
  if (skillsPublishBundleBtn) {
    const pending = raw.filter((s) => !s.storedOn0G).length;
    skillsPublishBundleBtn.disabled = pending === 0;
    skillsPublishBundleBtn.textContent = pending ? `发布未上链 Skills 到 0G（${pending}）` : "发布未上链 Skills 到 0G";
  }

  const folderMap = new Map();
  for (const sk of raw) {
    const fn = String(sk.filename || "").trim();
    const folder = fn.includes("/") ? fn.split("/")[0] : "(root)";
    if (!folderMap.has(folder)) folderMap.set(folder, []);
    folderMap.get(folder).push(sk);
  }

  // Clean selection if folders disappear.
  const folderSet = new Set(Array.from(folderMap.keys()));
  for (const f of Array.from(state.selectedSkillFolders)) {
    if (!folderSet.has(f)) state.selectedSkillFolders.delete(f);
  }

  const folders = Array.from(folderMap.entries())
    .map(([folder, items]) => {
      const pending = items.filter((s) => !s.storedOn0G).length;
      return { folder, items, pending };
    })
    .sort((a, b) => a.folder.localeCompare(b.folder));

  skillsList.innerHTML = "";
  folders.forEach((f) => {
    const div = document.createElement("article");
    div.className = "skill-folder-item";

    const checked = state.selectedSkillFolders.has(f.folder);
    const names = f.items
      .slice()
      .sort((a, b) => String(a.filename || "").localeCompare(String(b.filename || "")))
      .map((s) => {
        const fn = String(s.filename || "").trim();
        // show file name without the folder prefix
        if (fn.includes("/")) return fn.split("/").slice(1).join("/");
        return fn || (s.name || s.id);
      })
      .filter(Boolean);
    const preview = names.slice(0, 6).join(", ") + (names.length > 6 ? ` ... +${names.length - 6}` : "");

    div.innerHTML = `
      <div class="row">
        <div class="skill-title">
          <input type="checkbox" data-skill-folder-check="${escapeHtml(f.folder)}" ${checked ? "checked" : ""} />
          <strong>${escapeHtml(f.folder)}</strong>
        </div>
      </div>
      <small class="hint">文件数：${f.items.length}；待发布：${f.pending}</small>
      <small class="hint">Skills：${escapeHtml(preview || "-")}</small>
      <small class="hint">状态：${f.pending === 0 ? "已上链" : "未上链"}（未上链也可启用；发布仅用于 0G 存储/分享）</small>
    `;
    skillsList.append(div);
  });

  skillsList.querySelectorAll("input[data-skill-folder-check]").forEach((el) => {
    el.addEventListener("change", (e) => {
      const folder = e.target?.dataset?.skillFolderCheck;
      if (!folder) return;
      if (e.target.checked) state.selectedSkillFolders.add(folder);
      else state.selectedSkillFolders.delete(folder);
    });
  });

  // Folder-level publishing button removed; publish uses bundle publish.
}

async function importSkillsFromGitHub() {
  if (!state.activeBotId) {
    alert("请先选择机器人");
    return;
  }
  const ok = await ensureWalletAuthorized();
  if (!ok) return;

  const url = (skillsGitHubURL?.value || "").trim();
  if (!url) {
    alert("请输入 GitHub 地址");
    return;
  }
  const authToken = localStorage.getItem("authToken") || "";
  if (skillsSummary) skillsSummary.textContent = "正在从 GitHub 拉取并导入...";
  let resp, data;
  try {
    ({ resp, data } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/skills/import_github`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
        },
        body: JSON.stringify({ url }),
      },
      60000,
    ));
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${String(e?.message || e)}`;
    return;
  }
  if (!resp.ok) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${data?.error || resp.statusText}`;
    return;
  }
  if (skillsSummary) skillsSummary.textContent = `已从 GitHub 导入 ${Number(data?.count || 0)} 个 Skills。`;
  const listResp = await fetch(`${apiBase}/api/bots/${state.activeBotId}/skills`);
  const list = await listResp.json();
  state.skills = Array.isArray(list) ? list : [];
  renderSkills(state.skills);
}

async function importAndPublishSkillsFromGitHub() {
  await importSkillsFromGitHub();
  // If import succeeded, the summary is already updated; publish pending skills next.
  // This will still require the user to confirm a wallet transaction.
  await publishSkillsBundle();
}

async function publishSkillsBundle() {
  if (!state.activeBotId) {
    alert("请先选择机器人");
    return;
  }
  const ok = await ensureWalletAuthorized();
  if (!ok) return;

  const authToken = localStorage.getItem("authToken") || "";
  const zgsNodes = (zgsNodesInput?.value || "").trim();
  if (zgsNodesInput) localStorage.setItem("zgsNodes", zgsNodes);

  const from = localStorage.getItem("walletAddress") || "";
  if (!from) {
    if (skillsSummary) skillsSummary.textContent = "[错误] 未连接钱包";
    return;
  }

  // Publish all pending skills as a single bundle transaction (folder structure is kept in filenames).
  const pendingIds = (state.skills || [])
    .filter((s) => {
      return !s.storedOn0G;
    })
    .map((s) => s.id);
  if (!pendingIds.length) {
    if (skillsSummary) skillsSummary.textContent = "暂无需要发布的 Skills";
    return;
  }

  if (skillsSummary) skillsSummary.textContent = "准备链上交易（Skills Bundle）...";
  let prepResp, prep;
  try {
    ({ resp: prepResp, data: prep } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/skills/publish_bundle_prepare`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
        },
        body: JSON.stringify({ zgsNodes, skillIds: pendingIds }),
      },
      45000,
    ));
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] 准备链上交易超时或失败：${String(e?.message || e)}`;
    return;
  }
  if (!prepResp.ok) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${prep?.error || prepResp.statusText}`;
    return;
  }
  if (!prep?.publishId) {
    if (skillsSummary) skillsSummary.textContent = "[错误] 后端未返回 publishId，请重启后端并刷新页面后重试";
    return;
  }

  if (skillsSummary) skillsSummary.textContent = "请在钱包中确认交易（Skills Bundle）...";
  let txHash;
  try {
    txHash = await window.ethereum.request({
      method: "eth_sendTransaction",
      params: [{ from, to: prep.to, data: prep.data, value: prep.value }],
    });
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] 交易发送失败：${String(e?.message || e)}`;
    return;
  }

  if (skillsSummary) skillsSummary.textContent = `交易已发送：${txHash}，等待发布...`;
  let finResp, result;
  try {
    ({ resp: finResp, data: result } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/skills/publish_bundle_finalize`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
        },
        body: JSON.stringify({ zgsNodes, publishId: String(prep.publishId), txHash, rootHash: prep.rootHash }),
      },
      120000,
    ));
  } catch (e) {
    const msg = String(e?.message || e);
    if (msg.includes("请求超时") && txHash) {
      try {
        const { resp, data } = await fetchJsonWithTimeout(
          `${apiBase}/api/zerog/tx_status?txHash=${encodeURIComponent(txHash)}`,
          { method: "GET" },
          12000,
        );
        if (resp.ok && data?.status === "success") {
          if (skillsSummary) skillsSummary.textContent = `链上已成功：${data.txHash}（可查：${data.explorerTxUrl || ""}）。Skills 数据会在后台同步到存储节点。`;
          const listResp = await fetch(`${apiBase}/api/bots/${state.activeBotId}/skills`);
          const list = await listResp.json();
          state.skills = Array.isArray(list) ? list : [];
          renderSkills(state.skills);
          return;
        }
      } catch {}
    }
    if (skillsSummary) skillsSummary.textContent = `[错误] 发布 Skills Bundle 超时或失败：${msg}`;
    return;
  }
  if (!finResp.ok) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${result?.error || finResp.statusText}`;
    return;
  }

  if (skillsSummary) skillsSummary.textContent = `Skills Bundle 已发布：root=${result.rootHash || ""} tx=${result.txHash || ""}`;
  const listResp = await fetch(`${apiBase}/api/bots/${state.activeBotId}/skills`);
  const list = await listResp.json();
  state.skills = Array.isArray(list) ? list : [];
  renderSkills(state.skills);
}

async function handleChatSubmit(event) {
  event.preventDefault();
  if (!state.activeBotId) {
    alert("请先创建机器人");
    return;
  }

  const message = chatInput.value.trim();
  if (!message) {
    return;
  }

  // Built-in x402 command (no skills required).
  // Usage: /402 [METHOD] <URL> [--headers <json>] [--body <json>]
  if (/^\/(x402|402)\b/i.test(message)) {
    await handleX402CommandMessage(message);
    chatInput.value = "";
    return;
  }

  appendBubble("user", message);
  chatInput.value = "";
  const thinking = appendThinkingBubble();

  const apiKey = llmApiKey?.value?.trim() || "";
  const modelFromBot = state.activeBot?.modelType || "";

  const llm =
    apiKey && modelFromBot
      ? {
          apiKey,
          baseUrl: inferBaseUrlFromModel(modelFromBot),
          model: modelFromBot,
          temperature: Number.parseFloat(llmTemp?.value || "0.7") || 0.7,
          maxTokens: 2048,
        }
      : null;

  let response;
  let data;
  try {
    // Expand selected folders into skill IDs.
    const enabledSkillIDs = [];
    const folderSet = new Set(Array.from(state.selectedSkillFolders || []));
    if (folderSet.size) {
      for (const sk of state.skills || []) {
        const fn = String(sk.filename || "").trim();
        const folder = fn.includes("/") ? fn.split("/")[0] : "(root)";
        if (folderSet.has(folder)) enabledSkillIDs.push(sk.id);
      }
    }

    // Frontend-executed x402 tools: run before sending chat so the backend can
    // include the results in the LLM prompt.
    const x402Results = await runX402SkillsInBrowser(enabledSkillIDs, message);

    response = await fetch(`${apiBase}/api/bots/${state.activeBotId}/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        message,
        llm,
        skills: enabledSkillIDs,
        x402: x402Results,
        debug: debugSkills?.checked ? { skillsUsed: true } : undefined,
      }),
    });
    data = await response.json();
  } catch (error) {
    resolveThinking(thinking, `[网络错误] ${String(error)}`);
    return;
  }

  if (!response.ok) {
    resolveThinking(thinking, `[错误] ${data?.error || response.statusText}`);
    return;
  }

  if (!data?.turn?.assistantMessage?.content) {
    resolveThinking(thinking, "[错误] 后端未返回有效回复");
    return;
  }

  resolveThinking(thinking, data.turn.assistantMessage.content);
  state.activeBot = data.bot;
  renderActiveBot(data.bot);
  if (llmStatus) {
    const llmText = data?.meta?.llmUsed ? "本轮回复：已使用大模型" : "本轮回复：未使用大模型（可能未填写 Key/Model 或调用失败）";
    const skillsUsed = Array.isArray(data?.meta?.skillsUsed) ? data.meta.skillsUsed : null;
    if (skillsUsed && skillsUsed.length) {
      const shown = skillsUsed
        .slice(0, 5)
        .map((s) => s?.filename || s?.name || s?.id)
        .filter(Boolean);
      const extra = skillsUsed.length > shown.length ? ` ... +${skillsUsed.length - shown.length}` : "";
      llmStatus.textContent = `${llmText}；Skills：${shown.join(", ")}${extra}`;
    } else if (skillsUsed && skillsUsed.length === 0) {
      llmStatus.textContent = `${llmText}；Skills：无`;
    } else {
      llmStatus.textContent = llmText;
    }
  }

  const datasetsResponse = await fetch(`${apiBase}/api/bots/${state.activeBotId}/datasets`);
  const datasets = await datasetsResponse.json();
  renderDatasets(datasets);
}

function parseX402Command(text) {
  const raw = String(text || "").trim();
  const parts = raw.split(/\s+/);
  if (!parts.length) return null;
  const cmd = parts[0].toLowerCase();
  if (cmd !== "/x402" && cmd !== "/402") return null;
  if (parts.length < 2) return null;

  let method = "GET";
  let url = "";
  let i = 1;

  const maybeMethod = String(parts[i] || "").toUpperCase();
  if (["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"].includes(maybeMethod)) {
    method = maybeMethod;
    i++;
  }
  url = parts[i] || "";
  i++;

  // Remaining args are optional flags. We keep it simple and allow JSON blobs with no spaces.
  let headers = {};
  let body = null;
  while (i < parts.length) {
    const k = parts[i];
    const v = parts[i + 1];
    if (k === "--headers" && v) {
      headers = JSON.parse(v);
      i += 2;
      continue;
    }
    if (k === "--body" && v) {
      body = JSON.parse(v);
      i += 2;
      continue;
    }
    i++;
  }

  return { method, url, headers, body };
}

async function handleX402CommandMessage(message) {
  appendBubble("user", message);

  const parsed = (() => {
    try {
      return parseX402Command(message);
    } catch (e) {
      return { error: String(e?.message || e) };
    }
  })();

  if (!parsed || parsed.error) {
    appendBubble(
      "assistant",
      `[错误] x402 命令格式错误。用法：/402 GET https://... （可选：--headers {"accept":"application/json"} --body {"a":1}，注意 JSON 中不要有空格）\n${parsed?.error ? "原因：" + parsed.error : ""}`,
    );
    return;
  }

  const url = String(parsed.url || "").trim();
  if (!url || (!url.startsWith("https://") && !url.startsWith("http://"))) {
    appendBubble("assistant", "[错误] /x402 URL 必须以 http:// 或 https:// 开头");
    return;
  }

  // Connect wallet once. Payment confirmations still require signature prompts.
  if (!localStorage.getItem("walletAddress")) {
    const ok = await connectWallet();
    if (!ok) {
      appendBubble("assistant", "[错误] 未连接钱包，无法执行 x402 支付");
      return;
    }
  }
  const from = localStorage.getItem("walletAddress") || "";
  if (!from) {
    appendBubble("assistant", "[错误] 未连接钱包，无法执行 x402 支付");
    return;
  }

  appendBubble("assistant", "x402 请求中（如需支付将弹出钱包签名确认）...");

  const startedAt = isoNow();
  let result = {
    skillId: "x402_command",
    filename: "/402",
    url,
    method: parsed.method,
    ok: false,
    status: 0,
    headers: {},
    body: "",
    error: "",
    startedAt,
    endedAt: "",
  };

  try {
    const fetchWithPayment = await getX402FetchWithMetaMask(from, { viaProxy: true });
    const controller = new AbortController();
    const timeoutMs = 65000;
    const t = setTimeout(() => controller.abort(), timeoutMs);
    let resp;
    try {
      resp = await fetchWithPayment(url, {
        method: parsed.method,
        headers: parsed.headers || {},
        ...(parsed.body != null && parsed.method !== "GET" && parsed.method !== "HEAD" ? { body: JSON.stringify(parsed.body) } : {}),
        signal: controller.signal,
      });
    } finally {
      clearTimeout(t);
    }

    const text = await resp.text().catch(() => "");
    result.ok = resp.ok;
    result.status = resp.status;
    result.headers = {
      "content-type": resp.headers.get("content-type") || "",
      "payment-required": resp.headers.get("payment-required") || resp.headers.get("PAYMENT-REQUIRED") || "",
      "payment-response": resp.headers.get("payment-response") || resp.headers.get("PAYMENT-RESPONSE") || "",
    };
    result.body = truncateTextClient(text, 12000);
  } catch (e) {
    const msg = String(e?.message || e);
    if (msg.includes("Failed to fetch")) {
      result.error =
        `Failed to fetch（可能是 URL 不正确或 CORS 跨域被拦截）。` +
        `建议先试 https://sandbox.agentrails.io/api/x402/pricing 或 https://sandbox.agentrails.io/api/x402/protected/analysis。` +
        ` 原始错误：${msg}`;
    } else {
      result.error = msg;
    }
  } finally {
    result.endedAt = isoNow();
  }

  // Send tool output to backend so the agent can respond using the result.
  const apiKey = llmApiKey?.value?.trim() || "";
  const modelFromBot = state.activeBot?.modelType || "";
  const llm =
    apiKey && modelFromBot
      ? {
          apiKey,
          baseUrl: inferBaseUrlFromModel(modelFromBot),
          model: modelFromBot,
          temperature: Number.parseFloat(llmTemp?.value || "0.7") || 0.7,
          maxTokens: 2048,
        }
      : null;

  try {
    const thinking = appendThinkingBubble();
    const response = await fetch(`${apiBase}/api/bots/${state.activeBotId}/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        message: `我通过 x402 发起了付费请求（${parsed.method} ${url}）。请基于工具返回结果给出解释/总结，如果失败请说明原因与下一步建议。`,
        llm,
        skills: [],
        x402: [result],
        debug: debugSkills?.checked ? { skillsUsed: true } : undefined,
      }),
    });
    const data = await response.json();
    if (!response.ok) {
      resolveThinking(thinking, `[错误] ${data?.error || response.statusText}`);
      return;
    }
    resolveThinking(thinking, data?.turn?.assistantMessage?.content || "（无回复）");
  } catch (e) {
    appendBubble("assistant", `[网络错误] ${String(e?.message || e)}`);
  }
}

function restoreDebugSettings() {
  if (!debugSkills) return;
  const v = localStorage.getItem("debugSkillsUsed");
  debugSkills.checked = v === "1";
  debugSkills.addEventListener("change", () => {
    localStorage.setItem("debugSkillsUsed", debugSkills.checked ? "1" : "0");
  });
}

async function publishDatasets() {
  if (!state.activeBotId) {
    alert("请先选择机器人");
    return;
  }

  const ok = await ensureWalletAuthorized();
  if (!ok) return;

  const zgsNodes = (zgsNodesInput?.value || "").trim();
  if (zgsNodesInput) {
    localStorage.setItem("zgsNodes", zgsNodes);
  }

  const authToken = localStorage.getItem("authToken") || "";
  // Wallet-based publish: prepare tx -> send via MetaMask -> finalize upload to ZGS nodes.
  if (!window.ethereum) {
    publishResult.textContent = "[错误] 未检测到 MetaMask";
    return;
  }
  const from = localStorage.getItem("walletAddress") || "";
  if (!from) {
    publishResult.textContent = "[错误] 未连接钱包";
    return;
  }

  publishResult.textContent = "准备链上交易...";
  let prepResp, prep;
  try {
    ({ resp: prepResp, data: prep } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/publish_prepare`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
        },
        body: JSON.stringify({ zgsNodes }),
      },
      45000,
    ));
  } catch (e) {
    publishResult.textContent = `[错误] 准备链上交易超时或失败：${String(e?.message || e)}`;
    return;
  }
  if (!prepResp.ok) {
    publishResult.textContent = `[错误] ${prep?.error || prepResp.statusText}`;
    return;
  }
  if (!prep?.publishId) {
    publishResult.textContent = "[错误] 后端未返回 publishId（可能未重启后端或前端缓存旧版本），请刷新页面并重启后端后再试";
    return;
  }

  publishResult.textContent = "请在钱包中确认交易...";
  let txHash;
  try {
    txHash = await window.ethereum.request({
      method: "eth_sendTransaction",
      params: [
        {
          from,
          to: prep.to,
          data: prep.data,
          value: prep.value,
        },
      ],
    });
  } catch (e) {
    publishResult.textContent = `[错误] 交易发送失败：${String(e?.message || e)}`;
    return;
  }

  publishResult.textContent = `交易已发送：${txHash}，等待上传...`;
  let finResp, result;
  try {
    ({ resp: finResp, data: result } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/publish_finalize`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
        },
        body: JSON.stringify({ zgsNodes, publishId: String(prep.publishId), txHash, rootHash: prep.rootHash }),
      },
      120000,
    ));
  } catch (e) {
    const msg = String(e?.message || e);
    // If finalize timed out, the tx may still be successful on-chain already.
    if (msg.includes("请求超时") && txHash) {
      const ok = await tryResolveTxSuccessFromChain(txHash);
      if (ok) {
        // Refresh dataset list after confirmed success.
        const datasetsResponse = await fetch(`${apiBase}/api/bots/${state.activeBotId}/datasets`);
        const datasets = await datasetsResponse.json();
        renderDatasets(datasets);
        return;
      }
      publishResult.textContent = `[错误] 上传到 0G 超时或失败：${msg}（可用 tx 在浏览器查看确认）`;
      return;
    }
    publishResult.textContent = `[错误] 上传到 0G 超时或失败：${msg}`;
    return;
  }
  if (!finResp.ok) {
    publishResult.textContent = `[错误] ${result?.error || finResp.statusText}`;
    return;
  }

  const txPart = result.explorerTxUrl ? `tx: ${result.txHash}（可查：${result.explorerTxUrl}）` : `tx: ${result.txHash}`;
  const locPart = (result.fileLocations || []).length ? `；nodes: ${(result.fileLocations || []).join(", ")}` : "";
  publishResult.textContent = `已发布 ${result.sampleCount} 条；root: ${result.rootHash}；${txPart}${locPart}`;

  const datasetsResponse = await fetch(`${apiBase}/api/bots/${state.activeBotId}/datasets`);
  const datasets = await datasetsResponse.json();
  renderDatasets(datasets);
}

async function tryResolveTxSuccessFromChain(txHash) {
  try {
    // One quick check first.
    const { resp, data } = await fetchJsonWithTimeout(
      `${apiBase}/api/zerog/tx_status?txHash=${encodeURIComponent(txHash)}`,
      { method: "GET" },
      12000,
    );
    if (resp.ok && data?.status === "success") {
      publishResult.textContent = `链上已成功：${data.txHash}（可查：${data.explorerTxUrl || ""}）。上传已完成或在后台继续同步。`;
      return true;
    }

    // If still pending, poll briefly (some RPCs lag behind the explorer UI).
    const deadline = Date.now() + 45000;
    while (Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 3500));
      const { resp: r2, data: d2 } = await fetchJsonWithTimeout(
        `${apiBase}/api/zerog/tx_status?txHash=${encodeURIComponent(txHash)}`,
        { method: "GET" },
        12000,
      );
      if (r2.ok && d2?.status === "success") {
        publishResult.textContent = `链上已成功：${d2.txHash}（可查：${d2.explorerTxUrl || ""}）。上传已完成或在后台继续同步。`;
        return true;
      }
      if (r2.ok && d2?.status === "failed") {
        publishResult.textContent = `[错误] 交易链上失败：${d2.txHash}（可查：${d2.explorerTxUrl || ""}）`;
        return false;
      }
    }
  } catch {
    // ignore and fall through
  }
  return false;
}

async function uploadSkillsFolder() {
  if (!state.activeBotId) {
    alert("请先选择机器人");
    return;
  }
  const files = Array.from(skillsFolder?.files || []);
  if (!files.length) {
    alert("请选择一个 skills 文件夹");
    return;
  }
  const fd = new FormData();
  for (const f of files) {
    const rel = f.webkitRelativePath || f.name;
    fd.append("files", f, rel);
  }

  if (skillsSummary) skillsSummary.textContent = "正在上传...";
  let resp, data;
  try {
    resp = await fetch(`${apiBase}/api/bots/${state.activeBotId}/skills/upload`, { method: "POST", body: fd });
    data = await resp.json();
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${String(e)}`;
    return;
  }
  if (!resp.ok) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${data?.error || resp.statusText}`;
    return;
  }

  if (skillsFolder) skillsFolder.value = "";

  const listResp = await fetch(`${apiBase}/api/bots/${state.activeBotId}/skills`);
  const list = await listResp.json();
  state.skills = Array.isArray(list) ? list : [];
  renderSkills(state.skills);
}

async function publishSkill(skillID) {
  const ok = await ensureWalletAuthorized();
  if (!ok) return;

  const zgsNodes = (zgsNodesInput?.value || "").trim();
  if (zgsNodesInput) localStorage.setItem("zgsNodes", zgsNodes);
  const authToken = localStorage.getItem("authToken") || "";

  if (!window.ethereum) {
    if (skillsSummary) skillsSummary.textContent = "[错误] 未检测到 MetaMask";
    return;
  }
  const from = localStorage.getItem("walletAddress") || "";
  if (!from) {
    if (skillsSummary) skillsSummary.textContent = "[错误] 未连接钱包";
    return;
  }

  if (skillsSummary) skillsSummary.textContent = "准备链上交易...";
  let prepResp, prep;
  try {
    ({ resp: prepResp, data: prep } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/skills/${skillID}/publish_prepare`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
        },
        body: JSON.stringify({ zgsNodes }),
      },
      45000,
    ));
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] 准备链上交易超时或失败：${String(e?.message || e)}`;
    return;
  }
  if (!prepResp.ok) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${prep?.error || prepResp.statusText}`;
    return;
  }
  if (!prep?.publishId) {
    if (skillsSummary) skillsSummary.textContent = "[错误] 后端未返回 publishId，请重启后端并刷新页面后重试";
    return;
  }

  if (skillsSummary) skillsSummary.textContent = "请在钱包中确认交易...";
  let txHash;
  try {
    txHash = await window.ethereum.request({
      method: "eth_sendTransaction",
      params: [{ from, to: prep.to, data: prep.data, value: prep.value }],
    });
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] 交易发送失败：${String(e?.message || e)}`;
    return;
  }

  if (skillsSummary) skillsSummary.textContent = `交易已发送：${txHash}，等待上传...`;
  let finResp, result;
  try {
    ({ resp: finResp, data: result } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/skills/${skillID}/publish_finalize`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
        },
        body: JSON.stringify({ zgsNodes, publishId: String(prep.publishId), txHash, rootHash: prep.rootHash }),
      },
      120000,
    ));
  } catch (e) {
    const msg = String(e?.message || e);
    if (msg.includes("请求超时") && txHash) {
      // Reuse the same tx-status check; just show in skill area.
      try {
        const { resp, data } = await fetchJsonWithTimeout(
          `${apiBase}/api/zerog/tx_status?txHash=${encodeURIComponent(txHash)}`,
          { method: "GET" },
          12000,
        );
        if (resp.ok && data?.status === "success") {
          if (skillsSummary) skillsSummary.textContent = `链上已成功：${data.txHash}（可查：${data.explorerTxUrl || ""}）。skills 上传可能仍在后台同步。`;
          return;
        }
      } catch {
        // ignore
      }
    }
    if (skillsSummary) skillsSummary.textContent = `[错误] 上传到 0G 超时或失败：${msg}`;
    return;
  }
  if (!finResp.ok) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${result?.error || finResp.statusText}`;
    return;
  }

  if (skillsSummary) skillsSummary.textContent = `Skill 已发布：root=${result.rootHash || ""} tx=${result.txHash || ""}`;

  const listResp = await fetch(`${apiBase}/api/bots/${state.activeBotId}/skills`);
  const list = await listResp.json();
  state.skills = Array.isArray(list) ? list : [];
  renderSkills(state.skills);
}

function appendBubble(role, content) {
  const div = document.createElement("div");
  div.className = `bubble ${role}`;
  div.textContent = content;
  chatStream.append(div);
  chatStream.scrollTop = chatStream.scrollHeight;
  return div;
}

function appendThinkingBubble() {
  const div = document.createElement("div");
  div.className = "bubble assistant thinking";
  div.textContent = "正在思考...";
  chatStream.append(div);
  chatStream.scrollTop = chatStream.scrollHeight;
  return div;
}

function resolveThinking(div, text) {
  if (!div) return;
  div.classList.remove("thinking");
  div.textContent = text;
  chatStream.scrollTop = chatStream.scrollHeight;
}

function escapeHtml(str) {
  return String(str || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function parseJsonOrEmpty(text) {
  const raw = String(text || "").trim();
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch (e) {
    throw new Error(`JSON 解析失败：${String(e?.message || e)}`);
  }
}

function pretty(obj) {
  try {
    return JSON.stringify(obj, null, 2);
  } catch {
    return String(obj);
  }
}

function looksLikeX402SkillContent(content) {
  const raw = String(content || "").trim();
  if (!raw || !raw.startsWith("{")) return false;
  try {
    const v = JSON.parse(raw);
    const t = String(v?.type || "").trim().toLowerCase();
    return t === "x402_fetch" || t === "x402";
  } catch {
    return false;
  }
}

function parseX402Spec(content) {
  const raw = String(content || "").trim();
  const v = JSON.parse(raw);
  const t = String(v?.type || "").trim().toLowerCase();
  if (t !== "x402_fetch" && t !== "x402") return null;
  const url = String(v?.url || "").trim();
  const method = String(v?.method || "GET").trim().toUpperCase();
  const headers = v?.headers && typeof v.headers === "object" ? v.headers : {};
  const body = v?.body ?? null;
  if (!url) return null;
  return { url, method, headers, body };
}

function isoNow() {
  try {
    return new Date().toISOString();
  } catch {
    return "";
  }
}

function truncateTextClient(s, max) {
  s = String(s || "");
  if (s.length <= max) return s;
  return s.slice(0, max) + "...(truncated)";
}

async function runX402SkillsInBrowser(enabledSkillIDs, userMessage) {
  const ids = Array.isArray(enabledSkillIDs) ? enabledSkillIDs : [];
  if (!ids.length) return [];

  const selected = [];
  for (const id of ids) {
    const sk = (state.skills || []).find((x) => x?.id === id);
    if (!sk) continue;
    if (!looksLikeX402SkillContent(sk.content)) continue;
    selected.push(sk);
  }
  if (!selected.length) return [];

  // Hard cap to avoid draining / too many wallet prompts.
  const toRun = selected.slice(0, 3);

  // Ensure wallet is connected (one-time). Payment confirmations still require signature prompts.
  if (!localStorage.getItem("walletAddress")) {
    const ok = await connectWallet();
    if (!ok) throw new Error("需要连接钱包才能执行 x402 支付");
  }
  const from = localStorage.getItem("walletAddress") || "";
  if (!from) throw new Error("未连接钱包");

  const fetchWithPayment = await getX402FetchWithMetaMask(from, { viaProxy: true });

  const results = [];
  for (let i = 0; i < toRun.length; i++) {
    const sk = toRun[i];
    const startedAt = isoNow();
    const filename = String(sk.filename || sk.name || sk.id || "").trim();
    let spec = null;
    try {
      spec = parseX402Spec(sk.content);
    } catch (e) {
      results.push({
        skillId: sk.id,
        filename,
        ok: false,
        status: 0,
        error: `invalid x402 skill json: ${String(e?.message || e)}`,
        startedAt,
        endedAt: isoNow(),
      });
      continue;
    }
    if (!spec) {
      results.push({
        skillId: sk.id,
        filename,
        ok: false,
        status: 0,
        error: "invalid x402 skill spec",
        startedAt,
        endedAt: isoNow(),
      });
      continue;
    }

    const url = spec.url.replaceAll("{input}", encodeURIComponent(String(userMessage || "")));
    const method = spec.method || "GET";
    const headers = spec.headers || {};

    if (llmStatus) {
      llmStatus.textContent = `x402 执行中（${i + 1}/${toRun.length}）：${filename}`;
    }

    try {
      const controller = new AbortController();
      const timeoutMs = 65000;
      const t = setTimeout(() => controller.abort(), timeoutMs);
      let resp;
      try {
        resp = await fetchWithPayment(url, {
          method,
          headers,
          ...(spec.body != null && method !== "GET" && method !== "HEAD" ? { body: JSON.stringify(spec.body) } : {}),
          signal: controller.signal,
        });
      } finally {
        clearTimeout(t);
      }
      const text = await resp.text().catch(() => "");
      results.push({
        skillId: sk.id,
        filename,
        url,
        method,
        ok: resp.ok,
        status: resp.status,
        headers: {
          "content-type": resp.headers.get("content-type") || "",
          "payment-required": resp.headers.get("payment-required") || resp.headers.get("PAYMENT-REQUIRED") || "",
          "payment-response": resp.headers.get("payment-response") || resp.headers.get("PAYMENT-RESPONSE") || "",
        },
        body: truncateTextClient(text, 12000),
        startedAt,
        endedAt: isoNow(),
      });
    } catch (e) {
      results.push({
        skillId: sk.id,
        filename,
        url,
        method,
        ok: false,
        status: 0,
        error: String(e?.message || e),
        startedAt,
        endedAt: isoNow(),
      });
    }
  }
  return results;
}

async function fetchJsonWithTimeout(url, options, timeoutMs) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), Math.max(1, Number(timeoutMs) || 30000));
  try {
    const resp = await fetch(url, { ...(options || {}), signal: controller.signal });
    let data = null;
    try {
      data = await resp.json();
    } catch {
      data = null;
    }
    return { resp, data };
  } catch (e) {
    // Safari/Chromium may show "signal is aborted without reason" for AbortController timeouts.
    const msg = String(e?.message || e);
    if (e?.name === "AbortError" || msg.toLowerCase().includes("aborted")) {
      throw new Error(`请求超时（${Math.round((Number(timeoutMs) || 30000) / 1000)}s）`);
    }
    throw e;
  } finally {
    clearTimeout(timeout);
  }
}

async function sendX402Test() {
  const ok = await ensureWalletAuthorized();
  if (!ok) return;

  const url = (x402Url?.value || "").trim();
  const method = (x402Method?.value || "GET").trim().toUpperCase();
  const timeoutMs = Math.max(1000, Number(x402Timeout?.value || "45000") || 45000);

  if (!url || (!url.startsWith("https://") && !url.startsWith("http://"))) {
    if (x402Output) x402Output.textContent = "[错误] URL 必须以 http:// 或 https:// 开头";
    return;
  }

  let headersObj = null;
  let bodyObj = null;
  try {
    headersObj = parseJsonOrEmpty(x402Headers?.value || "") || {};
    bodyObj = parseJsonOrEmpty(x402Body?.value || "");
  } catch (e) {
    const msg = String(e?.message || e);
    if (x402Output) {
      if (msg.includes("BigInt")) {
        x402Output.textContent =
          `[错误] ${msg}\n` +
          `这通常表示 x402 SDK 没有拿到正确的 PAYMENT-REQUIRED（或其内容缺少 network/amount 等字段）。\n` +
          `请确认 URL 是受保护端点，例如：https://sandbox.agentrails.io/api/x402/protected/analysis\n` +
          `并保持勾选“使用后端转发（解决 CORS）”。`;
        return;
      }
      if (msg.includes("Failed to fetch")) {
        x402Output.textContent =
          `[错误] Failed to fetch\n` +
          `可能原因：\n` +
          `1) URL 不正确（AgentRails sandbox 可用示例：/api/x402/pricing 或 /api/x402/protected/analysis）\n` +
          `2) 目标接口未开启 CORS，浏览器会直接拦截（这种情况必须改成后端代理才能访问）\n` +
          `3) 网络/DNS 问题\n` +
          `原始错误：${msg}`;
      } else {
        x402Output.textContent = `[错误] ${msg}`;
      }
    }
    return;
  }

  if (!window.ethereum?.request) {
    if (x402Output) x402Output.textContent = "[错误] 未检测到 MetaMask";
    return;
  }

  const from = localStorage.getItem("walletAddress") || "";
  if (!from) {
    if (x402Output) x402Output.textContent = "[错误] 未连接钱包";
    return;
  }

  if (x402Output) x402Output.textContent = "请求中（如遇到 402 将自动弹出钱包签名并重试）...";
  let resp;
  try {
    const fetchWithPayment = await getX402FetchWithMetaMask(from, { viaProxy: !!x402UseProxy?.checked });
    const controller = new AbortController();
    const t = setTimeout(() => controller.abort(), timeoutMs);
    try {
      resp = await fetchWithPayment(url, {
        method,
        headers: headersObj,
        ...(bodyObj != null && method !== "GET" && method !== "HEAD" ? { body: JSON.stringify(bodyObj) } : {}),
        signal: controller.signal,
      });

    } finally {
      clearTimeout(t);
    }
  } catch (e) {

    console.log("e",e);
    if (x402Output) x402Output.textContent = `[错误] ${String(e?.message || e)}`;
    return;
  }

  let text = "";
  try {
    text = await resp.text();
  } catch {
    text = "";
  }
  const out = {
    status: resp.status,
    ok: resp.ok,
    headers: {
      "content-type": resp.headers.get("content-type") || "",
      "payment-required": resp.headers.get("payment-required") || resp.headers.get("PAYMENT-REQUIRED") || "",
      "payment-response": resp.headers.get("payment-response") || resp.headers.get("PAYMENT-RESPONSE") || "",
    },
    body: text,
  };
  if (x402Output) x402Output.textContent = pretty(out);
}

let _x402Cache = { key: "", fetchWithPayment: null };

async function getX402FetchWithMetaMask(address, opts) {
  const viaProxy = opts?.viaProxy !== false;
  const addr = String(address || "").toLowerCase();
  const key = `${addr}::${viaProxy ? "proxy" : "direct"}`;
  if (_x402Cache.fetchWithPayment && _x402Cache.key === key) return _x402Cache.fetchWithPayment;

  // Lazy-load x402 browser SDK. This project is no-build vanilla JS, so use an ESM CDN.
  const X402_VER = "2.8.0";
  const fetchModUrls = [
    `https://esm.sh/@x402/fetch@${X402_VER}?bundle`,
    `https://esm.sh/@x402/fetch@${X402_VER}`,
    `https://cdn.jsdelivr.net/npm/@x402/fetch@${X402_VER}/+esm`,
    `https://unpkg.com/@x402/fetch@${X402_VER}?module`,
  ];
  const evmModUrls = [
    `https://esm.sh/@x402/evm@${X402_VER}/exact/client?bundle`,
    `https://esm.sh/@x402/evm@${X402_VER}/exact/client`,
    `https://cdn.jsdelivr.net/npm/@x402/evm@${X402_VER}/exact/client/+esm`,
    `https://unpkg.com/@x402/evm@${X402_VER}/exact/client?module`,
  ];

  const [fetchMod, evmMod] = await Promise.all([importFirst(fetchModUrls), importFirst(evmModUrls)]);
  const { wrapFetchWithPayment, x402Client } = fetchMod || {};
  const { registerExactEvmScheme } = evmMod || {};
  if (typeof wrapFetchWithPayment !== "function" || typeof x402Client !== "function" || typeof registerExactEvmScheme !== "function") {
    throw new Error("x402 SDK 加载失败（CDN 不可用或版本不兼容）。请检查网络能否访问 esm.sh / jsdelivr / unpkg");
  }
  // Minimal viem-like account used by the ExactEvmScheme.
  // It delegates EIP-712 signing to MetaMask via eth_signTypedData_v4.
  const account = {
    address: address,
    async signTypedData({ domain, types, primaryType, message }) {
      // Some wallets require the active chain to match domain.chainId.
      try {
        const wanted = domain?.chainId;
        if (wanted != null) {
          let wantDec = null;
          if (typeof wanted === "bigint") wantDec = Number(wanted);
          else if (typeof wanted === "number") wantDec = wanted;
          else if (typeof wanted === "string") {
            // Some SDKs (or mis-shaped requirements) may accidentally set chainId="eip155:8453".
            const parsed = parseEip155ChainId(wanted);
            wantDec = parsed != null ? parsed : Number(wanted);
          } else {
            wantDec = Number(wanted);
          }
          if (Number.isFinite(wantDec) && wantDec > 0) {
            await ensureEvmChainForSigning(wantDec);
          }
        }
      } catch {
        // ignore chain switching failures; signing may still work.
      }
      const typedData = { domain, types, primaryType, message };
      const sig = await window.ethereum.request({
        method: "eth_signTypedData_v4",
        // MetaMask expects a JSON string; JSON doesn't support BigInt.
        // Convert BigInt values to strings (EIP-712 allows uint/int values as strings).
        params: [address, safeJsonStringify(typedData)],
      });
      return sig;
    },
  };

  const client = new x402Client();
  // Register the Exact EVM scheme. Do not force a wildcard networks list here:
  // some SDK versions try to parse provided networks at registration time, and
  // "eip155:*" can lead to NaN -> BigInt(NaN).
  registerExactEvmScheme(client, {
    signer: account,
    // Explicit networks to cover AgentRails sandbox plus common EVM chains.
    // (Avoid "eip155:*" which can break on some SDK versions.)
    networks: ["eip155:5042002", "eip155:84532", "eip155:11155111", "eip155:8453", "eip155:1"],
  });
  const baseFetch = viaProxy ? makeBackendProxyFetch() : fetch;
  const fetchWithPayment = wrapFetchWithPayment(baseFetch, client);
  _x402Cache = { key, fetchWithPayment };
  return fetchWithPayment;
}

async function importFirst(urls) {
  const list = Array.isArray(urls) ? urls : [];
  let lastErr = null;
  for (const u of list) {
    try {
      // eslint-disable-next-line no-await-in-loop
      return await import(u);
    } catch (e) {
      lastErr = e;
    }
  }
  const msg = String(lastErr?.message || lastErr || "unknown error");
  throw new Error(`Failed to fetch dynamically imported module: ${list[0] || ""} (${msg})`);
}

function makeBackendProxyFetch() {
  return async function proxyFetch(input, init) {
    const url = typeof input === "string" ? input : input?.url;
    const method = (init?.method || "GET").toUpperCase();
    const headersObj = {};
    const h = init?.headers;
    if (h) {
      if (h.forEach) {
        h.forEach((v, k) => {
          headersObj[String(k)] = String(v);
        });
      } else {
        for (const k of Object.keys(h)) headersObj[String(k)] = String(h[k]);
      }
    }

    let bodyText = "";
    const b = init?.body;
    if (typeof b === "string") bodyText = b;

    let token = localStorage.getItem("authToken") || "";
    if (!token) {
      // x402 flow may trigger chain switches; ensure we still have a valid backend auth token.
      await ensureWalletAuthorized();
      token = localStorage.getItem("authToken") || "";
    }
    if (!token) {
      throw new Error("wallet authorization required (connect wallet first)");
    }
    const resp = await fetch(`${apiBase}/api/x402/proxy`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({
        url,
        method,
        headers: headersObj,
        body: bodyText,
        timeoutMs: 65000,
      }),
    });
    const data = await resp.json().catch(() => null);
    if (!resp.ok) {
      throw new Error(data?.error || resp.statusText);
    }
    const status = Number(data?.status || 0) || 0;
    const outHeaders = new Headers();
    const hh = data?.headers || {};
    for (const k of Object.keys(hh)) {
      if (hh[k] != null && String(hh[k]).trim()) outHeaders.set(k, String(hh[k]));
    }

    // Some sellers return slightly different PaymentRequired shapes across versions.
    // Normalize it so the x402 SDK won't crash on BigInt(undefined).
    if ((status || 0) === 402) {
      const pr = outHeaders.get("payment-required") || "";
      const normalized = normalizePaymentRequiredHeader(pr);
      if (normalized && normalized !== pr) {
        outHeaders.set("payment-required", normalized);
        // Keep a debug copy for troubleshooting.
        if (!outHeaders.get("x-payment-required-raw")) outHeaders.set("x-payment-required-raw", pr);
      }
    }

    return new Response(String(data?.body || ""), { status: status || 200, headers: outHeaders });
  };
}

function normalizePaymentRequiredHeader(value) {
  const raw = String(value || "").trim();
  if (!raw) return "";
  const obj = tryDecodePaymentRequired(raw) || tryParseStructuredPaymentRequired(raw);
  if (!obj || typeof obj !== "object") return "";

  // Some servers still use legacy field names (maxAmountRequired) while setting x402Version=2.
  // Normalize to the v2-ish shape expected by newer SDKs: accepts[].amount, accepts[].maxTimeoutSeconds.
  const nowSec = Math.floor(Date.now() / 1000);

  // Most SDKs expect: { accepts: [{ scheme, network, value, ... }], ... }
  // Some providers may use "amount"/"price" fields instead of "value".
  const accepts = Array.isArray(obj.accepts) ? obj.accepts : null;
  if (accepts) {
    for (const a of accepts) {
      if (!a || typeof a !== "object") continue;
      // Many sellers encode the payable amount under different field names.
      // x402 SDKs commonly expect `value` (base units string) or `amount`.
      let amountStr = "";
      if (a.amount != null && String(a.amount).trim() !== "") amountStr = String(a.amount).trim();
      else if (a.maxAmountRequired != null && String(a.maxAmountRequired).trim() !== "") amountStr = String(a.maxAmountRequired).trim();
      else if (a.amountRequired != null && String(a.amountRequired).trim() !== "") amountStr = String(a.amountRequired).trim();
      else if (a.maxAmount != null && String(a.maxAmount).trim() !== "") amountStr = String(a.maxAmount).trim();
      else if (a.value != null && String(a.value).trim() !== "") amountStr = String(a.value).trim();
      else if (a.price != null && String(a.price).trim() !== "") amountStr = normalizeUsdcLikePriceToBaseUnits(String(a.price));
      else if (a.amountUsdc != null && String(a.amountUsdc).trim() !== "") amountStr = normalizeUsdcLikePriceToBaseUnits(String(a.amountUsdc));

      if (amountStr) {
        // v2 field name (per spec / newer SDKs)
        if (a.amount == null || String(a.amount).trim() === "") a.amount = amountStr;
        // some SDKs use `value`
        if (a.value == null || String(a.value).trim() === "") a.value = amountStr;
      }

      // Keep an `amount` alias if the SDK expects it.
      if ((a.amount == null || String(a.amount).trim() === "") && a.value != null && String(a.value).trim() !== "") {
        a.amount = String(a.value).trim();
      }

      // Newer SDKs may require maxTimeoutSeconds to derive validBefore if no explicit expiry is provided.
      if (a.maxTimeoutSeconds == null || !Number.isFinite(Number(a.maxTimeoutSeconds)) || Number(a.maxTimeoutSeconds) <= 0) {
        let ttl = 60;
        const exp = a?.extra?.expiresAt;
        if (typeof exp === "number" && Number.isFinite(exp) && exp > nowSec) ttl = Math.max(1, Math.min(3600, Math.floor(exp - nowSec)));
        else if (typeof exp === "string" && /^\d+$/.test(exp) && Number(exp) > nowSec) ttl = Math.max(1, Math.min(3600, Math.floor(Number(exp) - nowSec)));
        a.maxTimeoutSeconds = ttl;
      }

      // Best-effort decimals defaults. Some sellers omit decimals but use a canonical
      // placeholder asset for USDC-like stablecoins.
      const asset = String(a.asset || "").toLowerCase();
      const token = String(a.token || "").toUpperCase();
      if ((a.decimals == null || !Number.isFinite(Number(a.decimals))) && (token === "USDC" || asset === "0x3600000000000000000000000000000000000000")) {
        a.decimals = 6;
      }
      if ((a.assetDecimals == null || !Number.isFinite(Number(a.assetDecimals))) && Number.isFinite(Number(a.decimals))) {
        a.assetDecimals = Number(a.decimals);
      }

      // Normalize common network variants.
      if ((!a.network || String(a.network).trim() === "") && a.chainId != null) {
        const id = Number(a.chainId);
        if (Number.isFinite(id) && id > 0) a.network = `eip155:${id}`;
      }

      // And the other way around: derive numeric chainId from CAIP-2 network.
      if (a.chainId == null || !Number.isFinite(Number(a.chainId)) || Number(a.chainId) <= 0) {
        const id = parseEip155ChainId(a.network);
        if (id != null) a.chainId = id;
      }

      // Some implementations put EIP-712 domain overrides under `extra`.
      // If chainId is missing there, set it so the signer doesn't see chainId="eip155:...".
      if (a.extra && typeof a.extra === "object") {
        if (a.extra.chainId == null || !Number.isFinite(Number(a.extra.chainId)) || Number(a.extra.chainId) <= 0) {
          const id = parseEip155ChainId(a.network);
          if (id != null) a.extra.chainId = id;
        }
      }
    }
  }

  const encoded = encodePaymentRequired(obj);
  return encoded || "";
}

function parseEip155ChainId(network) {
  const s = String(network || "").trim().toLowerCase();
  const m = /^eip155:(\d+)$/.exec(s);
  if (!m) return null;
  const n = Number(m[1]);
  if (!Number.isFinite(n) || n <= 0) return null;
  return n;
}

function safeJsonStringify(obj) {
  return JSON.stringify(obj, (_k, v) => {
    if (typeof v === "bigint") return v.toString(10);
    return v;
  });
}

function tryDecodePaymentRequired(b64) {
  try {
    const txt = base64ToUtf8(String(b64 || ""));
    if (!txt) return null;
    return JSON.parse(txt);
  } catch {
    return null;
  }
}

// Parse a structured header like:
// scheme="exact";network="eip155:8453";amount="10000";token="USDC";payTo="0x..."
function tryParseStructuredPaymentRequired(raw) {
  const s = String(raw || "").trim();
  if (!s) return null;
  if (!/scheme\s*=/.test(s) || !/amount\s*=/.test(s)) return null;

  const parts = s
    .split(";")
    .map((p) => p.trim())
    .filter(Boolean);
  const kv = {};
  for (const p of parts) {
    const idx = p.indexOf("=");
    if (idx <= 0) continue;
    const k = p.slice(0, idx).trim();
    let v = p.slice(idx + 1).trim();
    if ((v.startsWith("\"") && v.endsWith("\"")) || (v.startsWith("'") && v.endsWith("'"))) v = v.slice(1, -1);
    if (k) kv[k] = v;
  }
  const scheme = kv.scheme || "exact";
  const network = kv.network || kv.chain || "";
  const amount = kv.amount || kv.value || "";
  if (!amount) return null;

  const accept = {
    scheme,
    network,
    value: String(amount),
  };
  if (kv.token) accept.token = kv.token;
  if (kv.payTo) accept.payTo = kv.payTo;
  if (kv.decimals) accept.decimals = kv.decimals;

  // Minimal PaymentRequired shape.
  return {
    accepts: [accept],
  };
}

function encodePaymentRequired(obj) {
  try {
    const txt = JSON.stringify(obj);
    return utf8ToBase64(txt);
  } catch {
    return "";
  }
}

function base64ToUtf8(b64) {
  const norm = normalizeBase64(String(b64 || ""));
  if (!norm) return "";
  // atob returns binary string (latin1).
  const bin = atob(norm);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new TextDecoder("utf-8", { fatal: false }).decode(bytes);
}

function utf8ToBase64(txt) {
  const bytes = new TextEncoder().encode(String(txt || ""));
  let bin = "";
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
  return btoa(bin);
}

function normalizeBase64(s) {
  let v = String(s || "").trim();
  if (!v) return "";
  // base64url -> base64
  v = v.replace(/-/g, "+").replace(/_/g, "/");
  // pad
  const pad = v.length % 4;
  if (pad === 2) v += "==";
  else if (pad === 3) v += "=";
  else if (pad === 1) return "";
  return v;
}

// Best-effort parser: accepts "10000" (base units) or "0.01" / "$0.01".
function normalizeUsdcLikePriceToBaseUnits(input) {
  const s = String(input || "").trim();
  if (!s) return "";
  const cleaned = s.replace(/^\$/, "").trim();
  // If it's already an integer string, return as-is.
  if (/^\d+$/.test(cleaned)) return cleaned;
  // Decimal: convert to 6-decimal base units (USDC style).
  if (!/^\d+(\.\d+)?$/.test(cleaned)) return "";
  const [whole, fracRaw = ""] = cleaned.split(".");
  const frac = (fracRaw + "000000").slice(0, 6);
  const out = `${whole}${frac}`.replace(/^0+(\d)/, "$1");
  return out ? out : "0";
}

async function ensureEvmChainForSigning(chainIdDec) {
  if (!window.ethereum?.request) return;
  const wantHex = "0x" + Number(chainIdDec).toString(16);
  const cur = await window.ethereum.request({ method: "eth_chainId" });
  if (String(cur).toLowerCase() === wantHex.toLowerCase()) return;

  // If we don't know how to add this chain (no RPC info), don't attempt switching.
  // Otherwise MetaMask will throw 4902 and may show a scary error UI.
  const preset = knownChainPreset(Number(chainIdDec));
  if (!preset) return;

  try {
    await window.ethereum.request({ method: "wallet_switchEthereumChain", params: [{ chainId: wantHex }] });
    return;
  } catch (e) {
    const code = Number(e?.code ?? e?.data?.originalError?.code);
    if (code !== 4902) throw e;
  }

  await window.ethereum.request({ method: "wallet_addEthereumChain", params: [preset] });
  // MetaMask typically switches on add, but do a best-effort switch to be sure.
  try {
    await window.ethereum.request({ method: "wallet_switchEthereumChain", params: [{ chainId: wantHex }] });
  } catch {}
}

function knownChainPreset(chainIdDec) {
  // Minimal presets to make x402 paid APIs usable out of the box.
  // If the seller uses a different chain, user can add/switch manually.
  if (chainIdDec === 5042002) {
    return {
      chainId: "0x4cef52",
      chainName: "Arc Testnet",
      // Arc uses USDC as the native gas token; decimals are chain-specific.
      nativeCurrency: { name: "USDC", symbol: "USDC", decimals: 18 },
      rpcUrls: [
        "https://rpc.testnet.arc.network",
        "https://rpc.blockdaemon.testnet.arc.network",
        "https://rpc.drpc.testnet.arc.network",
        "https://rpc.quicknode.testnet.arc.network",
      ],
      blockExplorerUrls: ["https://testnet.arcscan.app"],
    };
  }
  if (chainIdDec === 8453) {
    return {
      chainId: "0x2105",
      chainName: "Base",
      nativeCurrency: { name: "ETH", symbol: "ETH", decimals: 18 },
      rpcUrls: ["https://mainnet.base.org"],
      blockExplorerUrls: ["https://basescan.org"],
    };
  }
  if (chainIdDec === 84532) {
    return {
      chainId: "0x14a34",
      chainName: "Base Sepolia",
      nativeCurrency: { name: "ETH", symbol: "ETH", decimals: 18 },
      rpcUrls: ["https://sepolia.base.org"],
      blockExplorerUrls: ["https://sepolia.basescan.org"],
    };
  }
  if (chainIdDec === 1) {
    return {
      chainId: "0x1",
      chainName: "Ethereum Mainnet",
      nativeCurrency: { name: "ETH", symbol: "ETH", decimals: 18 },
      rpcUrls: ["https://cloudflare-eth.com"],
      blockExplorerUrls: ["https://etherscan.io"],
    };
  }
  if (chainIdDec === 11155111) {
    return {
      chainId: "0xaa36a7",
      chainName: "Sepolia",
      nativeCurrency: { name: "ETH", symbol: "ETH", decimals: 18 },
      rpcUrls: ["https://rpc.sepolia.org"],
      blockExplorerUrls: ["https://sepolia.etherscan.io"],
    };
  }
  return null;
}

function restoreX402PanelState() {
  if (!x402Box || !x402Toggle) return;
  const collapsed = localStorage.getItem("x402PanelCollapsed");
  const isCollapsed = collapsed === null ? true : collapsed !== "0";
  x402Box.classList.toggle("collapsed", isCollapsed);
  x402Toggle.setAttribute("aria-expanded", isCollapsed ? "false" : "true");
}

function toggleX402Panel() {
  if (!x402Box || !x402Toggle) return;
  const next = !x402Box.classList.contains("collapsed");
  // next === true means currently expanded; we want to collapse.
  x402Box.classList.toggle("collapsed", next);
  x402Toggle.setAttribute("aria-expanded", next ? "false" : "true");
  localStorage.setItem("x402PanelCollapsed", next ? "1" : "0");
}

function restoreZgsNodes() {
  if (!zgsNodesInput) return;
  zgsNodesInput.value = localStorage.getItem("zgsNodes") || "";
}

async function prefillZeroGConfig() {
  if (!zgsNodesInput) return;
  if ((zgsNodesInput.value || "").trim()) return;
  try {
    const resp = await fetch(`${apiBase}/api/zerog/config`);
    const data = await resp.json();
    if (!resp.ok) return;
    const nodes = (data?.zgsNodes || "").trim();
    if (nodes) {
      zgsNodesInput.value = nodes;
      localStorage.setItem("zgsNodes", nodes);
    }
  } catch {
    // ignore
  }
}

function renderWalletStatus() {
  if (!walletStatus) return;
  const addr = localStorage.getItem("walletAddress") || "";
  const hasToken = !!(localStorage.getItem("authToken") || "");
  if (walletConnectBtn) {
    walletConnectBtn.textContent = addr ? "退出登录" : "登录钱包";
  }
  walletStatus.textContent = addr
    ? `钱包地址：${addr}${hasToken ? "（已授权）" : "（未授权）"}`
    : "未连接钱包";
}

function shortAddr(addr) {
  if (!addr || addr.length < 10) return addr;
  return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
}

function bindWalletEvents() {
  if (!window.ethereum?.on) return;
  window.ethereum.on("accountsChanged", (accounts) => {
    const next = accounts?.[0] || "";
    if (!next) {
      localStorage.removeItem("walletAddress");
      localStorage.removeItem("authToken");
      renderWalletStatus();
      return;
    }
    // Account switch invalidates the previous signature/session.
    localStorage.setItem("walletAddress", next);
    localStorage.removeItem("authToken");
    renderWalletStatus();
  });
  window.ethereum.on("chainChanged", () => {
    // Do not clear authToken on chain changes: x402 payments may require switching
    // to a different chain for signing, but our backend auth token is address-based.
    renderWalletStatus();
  });
}

function logoutLocal() {
  localStorage.removeItem("walletAddress");
  localStorage.removeItem("authToken");
  renderWalletStatus();
}

async function handleWalletButtonClick(e) {
  // Ensure this click is treated as a direct user gesture for MetaMask.
  // Some browsers are picky about popups/permissions.
  // (Even though the button is type="button", keep this defensive.)
  try {
    e?.preventDefault?.();
    e?.stopPropagation?.();
  } catch {}

  // Toggle behavior for the button only.
  if (localStorage.getItem("walletAddress")) {
    logoutLocal();
    return;
  }
  await connectWallet();
}

async function syncWalletFromProvider() {
  if (!window.ethereum?.request) return;
  try {
    const accounts = await window.ethereum.request({ method: "eth_accounts" });
    const address = accounts?.[0] || "";
    if (!address) return;
    localStorage.setItem("walletAddress", address);
    renderWalletStatus();
  } catch {
    // ignore
  }
}

async function ensureWalletAuthorized() {
  const addr = localStorage.getItem("walletAddress") || "";
  const token = localStorage.getItem("authToken") || "";
  if (addr && token) return true;

  // Bring wallet section into view for the user.
  walletBox?.scrollIntoView({ behavior: "smooth", block: "start" });
  if (walletStatus) walletStatus.textContent = "发布需要钱包授权，请先登录钱包...";

  const ok = await connectWallet();
  if (!ok) return false;

  const token2 = localStorage.getItem("authToken") || "";
  if (!token2) {
    if (walletStatus) walletStatus.textContent = "未完成授权签名，无法发布";
    return false;
  }
  return true;
}

async function connectWallet() {
  try {
    if (walletStatus) walletStatus.textContent = "正在连接钱包...";

    if (!window.ethereum) {
      alert("未检测到 MetaMask，请先安装浏览器钱包插件");
      if (walletStatus) walletStatus.textContent = "未检测到 MetaMask";
      return false;
    }

    // Fetch chain id from backend (source of truth: ZERO_G_EVM_RPC).
    let targetChainId = "0x40da";
    let rpcUrl = "https://evmrpc-testnet.0g.ai";
    let explorerUrl = "https://chainscan-galileo.0g.ai";
    try {
      const resp = await fetch(`${apiBase}/api/zerog/chaininfo`);
      const data = await resp.json();
      if (resp.ok) {
        if (data?.chainIdHex) targetChainId = String(data.chainIdHex);
        if (data?.evmRpc) rpcUrl = String(data.evmRpc);
        if (data?.explorerBase) explorerUrl = String(data.explorerBase);
      }
    } catch {
      // ignore
    }

    // Request permission explicitly first. In some environments, this is more reliable
    // than only calling eth_requestAccounts.
    try {
      await window.ethereum.request({
        method: "wallet_requestPermissions",
        params: [{ eth_accounts: {} }],
      });
    } catch {
      // Not all providers support it; fall back to eth_requestAccounts.
    }

    const accounts = await window.ethereum.request({ method: "eth_requestAccounts" });
    const address = accounts?.[0];
    if (!address) {
      alert("未获取到钱包地址");
      if (walletStatus) walletStatus.textContent = "未获取到钱包地址";
      return false;
    }

    const currentChainId = await window.ethereum.request({ method: "eth_chainId" });
    if (currentChainId !== targetChainId) {
      try {
        await window.ethereum.request({
          method: "wallet_switchEthereumChain",
          params: [{ chainId: targetChainId }],
        });
      } catch (error) {
        // 4902: unknown chain
        if (error?.code === 4902) {
          try {
            await window.ethereum.request({
              method: "wallet_addEthereumChain",
              params: [
                {
                  chainId: targetChainId,
                  chainName: "0GAI",
                  nativeCurrency: { name: "OG", symbol: "OG", decimals: 18 },
                  rpcUrls: [rpcUrl],
                  blockExplorerUrls: [explorerUrl],
                },
              ],
            });
          } catch (addErr) {
            // MetaMask may refuse adding a chain if another chain already uses the same RPC.
            // In that case, just ask user to switch to the existing 0GAI network and continue.
            const msg = String(addErr?.message || addErr);
            if (msg.includes("same RPC endpoint") && msg.includes("0x40da")) {
              await window.ethereum.request({
                method: "wallet_switchEthereumChain",
                params: [{ chainId: "0x40da" }],
              });
            } else {
              throw addErr;
            }
          }
        } else {
          alert(`切换网络失败：${String(error?.message || error)}`);
          if (walletStatus) walletStatus.textContent = "切换网络失败";
          return false;
        }
      }
    }

    const chainId = await window.ethereum.request({ method: "eth_chainId" });
    const chainIdDec = String(parseInt(chainId, 16));

    const nonceResp = await fetch(
      `${apiBase}/api/auth/nonce?address=${encodeURIComponent(address)}&chainId=${encodeURIComponent(chainIdDec)}`,
    );
    const nonceData = await nonceResp.json();
    if (!nonceResp.ok) {
      alert(`[授权失败] ${nonceData?.error || nonceResp.statusText}`);
      if (walletStatus) walletStatus.textContent = "授权失败";
      return false;
    }

    const message = nonceData.message;
    if (walletStatus) walletStatus.textContent = "请在钱包中签名...";
    const signature = await window.ethereum.request({
      method: "personal_sign",
      params: [message, address],
    });

    const verifyResp = await fetch(`${apiBase}/api/auth/verify`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ address, message, signature }),
    });
    const verifyData = await verifyResp.json();
    if (!verifyResp.ok) {
      alert(`[授权失败] ${verifyData?.error || verifyResp.statusText}`);
      if (walletStatus) walletStatus.textContent = "授权失败";
      return false;
    }

    localStorage.setItem("walletAddress", verifyData.address || address);
    localStorage.setItem("authToken", verifyData.token);
    renderWalletStatus();
    return true;
  } catch (error) {
    console.error(error);
    alert(`钱包连接失败：${String(error?.message || error)}`);
    if (walletStatus) walletStatus.textContent = "钱包连接失败";
    return false;
  }
}
