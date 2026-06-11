const state = {
  baseUrl: localStorage.getItem("signalbox.baseUrl") || window.location.origin,
  apiKey: localStorage.getItem("signalbox.apiKey") || "",
};

const $ = (id) => document.getElementById(id);
const logBox = $("activityLog");

function log(message, level = "info") {
  const ts = new Date().toISOString();
  logBox.textContent = `[${ts}] ${level.toUpperCase()} ${message}\n` + logBox.textContent;
}

function setStatus(message, ok = false) {
  const el = $("connectionStatus");
  el.textContent = message;
  el.className = ok ? "status ok" : "status bad";
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function shortID(value) {
  const text = String(value ?? "");
  if (text.length <= 16) return text;
  return `${text.slice(0, 8)}…${text.slice(-6)}`;
}

function formatDate(value) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function settingsReady() {
  return Boolean(state.baseUrl && state.apiKey);
}

async function api(path, options = {}) {
  if (!settingsReady()) {
    throw new Error("API base URL and X-API-Key are required");
  }

  const url = `${state.baseUrl.replace(/\/$/, "")}${path}`;
  const headers = {
    "X-API-Key": state.apiKey,
    ...(options.body ? { "Content-Type": "application/json" } : {}),
    ...(options.headers || {}),
  };

  const response = await fetch(url, { ...options, headers });
  const text = await response.text();
  const data = text ? JSON.parse(text) : null;

  if (!response.ok) {
    const error = data?.error || `${response.status} ${response.statusText}`;
    throw new Error(error);
  }

  return data;
}

function renderEmpty(table, colspan = 6) {
  table.innerHTML = `<tr><td colspan="${colspan}" class="empty">No data</td></tr>`;
}

async function loadStats() {
  const stats = await api("/v1/stats");
  $("statTotal").textContent = stats.total_events ?? 0;
  $("statUnique").textContent = stats.unique_events ?? 0;
  $("statDuplicate").textContent = stats.duplicate_events ?? 0;
  $("stat24h").textContent = stats.events_24h ?? 0;
  $("statSources").textContent = `${stats.active_sources ?? 0}/${stats.sources ?? 0}`;
  $("statFailed").textContent = stats.deliveries?.failed ?? 0;
}

async function loadSources() {
  const table = $("sourcesTable");
  table.innerHTML = `<tr><td colspan="6" class="empty">Loading…</td></tr>`;
  const data = await api("/v1/sources");
  const items = data.items || [];

  if (!items.length) {
    renderEmpty(table, 6);
    return;
  }

  table.innerHTML = items.map((item) => `
    <tr>
      <td>${escapeHTML(item.name)}</td>
      <td><code title="${escapeHTML(item.id)}">${escapeHTML(shortID(item.id))}</code></td>
      <td>${item.is_active ? '<span class="badge ok">active</span>' : '<span class="badge danger">inactive</span>'}</td>
      <td>${item.telegram_chat_id ? `<code>${escapeHTML(item.telegram_chat_id)}</code>` : "—"}</td>
      <td>${item.forward_url ? `<code title="${escapeHTML(item.forward_url)}">${escapeHTML(shortID(item.forward_url))}</code>` : "—"}</td>
      <td>${item.forward_hmac_key_set ? '<span class="badge ok">enabled</span>' : '<span class="badge warn">off</span>'}</td>
    </tr>
  `).join("");
}

async function createSource(event) {
  event.preventDefault();
  const name = $("sourceName").value.trim();
  const chat = $("sourceChat").value.trim();
  const forwardUrl = $("sourceForwardUrl").value.trim();
  const forwardKey = $("sourceForwardKey").value.trim();
  const payload = { name };
  if (chat) payload.telegram_chat_id = chat;
  if (forwardUrl) payload.forward_url = forwardUrl;
  if (forwardKey) payload.forward_hmac_key = forwardKey;

  const source = await api("/v1/sources", {
    method: "POST",
    body: JSON.stringify(payload),
  });

  $("sourceName").value = "";
  $("sourceChat").value = "";
  $("sourceForwardUrl").value = "";
  $("sourceForwardKey").value = "";
  await loadSources();
  await loadStats();

  const token = source.token ? ` Token: ${source.token}` : "";
  log(`source created: ${source.name} (${source.id}).${token}`);
}

async function loadEvents() {
  const table = $("eventsTable");
  table.innerHTML = `<tr><td colspan="5" class="empty">Loading…</td></tr>`;

  const params = new URLSearchParams({ limit: "25" });
  const type = $("eventTypeFilter").value.trim();
  const source = $("sourceFilter").value.trim();
  if (type) params.set("type", type);
  if (source) params.set("source", source);

  const data = await api(`/v1/events?${params.toString()}`);
  const items = data.items || [];

  if (!items.length) {
    renderEmpty(table, 5);
    return;
  }

  table.innerHTML = items.map((item) => `
    <tr>
      <td>${escapeHTML(item.event_type || "unknown")}</td>
      <td><code title="${escapeHTML(item.id)}">${escapeHTML(shortID(item.id))}</code></td>
      <td>${item.is_duplicate ? '<span class="badge warn">duplicate</span>' : '<span class="badge ok">unique</span>'}</td>
      <td>${escapeHTML(formatDate(item.created_at))}</td>
      <td><button class="secondary" data-replay="${escapeHTML(item.id)}">Replay</button></td>
    </tr>
  `).join("");

  table.querySelectorAll("[data-replay]").forEach((button) => {
    button.addEventListener("click", () => replayEvent(button.dataset.replay, button));
  });
}

async function replayEvent(id, button) {
  button.disabled = true;
  try {
    await api(`/v1/events/${encodeURIComponent(id)}/replay`, { method: "POST" });
    await loadDeliveries();
    await loadStats();
    log(`event replay queued: ${id}`);
  } finally {
    button.disabled = false;
  }
}

async function loadDeliveries() {
  const table = $("deliveriesTable");
  table.innerHTML = `<tr><td colspan="6" class="empty">Loading…</td></tr>`;

  const params = new URLSearchParams({ limit: "25" });
  const status = $("deliveryStatusFilter").value.trim();
  const channel = $("deliveryChannelFilter").value.trim();
  if (status) params.set("status", status);
  if (channel) params.set("channel", channel);

  const data = await api(`/v1/deliveries?${params.toString()}`);
  const items = data.items || [];

  if (!items.length) {
    renderEmpty(table, 6);
    return;
  }

  table.innerHTML = items.map((item) => `
    <tr>
      <td>${deliveryBadge(item.status)}</td>
      <td><code title="${escapeHTML(item.id)}">${escapeHTML(shortID(item.id))}</code></td>
      <td>${escapeHTML(item.channel)}</td>
      <td>${escapeHTML(item.attempts ?? 0)} / ${escapeHTML(item.max_attempts ?? "—")}</td>
      <td>${item.last_error ? escapeHTML(item.last_error).slice(0, 160) : "—"}</td>
      <td>${item.status === "failed" || item.status === "pending" ? `<button class="secondary" data-retry="${escapeHTML(item.id)}">Retry</button>` : "—"}</td>
    </tr>
  `).join("");

  table.querySelectorAll("[data-retry]").forEach((button) => {
    button.addEventListener("click", () => retryDelivery(button.dataset.retry, button));
  });
}

function deliveryBadge(status) {
  const normalized = String(status || "unknown");
  const cls = normalized === "sent" ? "ok" : normalized === "failed" ? "danger" : "warn";
  return `<span class="badge ${cls}">${escapeHTML(normalized)}</span>`;
}

async function retryDelivery(id, button) {
  button.disabled = true;
  try {
    await api(`/v1/deliveries/${encodeURIComponent(id)}/retry`, { method: "POST" });
    await loadDeliveries();
    await loadStats();
    log(`delivery retry scheduled: ${id}`);
  } finally {
    button.disabled = false;
  }
}

async function refreshAll() {
  try {
    setStatus("Connecting…", false);
    await loadStats();
    await Promise.all([loadSources(), loadEvents(), loadDeliveries()]);
    setStatus("Connected", true);
    log("dashboard refreshed");
  } catch (error) {
    setStatus("Connection error", false);
    log(error.message, "error");
  }
}

function saveSettings() {
  state.baseUrl = $("baseUrl").value.trim() || window.location.origin;
  state.apiKey = $("apiKey").value.trim();
  localStorage.setItem("signalbox.baseUrl", state.baseUrl);
  localStorage.setItem("signalbox.apiKey", state.apiKey);
  log("settings saved");
  refreshAll();
}

function clearSettings() {
  localStorage.removeItem("signalbox.baseUrl");
  localStorage.removeItem("signalbox.apiKey");
  state.baseUrl = window.location.origin;
  state.apiKey = "";
  $("baseUrl").value = state.baseUrl;
  $("apiKey").value = "";
  setStatus("Not connected", false);
  log("settings cleared");
}

function init() {
  $("baseUrl").value = state.baseUrl;
  $("apiKey").value = state.apiKey;

  $("saveSettings").addEventListener("click", saveSettings);
  $("clearSettings").addEventListener("click", clearSettings);
  $("refreshSources").addEventListener("click", () => loadSources().catch((e) => log(e.message, "error")));
  $("refreshEvents").addEventListener("click", () => loadEvents().catch((e) => log(e.message, "error")));
  $("refreshDeliveries").addEventListener("click", () => loadDeliveries().catch((e) => log(e.message, "error")));
  $("createSourceForm").addEventListener("submit", (e) => createSource(e).catch((err) => log(err.message, "error")));
  $("eventTypeFilter").addEventListener("keydown", (e) => { if (e.key === "Enter") loadEvents().catch((err) => log(err.message, "error")); });
  $("sourceFilter").addEventListener("keydown", (e) => { if (e.key === "Enter") loadEvents().catch((err) => log(err.message, "error")); });
  $("deliveryStatusFilter").addEventListener("change", () => loadDeliveries().catch((e) => log(e.message, "error")));
  $("deliveryChannelFilter").addEventListener("keydown", (e) => { if (e.key === "Enter") loadDeliveries().catch((err) => log(err.message, "error")); });
  $("clearLog").addEventListener("click", () => { logBox.textContent = ""; });

  if (settingsReady()) {
    refreshAll();
  }
}

init();
