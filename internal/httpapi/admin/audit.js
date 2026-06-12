function auditStatusBadge(statusCode) {
  const code = Number(statusCode || 0);
  const cls = code >= 200 && code < 300 ? "ok" : code >= 400 ? "danger" : "warn";
  return `<span class="badge ${cls}">${escapeHTML(code || "unknown")}</span>`;
}

function auditTargetLabel(item) {
  const type = normalizeAuditTargetType(item.target_type || "—");
  const id = item.target_id || "";
  if (!id) {
    return escapeHTML(type);
  }
  return `${escapeHTML(type)}<br><code title="${escapeHTML(id)}">${escapeHTML(shortID(id))}</code>`;
}

function normalizeAuditTargetType(type) {
  if (type === "deliverie") return "delivery";
  return type;
}

async function loadAudit() {
  const table = document.getElementById("auditTable");
  if (!table) {
    return;
  }

  table.innerHTML = `<tr><td colspan="6" class="empty">Loading…</td></tr>`;

  const params = new URLSearchParams({ limit: "25" });
  const action = document.getElementById("auditActionFilter").value.trim();
  const targetType = document.getElementById("auditTargetTypeFilter").value.trim();
  const targetID = document.getElementById("auditTargetIdFilter").value.trim();
  if (action) params.set("action", action);
  if (targetType) params.set("target_type", targetType);
  if (targetID) params.set("target_id", targetID);

  const data = await api(`/v1/audit?${params.toString()}`);
  const items = data.items || [];

  if (!items.length) {
    renderEmpty(table, 6);
    return;
  }

  table.innerHTML = items.map((item) => `
    <tr>
      <td><span class="badge">${escapeHTML(item.action || "unknown")}</span><br><code>${escapeHTML(item.method || "")}</code></td>
      <td>${auditTargetLabel(item)}</td>
      <td>${auditStatusBadge(item.status_code)}</td>
      <td><code title="${escapeHTML(item.path || "")}">${escapeHTML(shortID(item.path || "—"))}</code></td>
      <td>${item.ip ? `<code>${escapeHTML(item.ip)}</code>` : "—"}</td>
      <td>${escapeHTML(formatDate(item.created_at))}</td>
    </tr>
  `).join("");
}

function shouldRefreshAuditAfterAPI(path, options) {
  const method = String(options?.method || "GET").toUpperCase();
  if (method === "GET") {
    return false;
  }
  if (!path || path.startsWith("/v1/audit")) {
    return false;
  }
  return path.startsWith("/v1/sources") ||
    path.startsWith("/v1/events") ||
    path.startsWith("/v1/deliveries") ||
    path.startsWith("/v1/templates");
}

function installAuditAutoRefresh() {
  const originalAPI = window.api;
  if (typeof originalAPI !== "function" || originalAPI.__auditWrapped) {
    return;
  }

  async function auditedAPI(path, options = {}) {
    const result = await originalAPI(path, options);
    if (shouldRefreshAuditAfterAPI(path, options)) {
      window.setTimeout(() => loadAudit().catch((err) => log(err.message, "error")), 300);
    }
    return result;
  }

  auditedAPI.__auditWrapped = true;
  window.api = auditedAPI;
}

function selectedSourceIDForInvestigation() {
  const id = state?.selectedSourceId || "";
  if (!id) {
    log("select a source first", "error");
    return "";
  }
  return id;
}

function scrollToPanel(title) {
  const panels = Array.from(document.querySelectorAll(".panel"));
  const panel = panels.find((item) => item.querySelector("h2")?.textContent === title);
  if (panel) {
    panel.scrollIntoView({ behavior: "smooth", block: "start" });
  }
}

function addInvestigationButton(container, id, label, handler) {
  if (document.getElementById(id)) {
    return;
  }
  const button = document.createElement("button");
  button.type = "button";
  button.id = id;
  button.className = "secondary";
  button.textContent = label;
  button.addEventListener("click", handler);
  container.insertBefore(button, document.getElementById("saveSourceEdit"));
}

function installSourceInvestigationShortcuts() {
  const actions = document.getElementById("saveSourceEdit")?.parentElement;
  if (!actions) {
    return;
  }

  addInvestigationButton(actions, "filterSourceEvents", "Source events", () => {
    const id = selectedSourceIDForInvestigation();
    if (!id) return;
    document.getElementById("sourceFilter").value = id;
    loadEvents().catch((err) => log(err.message, "error"));
    scrollToPanel("Events");
  });

  addInvestigationButton(actions, "filterSourceDeliveries", "Source deliveries", () => {
    const id = selectedSourceIDForInvestigation();
    if (!id) return;
    document.getElementById("deliverySourceFilter").value = id;
    loadDeliveries().catch((err) => log(err.message, "error"));
    scrollToPanel("Deliveries");
  });

  addInvestigationButton(actions, "filterSourceAudit", "Source audit", () => {
    const id = selectedSourceIDForInvestigation();
    if (!id) return;
    document.getElementById("auditTargetTypeFilter").value = "source";
    document.getElementById("auditTargetIdFilter").value = id;
    loadAudit().catch((err) => log(err.message, "error"));
    scrollToPanel("Admin audit log");
  });
}

function initAuditUI() {
  const refreshButton = document.getElementById("refreshAudit");
  if (!refreshButton) {
    return;
  }

  installAuditAutoRefresh();
  installSourceInvestigationShortcuts();

  refreshButton.addEventListener("click", () => loadAudit().catch((err) => log(err.message, "error")));
  document.getElementById("auditActionFilter").addEventListener("keydown", (event) => {
    if (event.key === "Enter") loadAudit().catch((err) => log(err.message, "error"));
  });
  document.getElementById("auditTargetTypeFilter").addEventListener("keydown", (event) => {
    if (event.key === "Enter") loadAudit().catch((err) => log(err.message, "error"));
  });
  document.getElementById("auditTargetIdFilter").addEventListener("keydown", (event) => {
    if (event.key === "Enter") loadAudit().catch((err) => log(err.message, "error"));
  });
  document.getElementById("saveSettings").addEventListener("click", () => {
    window.setTimeout(() => loadAudit().catch((err) => log(err.message, "error")), 250);
  });

  if (settingsReady()) {
    loadAudit().catch((err) => log(err.message, "error"));
  }
}

initAuditUI();
