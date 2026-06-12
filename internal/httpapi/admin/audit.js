function auditStatusBadge(statusCode) {
  const code = Number(statusCode || 0);
  const cls = code >= 200 && code < 300 ? "ok" : code >= 400 ? "danger" : "warn";
  return `<span class="badge ${cls}">${escapeHTML(code || "unknown")}</span>`;
}

function auditTargetLabel(item) {
  const type = item.target_type || "—";
  const id = item.target_id || "";
  if (!id) {
    return escapeHTML(type);
  }
  return `${escapeHTML(type)}<br><code title="${escapeHTML(id)}">${escapeHTML(shortID(id))}</code>`;
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

function initAuditUI() {
  const refreshButton = document.getElementById("refreshAudit");
  if (!refreshButton) {
    return;
  }

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
