const apiBase = "";

const state = {
  activeBotId: "",
  activeBot: null,
  bots: [],
  datasets: [],
  distillation: null,
  infts: [],
  publishingINFTIDs: new Set(),
  registeringINFTIDs: new Set(),
  skills: [],
  selectedSkillIDs: new Set(),
};

const botForm = document.querySelector("#bot-form");
const botList = document.querySelector("#bot-list");
const chatForm = document.querySelector("#chat-form");
const chatInput = document.querySelector("#chat-input");
const chatStream = document.querySelector("#chat-stream");
const activeBotName = document.querySelector("#active-bot-name");
const growthScore = document.querySelector("#growth-score");
const llmStatus = document.querySelector("#llm-status");
const datasetList = document.querySelector("#dataset-list");
const memoryOverview = document.querySelector("#memory-overview");
const publishBtn = document.querySelector("#publish-btn");
const datasetExportSkillsBtn = document.querySelector("#dataset-export-skills");
const memoryDistillRunBtn = document.querySelector("#memory-distill-run");
const memoryDistillSaveBtn = document.querySelector("#memory-distill-save");
const memoryDistillStatus = document.querySelector("#memory-distill-status");
const memoryDistillResult = document.querySelector("#memory-distill-result");
const inftCreateTrainingBtn = document.querySelector("#inft-create-training");
const inftCreateDistilledBtn = document.querySelector("#inft-create-distilled");
const inftStatus = document.querySelector("#inft-status");
const inftList = document.querySelector("#inft-list");
const publishResult = document.querySelector("#publish-result");
const zgsNodesInput = document.querySelector("#zgs-nodes");
const skillsSummary = document.querySelector("#skills-summary");
const skillsList = document.querySelector("#skills-list");
const skillsLocalFiles = document.querySelector("#skills-local-files");
const skillsLocalUploadBtn = document.querySelector("#skills-local-upload");
const skillsClearSelectionBtn = document.querySelector("#skills-clear-selection");
const skillsDeleteSelectedBtn = document.querySelector("#skills-delete-selected");
const skillsGitHubURL = document.querySelector("#skills-github-url");
const skillsGitHubImportPublishBtn = document.querySelector("#skills-github-import-publish");
const skillsPublishBundleBtn = document.querySelector("#skills-publish-bundle");
const llmApiKey = document.querySelector("#llm-apikey");
const debugSkills = document.querySelector("#debug-skills");
const botModelPreset = document.querySelector("#bot-model-preset");
const botModelCustom = document.querySelector("#bot-model-custom");
const botModelProvider = document.querySelector("#bot-model-provider");
const botModelBaseUrl = document.querySelector("#bot-model-base-url");
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
const confirmDialog = document.querySelector("#confirm-dialog");
const confirmDialogBackdrop = document.querySelector("#confirm-dialog-backdrop");
const confirmDialogHeading = document.querySelector("#confirm-dialog-heading");
const confirmDialogMessage = document.querySelector("#confirm-dialog-message");
const confirmDialogCancelBtn = document.querySelector("#confirm-dialog-cancel");
const confirmDialogConfirmBtn = document.querySelector("#confirm-dialog-confirm");

let activeConfirmDialog = null;

function normalizeSkillFilename(filename) {
  let out = String(filename || "").trim().replaceAll("\\", "/");
  while (out.startsWith("./")) out = out.slice(2);
  out = out.replace(/^\/+/, "");
  return out;
}

function skillDisplayNameFromFilename(filename) {
  const normalized = normalizeSkillFilename(filename);
  if (!normalized) return "skill";
  const base = normalized.split("/").pop() || normalized;
  const idx = base.lastIndexOf(".");
  return idx > 0 ? base.slice(0, idx) : base;
}

function getEnabledSkillIDsFromSelection() {
  return Array.from(state.selectedSkillIDs || []).filter(Boolean);
}

function syncSkillSelectionActions() {
  const count = state.selectedSkillIDs.size;
  if (skillsClearSelectionBtn) {
    skillsClearSelectionBtn.disabled = count === 0;
  }
  if (skillsDeleteSelectedBtn) {
    skillsDeleteSelectedBtn.disabled = count === 0;
    skillsDeleteSelectedBtn.textContent = count > 0 ? `删除已选 Skills（${count}）` : "删除已选 Skills";
  }
}

function syncINFTActions() {
  const datasetCount = Array.isArray(state.datasets) ? state.datasets.length : 0;
  const hasDistilled = !!String(state.distillation?.memorySummary || "").trim();
  if (inftCreateTrainingBtn) {
    inftCreateTrainingBtn.disabled = datasetCount === 0;
    inftCreateTrainingBtn.textContent = datasetCount > 0 ? `训练数据制作成 iNFT（${datasetCount}）` : "训练数据制作成 iNFT";
  }
  if (inftCreateDistilledBtn) {
    inftCreateDistilledBtn.disabled = !hasDistilled;
  }
}

function syncDatasetActions() {
  const count = Array.isArray(state.datasets) ? state.datasets.length : 0;
  if (publishBtn) {
    publishBtn.disabled = count === 0;
  }
  if (datasetExportSkillsBtn) {
    datasetExportSkillsBtn.disabled = count === 0;
    datasetExportSkillsBtn.textContent = count > 0 ? `导出训练数据为 Skills（${count}）` : "导出训练数据为 Skills";
  }
  syncINFTActions();
  syncDistillationActions();
}

function syncDistillationActions() {
  const datasetCount = Array.isArray(state.datasets) ? state.datasets.length : 0;
  if (memoryDistillRunBtn) {
    memoryDistillRunBtn.disabled = datasetCount === 0;
    memoryDistillRunBtn.textContent = datasetCount > 0 ? `0G Compute 蒸馏记忆（${Math.min(datasetCount, 12)}）` : "0G Compute 蒸馏记忆";
  }
}

function pruneSelectedSkillIDs(skillIDSet) {
  for (const id of Array.from(state.selectedSkillIDs)) {
    if (!skillIDSet.has(id)) state.selectedSkillIDs.delete(id);
  }
}

function summarizeSkillNames(names) {
  const list = Array.isArray(names) ? names.filter(Boolean) : [];
  if (!list.length) return "";
  const head = list.slice(0, 3).join("、");
  return list.length > 3 ? `${head} 等 ${list.length} 个文件` : head;
}

function findDuplicateSkillFilenames(names) {
  const seen = new Set();
  const duplicates = [];
  for (const rawName of Array.isArray(names) ? names : []) {
    const name = normalizeSkillFilename(rawName);
    if (!name) continue;
    if (seen.has(name)) {
      if (!duplicates.includes(name)) duplicates.push(name);
      continue;
    }
    seen.add(name);
  }
  return duplicates;
}

function readAttachmentFilename(contentDisposition, fallback) {
  const raw = String(contentDisposition || "").trim();
  if (!raw) return fallback;

  const utf8Match = raw.match(/filename\*\s*=\s*UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]);
    } catch {
      return utf8Match[1];
    }
  }

  const plainMatch = raw.match(/filename\s*=\s*\"?([^\";]+)\"?/i);
  if (plainMatch?.[1]) {
    return plainMatch[1];
  }
  return fallback;
}

function downloadBlobFile(blob, filename) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = String(filename || "download");
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

function formatDateTime(value) {
  const raw = String(value || "").trim();
  if (!raw) return "未记录";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw;
  try {
    return d.toLocaleString("zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return d.toISOString();
  }
}

function getCurrentProofTimeLabel() {
  return formatDateTime(new Date().toISOString());
}

function shortenMiddle(value, head = 10, tail = 8) {
  const s = String(value || "").trim();
  if (!s) return "";
  if (s.length <= head + tail + 3) return s;
  return `${s.slice(0, head)}...${s.slice(-tail)}`;
}

function renderMemoryOverview() {
  if (!memoryOverview) return;
  const datasets = Array.isArray(state.datasets) ? state.datasets : [];
  const skills = Array.isArray(state.skills) ? state.skills : [];
  const infts = Array.isArray(state.infts) ? state.infts : [];
  const verifiedMemories = datasets.filter((s) => s?.storedOn0G).length;
  const pendingMemories = datasets.filter((s) => s?.uploadPending).length;
  const verifiedSkills = skills.filter((s) => s?.storedOn0G).length;
  const verifiedINFTs = infts.filter((s) => s?.storedOn0G).length;
  const registeredINFTs = infts.filter((s) => s?.registryRegistered).length;
  const totalVerifiableAssets = verifiedMemories + verifiedSkills + verifiedINFTs;
  const latestProof = totalVerifiableAssets > 0 || pendingMemories > 0 || registeredINFTs > 0 ? getCurrentProofTimeLabel() : "未上链";

  const cards = [
    { label: "Memory Samples", value: String(datasets.length), detail: "每轮对话都会沉淀为长期记忆" },
    {
      label: "Verifiable on 0G",
      value: String(totalVerifiableAssets),
      detail:
        pendingMemories > 0
          ? `Memory ${verifiedMemories} / Skills ${verifiedSkills} / iNFT ${verifiedINFTs}；还有 ${pendingMemories} 条同步中`
          : `Memory ${verifiedMemories} / Skills ${verifiedSkills} / iNFT ${verifiedINFTs}；Chain 登记 ${registeredINFTs}`,
    },
    { label: "Skill Assets", value: String(skills.length), detail: "支持导入、导出、发布与复用" },
    { label: "iNFT Assets", value: String(infts.length), detail: "训练数据与蒸馏记忆都可资产化" },
    { label: "Latest Proof", value: latestProof, detail: "从 Chat -> Memory -> Skills -> 0G" },
  ];

  memoryOverview.innerHTML = cards
    .map(
      (card) => `
        <article class="memory-card ${card.label === "Latest Proof" ? "memory-card-wide" : ""}">
          <span>${escapeHtml(card.label)}</span>
          <strong>${escapeHtml(card.value)}</strong>
          <small>${escapeHtml(card.detail)}</small>
        </article>
      `,
    )
    .join("");
}

function resetDistillationState() {
  state.distillation = null;
  if (memoryDistillStatus) memoryDistillStatus.textContent = "";
  renderDistillationResult();
}

function stringifyDistillValue(value) {
  if (Array.isArray(value)) return value.map((v) => String(v || "").trim()).filter(Boolean).join(", ");
  return String(value || "").trim();
}

function renderDistillationResult() {
  if (!memoryDistillResult) return;
  syncDistillationActions();
  syncINFTActions();

  const result = state.distillation;
  if (!result) {
    memoryDistillResult.innerHTML = `
      <div class="distill-empty">
        <small>点击“0G Compute 蒸馏记忆”，把原始对话记忆整理成一段简洁的记忆摘要。</small>
      </div>
    `;
    return;
  }

  memoryDistillResult.innerHTML = `
    <section class="distill-section">
      <strong>记忆摘要</strong>
      <small>${escapeHtml(result.memorySummary || "暂无摘要")}</small>
    </section>
  `;
}

function closeConfirmDialog(confirmed) {
  if (!activeConfirmDialog) return;
  const { resolve } = activeConfirmDialog;
  activeConfirmDialog = null;
  if (confirmDialog) {
    confirmDialog.classList.add("hidden");
    confirmDialog.setAttribute("aria-hidden", "true");
  }
  if (confirmDialogConfirmBtn) {
    confirmDialogConfirmBtn.classList.remove("danger");
  }
  resolve(Boolean(confirmed));
}

function showConfirmDialog({ title = "请确认", message = "", confirmLabel = "继续", cancelLabel = "取消", danger = false } = {}) {
  if (!confirmDialog || !confirmDialogConfirmBtn || !confirmDialogCancelBtn || !confirmDialogHeading || !confirmDialogMessage) {
    return Promise.resolve(false);
  }
  if (activeConfirmDialog) {
    activeConfirmDialog.resolve(false);
    activeConfirmDialog = null;
  }

  confirmDialogHeading.textContent = String(title || "请确认");
  confirmDialogMessage.textContent = String(message || "");
  confirmDialogConfirmBtn.textContent = String(confirmLabel || "继续");
  confirmDialogCancelBtn.textContent = String(cancelLabel || "取消");
  confirmDialogConfirmBtn.classList.toggle("danger", !!danger);
  confirmDialog.classList.remove("hidden");
  confirmDialog.setAttribute("aria-hidden", "false");

  return new Promise((resolve) => {
    activeConfirmDialog = { resolve };
    setTimeout(() => {
      confirmDialogConfirmBtn.focus();
    }, 0);
  });
}

if (skillsList) {
  skillsList.addEventListener("change", (e) => {
    const target = e.target;
    if (!(target instanceof HTMLInputElement)) return;
    const skillID = target.dataset?.skillCheck;
    if (!skillID) return;
    if (target.checked) state.selectedSkillIDs.add(skillID);
    else state.selectedSkillIDs.delete(skillID);
    syncSkillSelectionActions();
  });
}

if (inftList) {
  inftList.addEventListener("click", async (e) => {
    const target = e.target;
    if (!(target instanceof HTMLButtonElement)) return;
    const inftID = target.dataset?.publishInft;
    const registerINFTID = target.dataset?.registerInft;
    if (inftID) {
      await publishINFT(inftID);
      return;
    }
    if (registerINFTID) {
      await registerINFT(registerINFTID);
    }
  });
}

botForm.addEventListener("submit", handleBotSubmit);
botList.addEventListener("change", handleBotSwitch);
chatForm.addEventListener("submit", handleChatSubmit);
publishBtn.addEventListener("click", publishDatasets);
datasetExportSkillsBtn?.addEventListener("click", exportDatasetsAsSkills);
memoryDistillRunBtn?.addEventListener("click", distillMemoriesWith0GCompute);
inftCreateTrainingBtn?.addEventListener("click", createTrainingINFT);
inftCreateDistilledBtn?.addEventListener("click", createDistilledINFT);
skillsLocalUploadBtn?.addEventListener("click", uploadLocalSkills);
skillsClearSelectionBtn?.addEventListener("click", clearSelectedSkillSelections);
skillsDeleteSelectedBtn?.addEventListener("click", deleteSelectedSkills);
skillsGitHubImportPublishBtn?.addEventListener("click", importAndPublishSkillsFromGitHub);
skillsPublishBundleBtn?.addEventListener("click", publishSkillsBundle);
botModelPreset?.addEventListener("change", syncBotModelPreset);
botModelProvider?.addEventListener("change", () => {
  if (!botModelBaseUrl) return;
  const current = String(botModelBaseUrl.value || "").trim();
  if (!current) {
    botModelBaseUrl.value = providerDefaultBaseUrl(botModelProvider.value);
  }
});
confirmDialogBackdrop?.addEventListener("click", () => closeConfirmDialog(false));
confirmDialogCancelBtn?.addEventListener("click", () => closeConfirmDialog(false));
confirmDialogConfirmBtn?.addEventListener("click", () => closeConfirmDialog(true));
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && activeConfirmDialog) {
    event.preventDefault();
    closeConfirmDialog(false);
  }
});
walletConnectBtn?.addEventListener("click", handleWalletButtonClick);
x402Send?.addEventListener("click", sendX402Test);
x402Toggle?.addEventListener("click", toggleX402Panel);

syncDatasetActions();
syncSkillSelectionActions();
renderMemoryOverview();
renderDistillationResult();
if (botModelProvider && !botModelProvider.value) botModelProvider.value = "openai_compat";
if (botModelBaseUrl && !String(botModelBaseUrl.value || "").trim()) {
  botModelBaseUrl.value = providerDefaultBaseUrl(botModelProvider?.value || "");
}
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
    const option = botModelPreset.selectedOptions?.[0];
    if (botModelProvider) {
      botModelProvider.value = option?.dataset?.provider || inferProviderFromModel(chosen);
    }
    if (botModelBaseUrl) {
      botModelBaseUrl.value = option?.dataset?.baseUrl || inferBaseUrlFromModel(chosen, botModelProvider?.value || "");
    }
  }
}

function providerDefaultBaseUrl(provider) {
  const p = String(provider || "").trim();
  if (p === "anthropic") return "https://api.anthropic.com/v1";
  return "";
}

function inferProviderFromModel(model) {
  const m = String(model || "").trim();
  const low = m.toLowerCase();
  if (!m) return "openai_compat";
  if (low.startsWith("claude")) return "anthropic";
  return "openai_compat";
}

function inferBaseUrlFromModel(model, provider = "") {
  const m = String(model || "").trim();
  const low = m.toLowerCase();
  const p = String(provider || "").trim() || inferProviderFromModel(m);
  if (!m) return providerDefaultBaseUrl(p);
  if (p === "anthropic") return "https://api.anthropic.com/v1";
  if (low.startsWith("deepseek-")) return "https://api.deepseek.com/v1";
  if (low.startsWith("grok-")) return "https://api.x.ai/v1";
  if (low.startsWith("gemini-")) return "https://generativelanguage.googleapis.com/v1beta/openai";
  if (low.startsWith("qwen-")) return "https://dashscope-intl.aliyuncs.com/compatible-mode/v1";
  if (low.startsWith("kimi-") || low.startsWith("moonshot-")) return "https://api.moonshot.cn/v1";
  if (low.startsWith("glm-")) return "https://open.bigmodel.cn/api/paas/v4";
  if (m.startsWith("MiniMax-") || low.startsWith("minimax-")) return "https://api.minimax.io/v1";
  if (low.startsWith("gpt-") || low.startsWith("o1") || low.startsWith("o3") || low.startsWith("o4")) return "https://api.openai.com/v1";
  return providerDefaultBaseUrl(p) || "https://api.openai.com/v1";
}

function buildBotLLMConfig() {
  const apiKey = llmApiKey?.value?.trim() || "";
  const model = String(state.activeBot?.modelType || "").trim();
  if (!apiKey || !model) return null;

  const provider = String(state.activeBot?.modelProvider || "").trim() || inferProviderFromModel(model);
  const baseUrl = String(state.activeBot?.modelBaseUrl || "").trim() || inferBaseUrlFromModel(model, provider);

  return {
    apiKey,
    provider,
    baseUrl,
    model,
    temperature: 0.7,
    maxTokens: 2048,
  };
}

async function fetchJson(url, options) {
  let resp;
  try {
    resp = await fetch(url, options);
  } catch (e) {
    throw new Error(`网络请求失败：${String(e?.message || e)}`);
  }

  let data = null;
  try {
    data = await resp.json();
  } catch {
    data = null;
  }

  if (!resp.ok) {
    throw new Error(data?.error || resp.statusText || `HTTP ${resp.status}`);
  }
  return data;
}

async function fetchJsonTimed(url, options, timeoutMs) {
  const { resp, data } = await fetchJsonWithTimeout(url, options, timeoutMs);
  if (!resp.ok) {
    throw new Error(data?.error || resp.statusText || `HTTP ${resp.status}`);
  }
  return data;
}

async function refreshSkills() {
  if (!state.activeBotId) {
    state.skills = [];
    renderSkills([]);
    return [];
  }
  const list = await fetchJson(`${apiBase}/api/bots/${state.activeBotId}/skills`);
  state.skills = Array.isArray(list) ? list : [];
  renderSkills(state.skills);
  return state.skills;
}

async function refreshDatasets() {
  if (!state.activeBotId) {
    state.datasets = [];
    renderDatasets([]);
    return [];
  }
  const datasets = await fetchJson(`${apiBase}/api/bots/${state.activeBotId}/datasets`);
  const out = Array.isArray(datasets) ? datasets : [];
  renderDatasets(out);
  return state.datasets;
}

async function refreshINFTs() {
  if (!state.activeBotId) {
    state.infts = [];
    renderINFTs([]);
    return [];
  }
  const list = await fetchJson(`${apiBase}/api/bots/${state.activeBotId}/infts`);
  state.infts = Array.isArray(list) ? list : [];
  renderINFTs(state.infts);
  return state.infts;
}

async function loadBots() {
  let bots;
  try {
    bots = await fetchJson(`${apiBase}/api/bots`);
  } catch (e) {
    const msg = String(e?.message || e);
    if (llmStatus) llmStatus.textContent = `[错误] ${msg}`;
    state.bots = [];
    botList.innerHTML = "";
    const option = document.createElement("option");
    option.textContent = "暂无机器人";
    option.value = "";
    botList.append(option);
    state.activeBotId = "";
    renderActiveBot();
    renderDatasets([]);
    renderSkills([]);
    renderINFTs([]);
    resetDistillationState();
    return;
  }
  state.bots = Array.isArray(bots) ? bots : [];

  botList.innerHTML = "";
  if (!state.bots.length) {
    const option = document.createElement("option");
    option.textContent = "暂无机器人";
    option.value = "";
    botList.append(option);
    state.activeBotId = "";
    renderActiveBot();
    renderDatasets([]);
    renderINFTs([]);
    resetDistillationState();
    return;
  }

  state.bots.forEach((bot) => {
    const option = document.createElement("option");
    option.value = bot.id;
    option.textContent = `${bot.name || bot.id} (${bot.modelType || "default"})`;
    botList.append(option);
  });

  if (!state.activeBotId || !state.bots.some((bot) => bot.id === state.activeBotId)) {
    state.activeBotId = state.bots[0].id;
  }
  botList.value = state.activeBotId;
  await refreshActiveBotViews();
}

async function handleBotSubmit(event) {
  event.preventDefault();
  syncBotModelPreset();
  const formData = new FormData(botForm);
  const payload = Object.fromEntries(formData.entries());
  let bot;
  try {
    bot = await fetchJson(`${apiBase}/api/bots`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
  } catch (e) {
    const msg = String(e?.message || e);
    if (llmStatus) llmStatus.textContent = `[错误] ${msg}`;
    return;
  }
  state.activeBotId = bot.id;
  botForm.reset();
  await loadBots();
}

async function handleBotSwitch() {
  state.activeBotId = botList.value;
  resetDistillationState();
  await refreshActiveBotViews();
}

async function refreshActiveBotViews() {
  if (!state.activeBotId) {
    renderActiveBot();
    renderDatasets([]);
    renderSkills([]);
    renderINFTs([]);
    resetDistillationState();
    return;
  }

  let bot;
  let memories;
  try {
    [bot, memories] = await Promise.all([
      fetchJson(`${apiBase}/api/bots/${state.activeBotId}`),
      fetchJson(`${apiBase}/api/bots/${state.activeBotId}/memories`),
    ]);
    await Promise.all([refreshDatasets(), refreshSkills(), refreshINFTs()]);
  } catch (e) {
    const msg = String(e?.message || e);
    if (llmStatus) llmStatus.textContent = `[错误] ${msg}`;
    return;
  }

  state.activeBot = bot;
  renderActiveBot(bot);
  renderMemories(Array.isArray(memories) ? memories : []);
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
  state.datasets = Array.isArray(samples) ? samples : [];
  syncDatasetActions();
  renderMemoryOverview();
  datasetList.innerHTML = "";
  if (!state.datasets.length) {
    datasetList.innerHTML = `
      <article class="dataset-item">
        <strong>暂无 Agent Memory</strong>
        <small>开始对话后，系统会自动把每轮交互沉淀为可导出、可验证、可迁移的记忆资产。</small>
      </article>
    `;
    return;
  }

  state.datasets
    .slice()
    .reverse()
    .forEach((sample) => {
      const createdAt = formatDateTime(sample.createdAt);
      const publishedAt = sample.storedOn0G || sample.uploadPending ? getCurrentProofTimeLabel() : "";
      const statusLabel = sample.uploadPending ? "链上已确认，节点同步中" : sample.storedOn0G ? "已验证上链" : "本地记忆";
      const statusClass = sample.uploadPending ? "pending" : sample.storedOn0G ? "verified" : "";
      const tags = Array.isArray(sample.tags) ? sample.tags.filter(Boolean) : [];
      const pills = [
        `<span class="dataset-pill ${statusClass}">${escapeHtml(statusLabel)}</span>`,
        `<span class="dataset-pill">ID: ${escapeHtml(shortenMiddle(sample.id || "-", 12, 6) || "-")}</span>`,
        `<span class="dataset-pill">创建: ${escapeHtml(createdAt)}</span>`,
      ];
      if (publishedAt) pills.push(`<span class="dataset-pill verified">证明时间: ${escapeHtml(publishedAt)}</span>`);
      tags.slice(0, 4).forEach((tag) => pills.push(`<span class="dataset-pill">${escapeHtml(tag)}</span>`));

      const proofSummary = sample.uploadPending
        ? "已提交到 0G，正在等待节点同步完成。"
        : sample.storedOn0G
          ? "已同步到 0G，可继续导出为 Skills 或在多 Agent 场景中复用。"
          : "尚未发布到 0G，当前仅保存在本地 Memory Layer。";

      const div = document.createElement("article");
      div.className = "dataset-item";
      div.innerHTML = `
        <div class="row">
          <div>
            <strong>${escapeHtml(sample.summary || "未命名记忆样本")}</strong>
            <small>这条记忆可被导出为 Skills，并在多 Agent 场景中复用。</small>
          </div>
        </div>
        <div class="dataset-pills">${pills.join("")}</div>
        <div class="dataset-proof"><small>${escapeHtml(proofSummary)}</small></div>
      `;
      datasetList.append(div);
    });
}

function renderINFTs(infts) {
  state.infts = Array.isArray(infts) ? infts : [];
  syncINFTActions();
  renderMemoryOverview();
  if (!inftList || !inftStatus) return;

  if (!state.infts.length) {
    inftList.innerHTML = `
      <article class="inft-item">
        <strong>暂无 iNFT 资产</strong>
        <small>你可以把训练数据或蒸馏记忆制作成 iNFT，并发布到 0G 网络。</small>
      </article>
    `;
    return;
  }

  inftList.innerHTML = "";
  state.infts
    .slice()
    .reverse()
    .forEach((asset) => {
      const kindLabel = asset.kind === "distilled_memory" ? "Distilled Memory iNFT" : "Training Memory iNFT";
      const sourceLabel = asset.source === "distillation" ? "来源：0G Compute 蒸馏记忆" : "来源：训练数据";
      const statusLabel = asset.registryRegistered ? "已登记到 0G Chain" : asset.storedOn0G ? "待登记到 0G Chain" : "待发布到 0G";
      const storageLabel = asset.storedOn0G ? "Storage 已发布" : "Storage 待发布";
      const registryLabel = asset.registryRegistered ? "Registry 已登记" : asset.storedOn0G ? "Registry 待登记" : "Registry 未开始";
      const isPublishing = state.publishingINFTIDs.has(asset.id);
      const isRegistering = state.registeringINFTIDs.has(asset.id);
      const createdAt = formatDateTime(asset.createdAt);
      const metadataPills = [
        `<span class="dataset-pill ${asset.storedOn0G ? "verified" : ""}">${escapeHtml(storageLabel)}</span>`,
        `<span class="dataset-pill ${asset.registryRegistered ? "verified" : asset.storedOn0G ? "pending" : ""}">${escapeHtml(registryLabel)}</span>`,
        `<span class="dataset-pill">${escapeHtml(sourceLabel)}</span>`,
        `<span class="dataset-pill">样本数：${escapeHtml(String(asset.sampleCount || 0))}</span>`,
      ];

      const proofLines = [
        `<small>iNFT ID：${escapeHtml(shortenMiddle(asset.id || "-", 14, 10))}</small>`,
        `<small>创建时间：${escapeHtml(createdAt)}</small>`,
      ];
      if (asset.parentInftId) {
        proofLines.push(`<small>Parent iNFT：${escapeHtml(shortenMiddle(asset.parentInftId, 14, 10))}</small>`);
      }
      if (asset.rootHash) {
        proofLines.push(`<small>Storage Root：${escapeHtml(shortenMiddle(asset.rootHash, 14, 10))}</small>`);
      }
      if (asset.txHash) {
        proofLines.push(`<small>Storage Tx：${escapeHtml(shortenMiddle(asset.txHash, 14, 10))}</small>`);
      }
      if (asset.storageRef) {
        proofLines.push(`<small>Storage Ref：${escapeHtml(asset.storageRef)}</small>`);
      }
      if (asset.registryRegistered) {
        proofLines.push(`<small>Registry Asset ID：${escapeHtml(asset.registryAssetId || "-")}</small>`);
      }
      if (asset.registryExplorerTxUrl) {
        proofLines.push(
          `<small>Registry Tx：<a href="${escapeHtml(asset.registryExplorerTxUrl)}" target="_blank" rel="noreferrer">${escapeHtml(shortenMiddle(asset.registryTxHash || "-", 14, 10))}</a></small>`,
        );
      }

      const div = document.createElement("article");
      div.className = "inft-item";
      div.innerHTML = `
        <div class="row">
          <div>
            <strong>${escapeHtml(asset.name || "未命名 iNFT")}</strong>
            <small>${escapeHtml(kindLabel)} · 样本数：${escapeHtml(String(asset.sampleCount || 0))}</small>
          </div>
          <div class="actions">
            ${asset.registryRegistered
              ? `<button class="secondary inft-publish-btn" type="button" disabled>已登记</button>`
              : asset.storedOn0G
                ? `<button class="primary inft-publish-btn" type="button" data-register-inft="${escapeHtml(asset.id)}" ${isRegistering ? "disabled" : ""}>${isRegistering ? "登记中..." : "登记到 Chain"}</button>`
                : `<button class="primary inft-publish-btn" type="button" data-publish-inft="${escapeHtml(asset.id)}" ${isPublishing ? "disabled" : ""}>${isPublishing ? "发布中..." : "发布到 0G"}</button>`}
          </div>
        </div>
        <small>${escapeHtml(asset.description || "")}</small>
        <div class="dataset-pills">${metadataPills.join("")}</div>
        <div class="inft-proof">
          <small>当前状态：${escapeHtml(statusLabel)}</small>
          ${proofLines.join("")}
        </div>
      `;
      inftList.append(div);
    });
}

function clearPublishingINFTState(inftID) {
  const id = String(inftID || "").trim();
  if (!id) return;
  state.publishingINFTIDs.delete(id);
  renderINFTs(state.infts);
}

function clearRegisteringINFTState(inftID) {
  const id = String(inftID || "").trim();
  if (!id) return;
  state.registeringINFTIDs.delete(id);
  renderINFTs(state.infts);
}

function renderSkills(skills) {
  if (!skillsList || !skillsSummary) return;
  const raw = Array.isArray(skills) ? skills : [];
  skillsSummary.textContent = raw.length ? `已上传 ${raw.length} 个 Skills。勾选需要启用的 Skills 后可在对话中使用。` : "暂无 Skills。请先从 GitHub 拉取或上传 Skills。";
  if (skillsPublishBundleBtn) {
    const pending = raw.filter((s) => !s.storedOn0G).length;
    skillsPublishBundleBtn.disabled = pending === 0;
    skillsPublishBundleBtn.textContent = pending ? `发布未上链 Skills 到 0G（${pending}）` : "发布未上链 Skills 到 0G";
  }

  const skillIDSet = new Set(raw.map((sk) => String(sk.id || "").trim()).filter(Boolean));
  pruneSelectedSkillIDs(skillIDSet);
  syncSkillSelectionActions();
  renderMemoryOverview();

  skillsList.innerHTML = "";
  raw
    .slice()
    .sort((a, b) => String(a.filename || "").localeCompare(String(b.filename || "")))
    .forEach((sk) => {
    const div = document.createElement("article");
    div.className = "skill-item";

    const checked = state.selectedSkillIDs.has(sk.id);
    const filename = normalizeSkillFilename(sk.filename);
    const skillName = String(sk.name || "").trim() || skillDisplayNameFromFilename(filename || sk.id);
    const statusText = sk.storedOn0G ? "已上链" : "未上链";

    div.innerHTML = `
      <div class="row">
        <div class="skill-title">
          <input type="checkbox" data-skill-check="${escapeHtml(sk.id)}" ${checked ? "checked" : ""} />
          <strong>${escapeHtml(skillName)}</strong>
        </div>
      </div>
      <small class="hint">文件：${escapeHtml(filename || sk.name || sk.id)}</small>
      <small class="hint">状态：${escapeHtml(statusText)}（未上链也可启用；发布仅用于 0G 存储/分享）</small>
    `;
    skillsList.append(div);
  });
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
  let data;
  try {
    data = await fetchJsonTimed(
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
    );
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${String(e?.message || e)}`;
    return;
  }
  if (skillsSummary) skillsSummary.textContent = `已从 GitHub 导入 ${Number(data?.count || 0)} 个 Skills。`;
  try {
    await refreshSkills();
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${String(e?.message || e)}`;
  }
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
          try {
            await refreshSkills();
          } catch (e) {
            if (skillsSummary) skillsSummary.textContent = `[错误] ${String(e?.message || e)}`;
          }
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
  try {
    await refreshSkills();
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${String(e?.message || e)}`;
  }
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

  const llm = buildBotLLMConfig();

  let data;
  try {
    const enabledSkillIDs = getEnabledSkillIDsFromSelection();

    // Frontend-executed transfer tools run first; if a transfer is attempted in
    // this turn, skip x402 tools to avoid dual wallet flows in one message.
    const transferResults = await runTransferSkillsInBrowser(enabledSkillIDs, message);

    // Frontend-executed x402 tools: run before sending chat so the backend can
    // include the results in the LLM prompt.
    const x402Results = transferResults.length ? [] : await runX402SkillsInBrowser(enabledSkillIDs, message);

    data = await fetchJson(`${apiBase}/api/bots/${state.activeBotId}/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        message,
        llm,
        skills: enabledSkillIDs,
        x402: x402Results,
        transfers: transferResults,
        debug: debugSkills?.checked ? { skillsUsed: true } : undefined,
      }),
    });
  } catch (error) {
    resolveThinking(thinking, `[网络错误] ${String(error)}`);
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

  try {
    await refreshDatasets();
  } catch (e) {
    if (llmStatus) llmStatus.textContent = `[错误] ${String(e?.message || e)}`;
  }
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
  const llm = buildBotLLMConfig();

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
        try {
          await refreshDatasets();
          publishResult.textContent = "";
        } catch (e) {
          publishResult.textContent = `[错误] ${String(e?.message || e)}`;
        }
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

  try {
    await refreshDatasets();
    publishResult.textContent = "";
  } catch (e) {
    publishResult.textContent = `[错误] ${String(e?.message || e)}`;
  }
}

async function exportDatasetsAsSkills() {
  if (!state.activeBotId) {
    alert("请先选择机器人");
    return;
  }
  if (!Array.isArray(state.datasets) || state.datasets.length === 0) {
    alert("暂无训练样本可导出");
    return;
  }

  if (publishResult) {
    publishResult.textContent = "正在导出训练数据为 Skills...";
  }

  const distilledSummary = String(state.distillation?.memorySummary || "").trim();
  const includeDistilledMemory =
    distilledSummary !== ""
      ? await showConfirmDialog({
          title: "一起导出蒸馏记忆？",
          message: "点击“确定”会把蒸馏记忆和训练数据合并导出为一个 Skill。点击“取消”则只导出训练数据为 Skills。",
          confirmLabel: "一起导出",
          cancelLabel: "只导出训练数据",
        })
      : false;

  let resp;
  try {
    resp = await fetch(`${apiBase}/api/bots/${state.activeBotId}/datasets/export_skills`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        memorySummary: includeDistilledMemory ? distilledSummary : "",
      }),
    });
  } catch (e) {
    if (publishResult) publishResult.textContent = `[错误] ${String(e?.message || e)}`;
    return;
  }

  if (!resp.ok) {
    let errMsg = resp.statusText;
    try {
      const data = await resp.json();
      errMsg = data?.error || errMsg;
    } catch {
      // ignore
    }
    if (publishResult) publishResult.textContent = `[错误] ${errMsg}`;
    return;
  }

  let blob;
  try {
    blob = await resp.blob();
  } catch (e) {
    if (publishResult) publishResult.textContent = `[错误] ${String(e?.message || e)}`;
    return;
  }

  const fallbackName = `training-skills-${state.activeBotId || "export"}.zip`;
  const filename = readAttachmentFilename(resp.headers.get("Content-Disposition"), fallbackName);
  downloadBlobFile(blob, filename);
  if (publishResult) {
    publishResult.textContent = includeDistilledMemory
      ? `已将蒸馏记忆和 ${state.datasets.length} 条训练样本合并导出为单个 Skill：${filename}`
      : `已导出 ${state.datasets.length} 条训练样本为 Skills 压缩包：${filename}`;
  }
}

async function distillMemoriesWith0GCompute() {
  if (!state.activeBotId) {
    alert("请先选择机器人");
    return;
  }
  if (!Array.isArray(state.datasets) || state.datasets.length === 0) {
    alert("暂无训练样本可蒸馏");
    return;
  }

  state.distillation = null;
  renderDistillationResult();
  if (memoryDistillStatus) {
    memoryDistillStatus.textContent = "正在通过 0G Compute 蒸馏记忆...";
  }

  const enabledSkillIDs = getEnabledSkillIDsFromSelection();
  let data;
  try {
    data = await fetchJsonTimed(
      `${apiBase}/api/bots/${state.activeBotId}/datasets/distill`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ maxSamples: 12, skills: enabledSkillIDs }),
      },
      120000,
    );
  } catch (e) {
    if (memoryDistillStatus) memoryDistillStatus.textContent = `[错误] ${String(e?.message || e)}`;
    return;
  }

  state.distillation = {
    ...data,
    saved: false,
  };
  renderDistillationResult();
  if (memoryDistillStatus) {
    memoryDistillStatus.textContent = `蒸馏完成：已基于 ${Number(data?.sampleCount || 0)} 条记忆生成记忆摘要。`;
  }
}

async function createTrainingINFT() {
  if (!state.activeBotId) {
    alert("请先选择机器人");
    return;
  }
  if (!Array.isArray(state.datasets) || state.datasets.length === 0) {
    alert("暂无训练样本可制作 iNFT");
    return;
  }
  if (inftStatus) inftStatus.textContent = "正在生成训练数据 iNFT...";
  try {
    await fetchJsonTimed(
      `${apiBase}/api/bots/${state.activeBotId}/infts/create_training`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      },
      30000,
    );
    await refreshINFTs();
    if (inftStatus) inftStatus.textContent = "训练数据 iNFT 已生成，可继续发布到 0G。";
  } catch (e) {
    if (inftStatus) inftStatus.textContent = `[错误] ${String(e?.message || e)}`;
  }
}

async function createDistilledINFT() {
  if (!state.activeBotId) {
    alert("请先选择机器人");
    return;
  }
  const memorySummary = String(state.distillation?.memorySummary || "").trim();
  if (!memorySummary) {
    alert("请先生成蒸馏记忆摘要");
    return;
  }
  if (inftStatus) inftStatus.textContent = "正在生成蒸馏记忆 iNFT...";
  try {
    await fetchJsonTimed(
      `${apiBase}/api/bots/${state.activeBotId}/infts/create_distilled`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ memorySummary }),
      },
      30000,
    );
    await refreshINFTs();
    if (inftStatus) inftStatus.textContent = "蒸馏记忆 iNFT 已生成，可继续发布到 0G。";
  } catch (e) {
    if (inftStatus) inftStatus.textContent = `[错误] ${String(e?.message || e)}`;
  }
}

async function publishINFT(inftID) {
  const id = String(inftID || "").trim();
  if (!id) return;
  if (state.publishingINFTIDs.has(id)) return;
  const ok = await ensureWalletAuthorized();
  if (!ok) return;

  const authToken = localStorage.getItem("authToken") || "";
  const zgsNodes = (zgsNodesInput?.value || "").trim();
  if (zgsNodesInput) localStorage.setItem("zgsNodes", zgsNodes);
  const from = localStorage.getItem("walletAddress") || "";
  if (!from) {
    if (inftStatus) inftStatus.textContent = "[错误] 未连接钱包";
    return;
  }

  state.publishingINFTIDs.add(id);
  renderINFTs(state.infts);
  if (inftStatus) inftStatus.textContent = "准备链上交易（iNFT）...";
  let prepResp, prep;
  try {
    ({ resp: prepResp, data: prep } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/infts/${id}/publish_prepare`,
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
    clearPublishingINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] 准备链上交易超时或失败：${String(e?.message || e)}`;
    return;
  }
  if (!prepResp.ok) {
    clearPublishingINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] ${prep?.error || prepResp.statusText}`;
    return;
  }
  if (!prep?.publishId) {
    clearPublishingINFTState(id);
    if (inftStatus) inftStatus.textContent = "[错误] 后端未返回 publishId，请重启后端并刷新页面后重试";
    return;
  }

  if (inftStatus) inftStatus.textContent = "请在钱包中确认交易（iNFT）...";
  let txHash;
  try {
    txHash = await window.ethereum.request({
      method: "eth_sendTransaction",
      params: [{ from, to: prep.to, data: prep.data, value: prep.value }],
    });
  } catch (e) {
    clearPublishingINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] 交易发送失败：${String(e?.message || e)}`;
    return;
  }

  if (inftStatus) inftStatus.textContent = "交易已发送，等待发布 iNFT...";
  let finResp, result;
  try {
    ({ resp: finResp, data: result } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/infts/${id}/publish_finalize`,
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
      const ok = await tryResolveTxSuccessFromChain(txHash);
      if (ok) {
        try {
          clearPublishingINFTState(id);
          await refreshINFTs();
          if (inftStatus) inftStatus.textContent = "iNFT 已成功发布到 0G。";
        } catch (refreshErr) {
          if (inftStatus) inftStatus.textContent = `[错误] ${String(refreshErr?.message || refreshErr)}`;
        }
        return;
      }
    }
    clearPublishingINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] 发布 iNFT 超时或失败：${msg}`;
    return;
  }
  if (!finResp.ok) {
    clearPublishingINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] ${result?.error || finResp.statusText}`;
    return;
  }

  try {
    clearPublishingINFTState(id);
    await refreshINFTs();
    if (inftStatus) inftStatus.textContent = "iNFT 已成功发布到 0G。";
  } catch (e) {
    clearPublishingINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] ${String(e?.message || e)}`;
  }
}

async function registerINFT(inftID) {
  const id = String(inftID || "").trim();
  if (!id) return;
  if (state.registeringINFTIDs.has(id)) return;
  const ok = await ensureWalletAuthorized();
  if (!ok) return;

  const authToken = localStorage.getItem("authToken") || "";
  const from = localStorage.getItem("walletAddress") || "";
  if (!from) {
    if (inftStatus) inftStatus.textContent = "[错误] 未连接钱包";
    return;
  }

  state.registeringINFTIDs.add(id);
  renderINFTs(state.infts);
  if (inftStatus) inftStatus.textContent = "准备链上登记交易（Memory Registry）...";

  let prepResp, prep;
  try {
    ({ resp: prepResp, data: prep } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/infts/${id}/register_prepare`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
        },
      },
      30000,
    ));
  } catch (e) {
    clearRegisteringINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] 准备链上登记超时或失败：${String(e?.message || e)}`;
    return;
  }
  if (!prepResp.ok) {
    clearRegisteringINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] ${prep?.error || prepResp.statusText}`;
    return;
  }

  if (inftStatus) inftStatus.textContent = "请在钱包中确认登记交易（Memory Registry）...";
  let txHash;
  try {
    txHash = await window.ethereum.request({
      method: "eth_sendTransaction",
      params: [{ from, to: prep.to, data: prep.data, value: prep.value }],
    });
  } catch (e) {
    clearRegisteringINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] 登记交易发送失败：${String(e?.message || e)}`;
    return;
  }

  if (inftStatus) inftStatus.textContent = "登记交易已发送，等待链上确认...";
  let finResp, result;
  try {
    ({ resp: finResp, data: result } = await fetchJsonWithTimeout(
      `${apiBase}/api/bots/${state.activeBotId}/infts/${id}/register_finalize`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
        },
        body: JSON.stringify({ publishId: String(prep.publishId), txHash }),
      },
      45000,
    ));
  } catch (e) {
    const msg = String(e?.message || e);
    if (msg.includes("请求超时") && txHash) {
      const ok = await tryResolveTxSuccessFromChain(txHash);
      if (ok) {
        try {
          clearRegisteringINFTState(id);
          await refreshINFTs();
          if (inftStatus) inftStatus.textContent = "iNFT 已登记到 0G Chain。";
        } catch (refreshErr) {
          if (inftStatus) inftStatus.textContent = `[错误] ${String(refreshErr?.message || refreshErr)}`;
        }
        return;
      }
    }
    clearRegisteringINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] iNFT 登记超时或失败：${msg}`;
    return;
  }
  if (!finResp.ok) {
    clearRegisteringINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] ${result?.error || finResp.statusText}`;
    return;
  }

  try {
    clearRegisteringINFTState(id);
    await refreshINFTs();
    if (inftStatus) inftStatus.textContent = "iNFT 已登记到 0G Chain。";
  } catch (e) {
    clearRegisteringINFTState(id);
    if (inftStatus) inftStatus.textContent = `[错误] ${String(e?.message || e)}`;
  }
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
      publishResult.textContent = "链上已成功，上传已完成或在后台继续同步。";
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
        publishResult.textContent = "链上已成功，上传已完成或在后台继续同步。";
        return true;
      }
      if (r2.ok && d2?.status === "failed") {
        publishResult.textContent = "[错误] 交易链上失败";
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
  const files = Array.from(skillsLocalFiles?.files || []);
  if (!files.length) {
    alert("请选择本地 skills 文件");
    return;
  }

  const uploadNames = files.map((f) => normalizeSkillFilename(f.webkitRelativePath || f.name)).filter(Boolean);
  const duplicatesInUpload = findDuplicateSkillFilenames(uploadNames);
  if (duplicatesInUpload.length) {
    const msg = `检测到重复的 Skills 文件：${summarizeSkillNames(duplicatesInUpload)}`;
    if (skillsSummary) skillsSummary.textContent = `[错误] ${msg}`;
    alert(msg);
    return;
  }

  const existingNames = new Set((state.skills || []).map((sk) => normalizeSkillFilename(sk.filename)).filter(Boolean));
  const conflicts = uploadNames.filter((name, index) => existingNames.has(name) && uploadNames.indexOf(name) === index);
  if (conflicts.length) {
    const msg = `以下 Skills 文件已存在，请勿重复上传：${summarizeSkillNames(conflicts)}`;
    if (skillsSummary) skillsSummary.textContent = `[错误] ${msg}`;
    alert(msg);
    return;
  }

  const fd = new FormData();
  for (const file of files) {
    const rel = normalizeSkillFilename(file.webkitRelativePath || file.name);
    fd.append("files", file, rel);
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

  if (skillsLocalFiles) skillsLocalFiles.value = "";

  try {
    await refreshSkills();
    if (skillsSummary) skillsSummary.textContent = `已上传 ${Number(data?.count || files.length)} 个本地 Skills。勾选需要启用的 Skills 后可在对话中使用。`;
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${String(e?.message || e)}`;
  }
}

async function uploadLocalSkills() {
  await uploadSkillsFolder();
}

function clearSelectedSkillSelections() {
  state.selectedSkillIDs.clear();
  syncSkillSelectionActions();
  renderSkills(state.skills);
}

async function deleteSelectedSkills() {
  if (!state.activeBotId) {
    alert("请先选择机器人");
    return;
  }
  const selectedSkillIDs = Array.from(state.selectedSkillIDs || []).filter(Boolean);
  if (!selectedSkillIDs.length) {
    alert("请先勾选要删除的 Skills");
    return;
  }
  const skillIDs = selectedSkillIDs;

  if (!skillIDs.length) {
    clearSelectedSkillSelections();
    return;
  }

  const confirmed = await showConfirmDialog({
    title: "删除已选 Skills？",
    message: `将删除已选的 ${skillIDs.length} 个 Skills。此操作不可撤销。`,
    confirmLabel: "确认删除",
    cancelLabel: "取消",
    danger: true,
  });
  if (!confirmed) {
    return;
  }

  if (skillsSummary) skillsSummary.textContent = "正在删除已选 Skills...";
  let data;
  try {
    data = await fetchJsonTimed(
      `${apiBase}/api/bots/${state.activeBotId}/skills/delete`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ skillIds: skillIDs }),
      },
      20000,
    );
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${String(e?.message || e)}`;
    return;
  }

  state.selectedSkillIDs.clear();
  try {
    await refreshSkills();
    if (skillsSummary) skillsSummary.textContent = `已删除 ${Number(data?.deleted || 0)} 个 Skills。`;
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${String(e?.message || e)}`;
  }
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

  try {
    await refreshSkills();
  } catch (e) {
    if (skillsSummary) skillsSummary.textContent = `[错误] ${String(e?.message || e)}`;
  }
}

function shouldAutoScroll(container, threshold = 48) {
  if (!container) return true;
  return container.scrollTop + container.clientHeight >= container.scrollHeight - threshold;
}

function appendBubble(role, content) {
  const stickToBottom = shouldAutoScroll(chatStream);
  const div = document.createElement("div");
  div.className = `bubble ${role}`;
  div.textContent = content;
  chatStream.append(div);
  if (stickToBottom) {
    chatStream.scrollTop = chatStream.scrollHeight;
  }
  return div;
}

function appendThinkingBubble() {
  const stickToBottom = shouldAutoScroll(chatStream);
  const div = document.createElement("div");
  div.className = "bubble assistant thinking";
  div.textContent = "正在思考...";
  chatStream.append(div);
  if (stickToBottom) {
    chatStream.scrollTop = chatStream.scrollHeight;
  }
  return div;
}

function resolveThinking(div, text) {
  if (!div) return;
  const stickToBottom = shouldAutoScroll(chatStream);
  div.classList.remove("thinking");
  div.textContent = text;
  if (stickToBottom) {
    chatStream.scrollTop = chatStream.scrollHeight;
  }
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

function looksLikeTransferSkillContent(content) {
  const raw = String(content || "").trim();
  if (!raw || !raw.startsWith("{")) return false;
  try {
    const v = JSON.parse(raw);
    const t = String(v?.type || "").trim().toLowerCase();
    return t === "evm_transfer" || t === "metamask_transfer" || t === "inft_transfer" || t === "metamask_inft_transfer";
  } catch {
    return false;
  }
}

function parseTransferSkillSpec(content) {
  const raw = String(content || "").trim();
  const v = JSON.parse(raw);
  const t = String(v?.type || "").trim().toLowerCase();
  if (t !== "evm_transfer" && t !== "metamask_transfer" && t !== "inft_transfer" && t !== "metamask_inft_transfer") return null;

  if (t === "inft_transfer" || t === "metamask_inft_transfer") {
    const triggerKeywords = Array.isArray(v?.triggerKeywords)
      ? v.triggerKeywords
          .map((k) => String(k || "").trim().toLowerCase())
          .filter(Boolean)
      : ["inft", "nft", "出售", "转移", "sell", "transfer", "trade"];
    const queryKeywords = Array.isArray(v?.queryKeywords)
      ? v.queryKeywords
          .map((k) => String(k || "").trim().toLowerCase())
          .filter(Boolean)
      : ["查询", "查看", "我的inft", "我的nft", "已有的inft", "已有的nft", "创建的inft", "创建的nft", "inft列表", "nft列表", "list", "show", "query"];

    const chainId = Number(v?.chainId);
    return {
      mode: "inft",
      triggerKeywords,
      queryKeywords,
      chainId: Number.isFinite(chainId) && chainId > 0 ? chainId : 16602,
      token: "",
      tokenAddress: "",
      decimals: null,
    };
  }

  const triggerKeywords = Array.isArray(v?.triggerKeywords)
    ? v.triggerKeywords
        .map((k) => String(k || "").trim().toLowerCase())
        .filter(Boolean)
    : ["转账", "支付", "付款", "pay", "transfer"];

  const chainId = Number(v?.chainId);
  const token = String(v?.token || "native").trim();
  const tokenAddress = String(v?.tokenAddress || "").trim();
  const decimalsRaw = v?.decimals;
  const decimals = Number.isInteger(Number(decimalsRaw)) ? Number(decimalsRaw) : null;

  return {
    mode: "fungible",
    triggerKeywords,
    chainId: Number.isFinite(chainId) && chainId > 0 ? chainId : null,
    token,
    tokenAddress,
    decimals,
  };
}

function hasTransferKeyword(message, keywords) {
  const text = String(message || "").toLowerCase();
  if (!text) return false;
  const list = Array.isArray(keywords) && keywords.length ? keywords : ["转账", "支付", "付款", "pay", "transfer"];
  return list.some((k) => k && text.includes(String(k).toLowerCase()));
}

function extractTransferIntent(message) {
  const text = String(message || "");
  const lower = text.toLowerCase();

  const toMatch = text.match(/0x[a-fA-F0-9]{40}/);
  const to = toMatch ? toMatch[0] : "";

  // 支持: chain 8453 / 链8453 / chainId=8453
  let chainId = null;
  const chainMatch = lower.match(/(?:chain\s*id|chainid|chain|链)\s*[:=：]?\s*(\d{1,10})/i);
  if (chainMatch) {
    const n = Number(chainMatch[1]);
    if (Number.isFinite(n) && n > 0) chainId = n;
  }

  // 支持: 0.01 ETH / 10 usdc / amount 1.23 / 金额 1.23
  let amount = "";
  let token = "";
  const amountTokenMatch = text.match(/(\d+(?:\.\d+)?)\s*(0g|og|eth|usdc|usdt|dai|matic|bnb)\b/i);
  if (amountTokenMatch) {
    amount = amountTokenMatch[1];
    token = amountTokenMatch[2].toLowerCase();
  } else {
    const amountMatch = text.match(/(?:amount|金额|转账|支付|付款)\s*[:=：]?\s*(\d+(?:\.\d+)?)/i);
    if (amountMatch) amount = amountMatch[1];
  }

  const tokenAddressMatch = text.match(/token\s*[:=：]?\s*(0x[a-fA-F0-9]{40})/i);
  const tokenAddress = tokenAddressMatch ? tokenAddressMatch[1] : "";

  return { to, amount, token, chainId, tokenAddress };
}

function normalizeAddress(addr) {
  const s = String(addr || "").trim();
  if (!/^0x[a-fA-F0-9]{40}$/.test(s)) return "";
  if (/^0x0{40}$/i.test(s)) return "";
  return s;
}

function isPositiveDecimal(value) {
  const s = String(value || "").trim();
  if (!/^\d+(\.\d+)?$/.test(s)) return false;
  return Number(s) > 0;
}

function decimalToBaseUnits(value, decimals) {
  const s = String(value || "").trim();
  if (!/^\d+(\.\d+)?$/.test(s)) throw new Error("金额格式不正确");
  const d = Number(decimals);
  if (!Number.isInteger(d) || d < 0 || d > 36) throw new Error("decimals 不合法");

  const [whole, fracRaw = ""] = s.split(".");
  if (fracRaw.length > d) throw new Error(`金额精度超过 ${d} 位`);
  const frac = (fracRaw + "0".repeat(d)).slice(0, d);
  const merged = `${whole}${frac}`.replace(/^0+(\d)/, "$1");
  return merged || "0";
}

function erc20TransferData(to, amountBaseUnits) {
  const methodId = "a9059cbb"; // transfer(address,uint256)
  const addr = normalizeAddress(to);
  if (!addr) throw new Error("收款地址无效");
  const addrNo0x = addr.slice(2).toLowerCase().padStart(64, "0");
  const amountHex = BigInt(String(amountBaseUnits)).toString(16).padStart(64, "0");
  return `0x${methodId}${addrNo0x}${amountHex}`;
}

function isAllowedTransferChain(chainId) {
  const n = Number(chainId);
  if (!Number.isFinite(n) || n <= 0) return false;
  return !!knownChainPreset(n);
}

function normalizeTransferTokenName(token) {
  const low = String(token || "").trim().toLowerCase();
  if (low === "0g" || low === "og") return "0g";
  if (low === "eth") return "native";
  return low;
}

function normalizeBytes32(value) {
  const s = String(value || "").trim();
  if (!/^0x[a-fA-F0-9]{64}$/.test(s)) return "";
  if (/^0x0{64}$/i.test(s)) return "";
  return s;
}

function buildINFTTransferData(assetId, to) {
  const methodId = "6ad527f3"; // transferAsset(bytes32,address)
  const normalizedAssetID = normalizeBytes32(assetId);
  const normalizedTo = normalizeAddress(to);
  if (!normalizedAssetID) throw new Error("iNFT Asset ID 无效");
  if (!normalizedTo) throw new Error("收款地址无效");
  const toNo0x = normalizedTo.slice(2).toLowerCase().padStart(64, "0");
  return `0x${methodId}${normalizedAssetID.slice(2).toLowerCase()}${toNo0x}`;
}

function hasINFTTransferKeyword(message, keywords) {
  const text = String(message || "").toLowerCase();
  if (!text) return false;
  const assetWords = ["inft", "nft", "记忆资产"];
  const verbWords = ["转移", "出售", "交易", "sell", "transfer", "trade"];
  const list = Array.isArray(keywords) && keywords.length ? keywords : [];
  const hasConfigured = list.some((k) => k && text.includes(String(k).toLowerCase()));
  const hasVerb = verbWords.some((k) => text.includes(k));
  const hasAssetWord = assetWords.some((k) => text.includes(k));
  return hasConfigured || (hasVerb && hasAssetWord);
}

function hasINFTQueryKeyword(message, keywords) {
  const text = String(message || "").toLowerCase();
  if (!text) return false;
  const assetWords = ["inft", "nft", "记忆资产", "资产"];
  const queryWords = ["查询", "查看", "有哪些", "已有", "创建的", "我的", "list", "show", "query", "owned", "created"];
  const list = Array.isArray(keywords) && keywords.length ? keywords : [];
  const hasConfigured = list.some((k) => k && text.includes(String(k).toLowerCase()));
  const hasQuery = queryWords.some((k) => text.includes(k));
  const hasAssetWord = assetWords.some((k) => text.includes(k));
  return hasConfigured || (hasQuery && hasAssetWord);
}

function formatINFTQueryText(assets) {
  const list = Array.isArray(assets) ? assets : [];
  if (!list.length) {
    return "当前账户下还没有 iNFT 资产。";
  }
  const lines = [`当前 Bot 下共有 ${list.length} 个 iNFT 资产：`];
  list
    .slice()
    .reverse()
    .forEach((asset, index) => {
      const kindLabel = asset.kind === "distilled_memory" ? "Distilled Memory iNFT" : "Training Memory iNFT";
      const statusLabel = asset.registryRegistered ? "已登记到 0G Chain" : asset.storedOn0G ? "已发布到 0G，待登记到 Chain" : "待发布到 0G";
      lines.push(`${index + 1}. ${asset.name || "未命名 iNFT"}`);
      lines.push(`   类型：${kindLabel}`);
      lines.push(`   状态：${statusLabel}`);
      lines.push(`   样本数：${Number(asset.sampleCount || 0)}`);
      if (asset.registryAssetId) lines.push(`   Registry Asset ID：${asset.registryAssetId}`);
      if (asset.storageRef) lines.push(`   Storage Ref：${asset.storageRef}`);
      if (asset.registryTxHash) lines.push(`   Registry Tx：${asset.registryTxHash}`);
      if (asset.registryExplorerTxUrl) lines.push(`   Registry Explorer：${asset.registryExplorerTxUrl}`);
      if (asset.id) lines.push(`   本地 iNFT ID：${asset.id}`);
      if (asset.parentInftId) lines.push(`   Parent iNFT：${asset.parentInftId}`);
    });
  return lines.join("\n");
}

async function syncOwnedINFTsFromRegistry() {
  if (!state.activeBotId) return [];
  let owner = localStorage.getItem("walletAddress") || "";
  if (!owner && window.ethereum?.request) {
    try {
      const accounts = await window.ethereum.request({ method: "eth_accounts" });
      owner = accounts?.[0] || "";
      if (owner) localStorage.setItem("walletAddress", owner);
    } catch {
      owner = "";
    }
  }
  if (!owner) return Array.isArray(state.infts) ? state.infts : [];

  const data = await fetchJsonTimed(
    `${apiBase}/api/bots/${state.activeBotId}/infts/owned?owner=${encodeURIComponent(owner)}`,
    {
      method: "GET",
    },
    20000,
  );
  state.infts = Array.isArray(data) ? data : [];
  renderINFTs(state.infts);
  return state.infts;
}

function findRegisteredINFTAssetForTransfer(message) {
  const text = String(message || "");
  const lower = text.toLowerCase();
  const assets = (state.infts || []).filter((asset) => asset?.registryRegistered && asset?.registryAssetId);
  if (!assets.length) return null;

  const explicitAssetIDMatch = text.match(/0x[a-fA-F0-9]{64}/);
  if (explicitAssetIDMatch) {
    const assetID = explicitAssetIDMatch[0];
    const matched = assets.find((asset) => String(asset.registryAssetId || "").toLowerCase() === assetID.toLowerCase());
    if (matched) return matched;
  }

  const byName = assets
    .filter((asset) => {
      const name = String(asset.name || "").trim().toLowerCase();
      return !!name && lower.includes(name);
    })
    .sort((a, b) => String(b.name || "").length - String(a.name || "").length);
  if (byName.length) return byName[0];

  if (assets.length === 1) return assets[0];
  return null;
}

async function ensureTransferChain(chainIdDec) {
  const n = Number(chainIdDec);
  if (!Number.isFinite(n) || n <= 0) throw new Error("链ID不合法");
  if (!isAllowedTransferChain(n)) throw new Error(`当前仅支持白名单链：${n}`);

  await ensureEvmChainForSigning(n);
  const cur = await window.ethereum.request({ method: "eth_chainId" });
  const wantHex = "0x" + n.toString(16);
  if (String(cur).toLowerCase() !== wantHex.toLowerCase()) {
    throw new Error(`当前钱包链为 ${cur}，目标链为 ${wantHex}，请先切换后重试`);
  }
}

async function runTransferSkillsInBrowser(enabledSkillIDs, userMessage) {
  const ids = Array.isArray(enabledSkillIDs) ? enabledSkillIDs : [];
  if (!ids.length) return [];

  const candidates = [];
  for (const id of ids) {
    const sk = (state.skills || []).find((x) => x?.id === id);
    if (!sk) continue;
    if (!looksLikeTransferSkillContent(sk.content)) continue;
    let spec = null;
    try {
      spec = parseTransferSkillSpec(sk.content);
    } catch {
      spec = null;
    }
    if (!spec) continue;
    candidates.push({ sk, spec });
  }
  if (!candidates.length) return [];

  const inftQueryMatched = candidates.find(({ spec }) => spec?.mode === "inft" && hasINFTQueryKeyword(userMessage, spec.queryKeywords));
  const inftMatched = candidates.find(({ spec }) => spec?.mode === "inft" && hasINFTTransferKeyword(userMessage, spec.triggerKeywords));
  // 单条消息最多执行一个 transfer skill，避免一次输入触发多笔转账。
  const matched = inftQueryMatched || inftMatched || candidates.find(({ spec }) => spec?.mode !== "inft" && hasTransferKeyword(userMessage, spec.triggerKeywords));
  if (!matched) return [];

  const { sk, spec } = matched;
  const startedAt = isoNow();
  const intent = extractTransferIntent(userMessage);

  if (spec?.mode === "inft") {
    if (inftQueryMatched && matched === inftQueryMatched) {
      let ownedAssets = state.infts || [];
      try {
        ownedAssets = await syncOwnedINFTsFromRegistry();
      } catch (e) {
        return [
          {
            skillId: sk.id,
            type: "inft_query",
            chainId: Number(spec.chainId || 16602),
            to: "",
            token: "",
            amount: "",
            amountWei: "",
            assetId: "",
            assetName: "",
            queryText: "",
            ok: false,
            error: `iNFT 查询失败：${String(e?.message || e)}`,
            startedAt,
            endedAt: isoNow(),
          },
        ];
      }
      return [
        {
          skillId: sk.id,
          type: "inft_query",
          chainId: Number(spec.chainId || 16602),
          to: "",
          token: "",
          amount: "",
          amountWei: "",
          assetId: "",
          assetName: "",
          queryText: formatINFTQueryText(ownedAssets || []),
          ok: true,
          startedAt,
          endedAt: isoNow(),
        },
      ];
    }

    const asset = findRegisteredINFTAssetForTransfer(userMessage);
    const to = normalizeAddress(intent.to);
    const assetId = normalizeBytes32(asset?.registryAssetId);
    const assetName = String(asset?.name || "").trim();
    const chainId = Number(spec.chainId || 16602);
    const contractAddr = normalizeAddress(asset?.registryContract || "");

    if (!asset) {
      return [
        {
          skillId: sk.id,
          type: "inft",
          chainId,
          to: to || intent.to || "",
          token: "",
          amount: "",
          amountWei: "",
          assetId: "",
          assetName: "",
          ok: false,
          error: "未找到可转移的已登记 iNFT，请在消息里提供 iNFT 名称或 Registry Asset ID",
          startedAt,
          endedAt: isoNow(),
        },
      ];
    }

    if (!to || !assetId || !contractAddr) {
      const missing = [];
      if (!to) missing.push("接收地址(0x...)");
      if (!assetId) missing.push("Registry Asset ID");
      if (!contractAddr) missing.push("MemoryRegistry 合约地址");
      return [
        {
          skillId: sk.id,
          type: "inft",
          chainId,
          to: to || intent.to || "",
          token: "",
          amount: "",
          amountWei: "",
          assetId: assetId || "",
          assetName,
          ok: false,
          error: `iNFT 转移参数不完整：缺少 ${missing.join("、")}`,
          startedAt,
          endedAt: isoNow(),
        },
      ];
    }

    const ok = await ensureWalletAuthorized();
    if (!ok) {
      return [
        {
          skillId: sk.id,
          type: "inft",
          chainId,
          to,
          token: "",
          amount: "",
          amountWei: "",
          assetId,
          assetName,
          ok: false,
          error: "未完成钱包授权",
          startedAt,
          endedAt: isoNow(),
        },
      ];
    }

    try {
      await ensureTransferChain(chainId);
    } catch (e) {
      return [
        {
          skillId: sk.id,
          type: "inft",
          chainId,
          to,
          token: "",
          amount: "",
          amountWei: "",
          assetId,
          assetName,
          ok: false,
          error: `切换链失败：${String(e?.message || e)}`,
          startedAt,
          endedAt: isoNow(),
        },
      ];
    }

    const from = localStorage.getItem("walletAddress") || "";
    if (!from) {
      return [
        {
          skillId: sk.id,
          type: "inft",
          chainId,
          to,
          token: "",
          amount: "",
          amountWei: "",
          assetId,
          assetName,
          ok: false,
          error: "未连接钱包",
          startedAt,
          endedAt: isoNow(),
        },
      ];
    }

    try {
      const txHash = await window.ethereum.request({
        method: "eth_sendTransaction",
        params: [
          {
            from,
            to: contractAddr,
            data: buildINFTTransferData(assetId, to),
            value: "0x0",
          },
        ],
      });

      state.infts = (state.infts || []).filter((item) => String(item?.registryAssetId || "").toLowerCase() !== assetId.toLowerCase());
      renderINFTs(state.infts);

      return [
        {
          skillId: sk.id,
          type: "inft",
          chainId,
          to,
          token: "",
          amount: "",
          amountWei: "",
          assetId,
          assetName,
          txHash: String(txHash || ""),
          ok: true,
          startedAt,
          endedAt: isoNow(),
        },
      ];
    } catch (e) {
      const code = Number(e?.code ?? e?.data?.originalError?.code);
      let msg = String(e?.message || e);
      if (code === 4001) msg = "用户取消签名/交易";
      return [
        {
          skillId: sk.id,
          type: "inft",
          chainId: Number(spec.chainId || 16602),
          to,
          token: "",
          amount: "",
          amountWei: "",
          assetId,
          assetName,
          ok: false,
          error: msg,
          startedAt,
          endedAt: isoNow(),
        },
      ];
    }
  }

  const to = normalizeAddress(intent.to);
  const normalizedTokenName = normalizeTransferTokenName(intent.token || spec.token || "native");
  const chainId = intent.chainId || spec.chainId || (normalizedTokenName === "0g" ? 16602 : null);
  const tokenAddr = normalizeAddress(intent.tokenAddress || spec.tokenAddress || "");
  const tokenName = normalizedTokenName;
  const isNative = !tokenAddr && (tokenName === "" || tokenName === "native" || tokenName === "eth" || tokenName === "0g");
  const decimals = Number.isInteger(spec.decimals) ? spec.decimals : isNative ? 18 : 18;

  if (!hasTransferKeyword(userMessage, spec.triggerKeywords)) {
    return [];
  }

  if (!to || !isPositiveDecimal(intent.amount) || !chainId) {
    const missing = [];
    if (!to) missing.push("收款地址(0x...) ");
    if (!isPositiveDecimal(intent.amount)) missing.push("金额(>0)");
    if (!chainId) missing.push("链ID(chain 16602)");
    return [
      {
        skillId: sk.id,
        type: isNative ? "native" : "erc20",
        chainId: Number(chainId) || 0,
        to: to || intent.to || "",
        token: isNative ? "native" : tokenAddr,
        amount: String(intent.amount || ""),
        amountWei: "",
        ok: false,
        error: `转账参数不完整：缺少 ${missing.join("、")}`,
        startedAt,
        endedAt: isoNow(),
      },
    ];
  }

  const ok = await ensureWalletAuthorized();
  if (!ok) {
    return [
      {
        skillId: sk.id,
        type: isNative ? "native" : "erc20",
        chainId: Number(chainId),
        to,
        token: isNative ? "native" : tokenAddr,
        amount: String(intent.amount),
        amountWei: "",
        ok: false,
        error: "未完成钱包授权",
        startedAt,
        endedAt: isoNow(),
      },
    ];
  }

  try {
    await ensureTransferChain(Number(chainId));
  } catch (e) {
    return [
      {
        skillId: sk.id,
        type: isNative ? "native" : "erc20",
        chainId: Number(chainId),
        to,
        token: isNative ? "native" : tokenAddr,
        amount: String(intent.amount),
        amountWei: "",
        ok: false,
        error: `切换链失败：${String(e?.message || e)}`,
        startedAt,
        endedAt: isoNow(),
      },
    ];
  }

  let amountWei = "";
  try {
    amountWei = decimalToBaseUnits(intent.amount, decimals);
  } catch (e) {
    return [
      {
        skillId: sk.id,
        type: isNative ? "native" : "erc20",
        chainId: Number(chainId),
        to,
        token: isNative ? "native" : tokenAddr,
        amount: String(intent.amount),
        amountWei: "",
        ok: false,
        error: String(e?.message || e),
        startedAt,
        endedAt: isoNow(),
      },
    ];
  }
  if (amountWei === "0") {
    return [
      {
        skillId: sk.id,
        type: isNative ? "native" : "erc20",
        chainId: Number(chainId),
        to,
        token: isNative ? "native" : tokenAddr,
        amount: String(intent.amount),
        amountWei,
        ok: false,
        error: "金额必须大于 0",
        startedAt,
        endedAt: isoNow(),
      },
    ];
  }

  const from = localStorage.getItem("walletAddress") || "";
  if (!from) {
    return [
      {
        skillId: sk.id,
        type: isNative ? "native" : "erc20",
        chainId: Number(chainId),
        to,
        token: isNative ? "native" : tokenAddr,
        amount: String(intent.amount),
        amountWei,
        ok: false,
        error: "未连接钱包",
        startedAt,
        endedAt: isoNow(),
      },
    ];
  }

  try {
    let txHash = "";
    if (isNative) {
      txHash = await window.ethereum.request({
        method: "eth_sendTransaction",
        params: [{ from, to, value: `0x${BigInt(amountWei).toString(16)}` }],
      });
    } else {
      txHash = await window.ethereum.request({
        method: "eth_sendTransaction",
        params: [
          {
            from,
            to: tokenAddr,
            data: erc20TransferData(to, amountWei),
            value: "0x0",
          },
        ],
      });
    }

    return [
      {
        skillId: sk.id,
        type: isNative ? "native" : "erc20",
        chainId: Number(chainId),
        to,
        token: isNative ? "native" : tokenAddr,
        amount: String(intent.amount),
        amountWei,
        txHash: String(txHash || ""),
        ok: true,
        startedAt,
        endedAt: isoNow(),
      },
    ];
  } catch (e) {
    const code = Number(e?.code ?? e?.data?.originalError?.code);
    let msg = String(e?.message || e);
    if (code === 4001) msg = "用户取消签名/交易";
    return [
      {
        skillId: sk.id,
        type: isNative ? "native" : "erc20",
        chainId: Number(chainId),
        to,
        token: isNative ? "native" : tokenAddr,
        amount: String(intent.amount),
        amountWei,
        ok: false,
        error: msg,
        startedAt,
        endedAt: isoNow(),
      },
    ];
  }
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

  async function runOne(sk, index) {
    const startedAt = isoNow();
    const filename = String(sk.filename || sk.name || sk.id || "").trim();
    let spec = null;
    try {
      spec = parseX402Spec(sk.content);
    } catch (e) {
      return {
        skillId: sk.id,
        filename,
        ok: false,
        status: 0,
        error: `invalid x402 skill json: ${String(e?.message || e)}`,
        startedAt,
        endedAt: isoNow(),
      };
    }
    if (!spec) {
      return {
        skillId: sk.id,
        filename,
        ok: false,
        status: 0,
        error: "invalid x402 skill spec",
        startedAt,
        endedAt: isoNow(),
      };
    }

    const url = spec.url.replaceAll("{input}", encodeURIComponent(String(userMessage || "")));
    const method = spec.method || "GET";
    const headers = spec.headers || {};

    if (llmStatus) {
      llmStatus.textContent = `x402 执行中（${index + 1}/${toRun.length}）：${filename}`;
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
      return {
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
      };
    } catch (e) {
      return {
        skillId: sk.id,
        filename,
        url,
        method,
        ok: false,
        status: 0,
        error: String(e?.message || e),
        startedAt,
        endedAt: isoNow(),
      };
    }
  }

  const results = new Array(toRun.length);
  const concurrency = Math.min(2, toRun.length);
  let cursor = 0;

  async function worker() {
    while (true) {
      const idx = cursor;
      cursor += 1;
      if (idx >= toRun.length) return;
      results[idx] = await runOne(toRun[idx], idx);
    }
  }

  await Promise.all(Array.from({ length: concurrency }, () => worker()));
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
  if (chainIdDec === 16602) {
    return {
      chainId: "0x40da",
      chainName: "0GAI",
      nativeCurrency: { name: "OG", symbol: "OG", decimals: 18 },
      rpcUrls: ["https://evmrpc-testnet.0g.ai"],
      blockExplorerUrls: ["https://chainscan-galileo.0g.ai"],
    };
  }
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
      state.infts = [];
      renderINFTs([]);
      renderWalletStatus();
      return;
    }
    // Account switch invalidates the previous signature/session.
    localStorage.setItem("walletAddress", next);
    localStorage.removeItem("authToken");
    state.infts = [];
    renderINFTs([]);
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
