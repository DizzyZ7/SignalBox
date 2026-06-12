const sourceHealthState = {
  installed: false,
  incidentInstalled: false,
  lastIncident: null,
};

function sourceHealthBadge(status, label, title) {
  const cls = status === "ok" ? "ok" : status === "danger" ? "danger" : "warn";
  return `<span class="badge ${cls}" title="${escapeHTML(title || label)}">${escapeHTML(label)}</span>`;
}

function sourceHealthCell(sourceID) {
  const button = document.querySelector(`[data-edit-source="${CSS.escape(sourceID)}"]`);
  const row = button?.closest("tr");
  if (!row) {
    return null;
  }
  return row.children[2] || null;
}

async function loadSourceHealth(source) {
  const sourceID = source.id;
  const [events, failed, pending] = await Promise.all([
    api(`/v1/events?source=${encodeURIComponent(sourceID)}&limit=1`),
    api(`/v1/deliveries?source=${encodeURIComponent(sourceID)}&status=failed&limit=1`),
    api(`/v1/deliveries?source=${encodeURIComponent(sourceID)}&status=pending&limit=1`),
  ]);

  const hasEvents = Boolean((events.items || []).length);
  const hasFailed = Boolean((failed.items || []).length);
  const hasPending = Boolean((pending.items || []).length);

  if (!source.is_active) {
    return sourceHealthBadge("danger", "inactive", "Source is disabled");
  }
  if (hasFailed) {
    return sourceHealthBadge("danger", "failed", "Source has failed delivery jobs");
  }
  if (hasPending) {
    return sourceHealthBadge("warn", "pending", "Source has pending delivery jobs");
  }
  if (!hasEvents) {
    return sourceHealthBadge("warn", "quiet", "No events found for this source");
  }
  return sourceHealthBadge("ok", "healthy", "Source is active and has recent stored events");
}

async function refreshSourceHealth() {
  if (!Array.isArray(state.sources) || !state.sources.length) {
    return;
  }

  for (const source of state.sources) {
    const cell = sourceHealthCell(source.id);
    if (!cell) {
      continue;
    }
    const original = source.is_active ? '<span class="badge ok">active</span>' : '<span class="badge danger">inactive</span>';
    cell.innerHTML = `${original}<br><span class="source-health-loading">checking…</span>`;
  }

  await Promise.all(state.sources.map(async (source) => {
    const cell = sourceHealthCell(source.id);
    if (!cell) {
      return;
    }
    try {
      const badge = await loadSourceHealth(source);
      const original = source.is_active ? '<span class="badge ok">active</span>' : '<span class="badge danger">inactive</span>';
      cell.innerHTML = `${original}<br>${badge}`;
    } catch (error) {
      const original = source.is_active ? '<span class="badge ok">active</span>' : '<span class="badge danger">inactive</span>';
      cell.innerHTML = `${original}<br>${sourceHealthBadge("warn", "unknown", error.message)}`;
    }
  }));
}

function incidentMiniTable(items, columns, emptyText) {
  if (!items.length) {
    return `<div class="incident-empty">${escapeHTML(emptyText)}</div>`;
  }
  const head = columns.map((column) => `<th>${escapeHTML(column.label)}</th>`).join("");
  const rows = items.map((item) => `
    <tr>
      ${columns.map((column) => `<td>${column.render(item)}</td>`).join("")}
    </tr>
  `).join("");
  return `<div class="table-wrap incident-table"><table><thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table></div>`;
}

function sourceByPublicID(sourceID) {
  return (state.sources || []).find((source) => source.id === sourceID) || null;
}

function selectedOrTypedIncidentSourceID() {
  const typed = document.getElementById("incidentSourceId")?.value.trim() || "";
  return typed || state.selectedSourceId || "";
}

async function loadSourceIncidentSnapshot(sourceID) {
  const source = sourceByPublicID(sourceID);
  const [events, deliveries, audit] = await Promise.all([
    api(`/v1/events?source=${encodeURIComponent(sourceID)}&limit=5`),
    api(`/v1/deliveries?source=${encodeURIComponent(sourceID)}&limit=5`),
    api(`/v1/audit?target_type=source&target_id=${encodeURIComponent(sourceID)}&limit=5`),
  ]);

  return {
    source,
    events: events.items || [],
    deliveries: deliveries.items || [],
    audit: audit.items || [],
  };
}

function renderSourceIncidentSnapshot(sourceID, snapshot) {
  const target = document.getElementById("sourceIncidentBody");
  if (!target) {
    return;
  }

  sourceHealthState.lastIncident = { sourceID, snapshot, generatedAt: new Date().toISOString() };

  const source = snapshot.source;
  const status = source?.is_active ? sourceHealthBadge("ok", "active", "Source is active") : sourceHealthBadge("danger", "inactive", "Source is disabled or not loaded in current source list");
  const deliveryFailures = snapshot.deliveries.filter((item) => item.status === "failed").length;
  const deliveryPending = snapshot.deliveries.filter((item) => item.status === "pending").length;
  const risk = deliveryFailures > 0 ? sourceHealthBadge("danger", "needs attention", "Failed deliveries found") : deliveryPending > 0 ? sourceHealthBadge("warn", "queue pending", "Pending deliveries found") : sourceHealthBadge("ok", "stable", "No failed/pending deliveries in latest sample");

  const eventsHTML = incidentMiniTable(snapshot.events, [
    { label: "Type", render: (item) => escapeHTML(item.event_type || "unknown") },
    { label: "ID", render: (item) => `<code title="${escapeHTML(item.id)}">${escapeHTML(shortID(item.id))}</code>` },
    { label: "Created", render: (item) => escapeHTML(formatDate(item.created_at)) },
  ], "No recent events for this source");

  const deliveriesHTML = incidentMiniTable(snapshot.deliveries, [
    { label: "Status", render: (item) => deliveryBadge(item.status) },
    { label: "Channel", render: (item) => escapeHTML(item.channel || "unknown") },
    { label: "Attempts", render: (item) => escapeHTML(`${item.attempts ?? 0}/${item.max_attempts ?? "—"}`) },
  ], "No recent delivery jobs for this source");

  const auditHTML = incidentMiniTable(snapshot.audit, [
    { label: "Action", render: (item) => `<span class="badge">${escapeHTML(item.action || "unknown")}</span>` },
    { label: "Status", render: (item) => auditStatusBadge(item.status_code) },
    { label: "Created", render: (item) => escapeHTML(formatDate(item.created_at)) },
  ], "No recent audit events for this source");

  target.innerHTML = `
    <div class="incident-summary">
      <article><span>Source</span><strong>${escapeHTML(source?.name || sourceID)}</strong><code>${escapeHTML(sourceID)}</code></article>
      <article><span>Status</span><strong>${status}</strong></article>
      <article><span>Risk</span><strong>${risk}</strong></article>
      <article><span>Latest sample</span><strong>${snapshot.events.length} events · ${snapshot.deliveries.length} deliveries · ${snapshot.audit.length} audit</strong></article>
    </div>
    <div class="incident-actions">
      <button id="copyIncidentDiagnostics" class="secondary" type="button">Copy diagnostics</button>
    </div>
    <div class="incident-grid">
      <section><h3>Recent events</h3>${eventsHTML}</section>
      <section><h3>Recent deliveries</h3>${deliveriesHTML}</section>
      <section><h3>Recent audit</h3>${auditHTML}</section>
    </div>
  `;

  document.getElementById("copyIncidentDiagnostics")?.addEventListener("click", () => copyIncidentDiagnostics().catch((err) => log(err.message, "error")));
}

function buildIncidentDiagnostics() {
  const incident = sourceHealthState.lastIncident;
  if (!incident) {
    throw new Error("load a source incident snapshot first");
  }

  const { sourceID, snapshot, generatedAt } = incident;
  const source = snapshot.source;
  const lines = [];
  lines.push("SignalBox source diagnostics");
  lines.push(`Generated at: ${generatedAt}`);
  lines.push(`Source: ${source?.name || "unknown"}`);
  lines.push(`Source ID: ${sourceID}`);
  lines.push(`Active: ${source?.is_active ?? "unknown"}`);
  lines.push(`Forward URL configured: ${Boolean(source?.forward_url)}`);
  lines.push(`Forward HMAC configured: ${Boolean(source?.forward_hmac_key_set)}`);
  lines.push("");

  lines.push("Recent events:");
  if (!snapshot.events.length) {
    lines.push("- none");
  } else {
    snapshot.events.forEach((event) => {
      lines.push(`- ${event.id} | type=${event.event_type || "unknown"} | duplicate=${Boolean(event.is_duplicate)} | created=${event.created_at || "unknown"}`);
    });
  }
  lines.push("");

  lines.push("Recent deliveries:");
  if (!snapshot.deliveries.length) {
    lines.push("- none");
  } else {
    snapshot.deliveries.forEach((job) => {
      lines.push(`- ${job.id} | status=${job.status || "unknown"} | channel=${job.channel || "unknown"} | attempts=${job.attempts ?? 0}/${job.max_attempts ?? "unknown"} | error=${job.last_error || "none"}`);
    });
  }
  lines.push("");

  lines.push("Recent audit:");
  if (!snapshot.audit.length) {
    lines.push("- none");
  } else {
    snapshot.audit.forEach((item) => {
      lines.push(`- ${item.id} | action=${item.action || "unknown"} | status=${item.status_code || "unknown"} | path=${item.path || "unknown"} | created=${item.created_at || "unknown"}`);
    });
  }

  return lines.join("\n");
}

async function writeClipboardText(text) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }

  const area = document.createElement("textarea");
  area.value = text;
  area.setAttribute("readonly", "readonly");
  area.style.position = "fixed";
  area.style.left = "-9999px";
  document.body.appendChild(area);
  area.select();
  document.execCommand("copy");
  document.body.removeChild(area);
}

async function copyIncidentDiagnostics() {
  const text = buildIncidentDiagnostics();
  await writeClipboardText(text);
  log("source diagnostics copied to clipboard");
}

async function refreshSourceIncidentSnapshot() {
  const sourceID = selectedOrTypedIncidentSourceID();
  const target = document.getElementById("sourceIncidentBody");
  if (!sourceID) {
    if (target) target.innerHTML = `<div class="incident-empty">Select a source or paste a source public id.</div>`;
    return;
  }

  document.getElementById("incidentSourceId").value = sourceID;
  if (target) target.innerHTML = `<div class="incident-empty">Loading source incident snapshot…</div>`;

  const snapshot = await loadSourceIncidentSnapshot(sourceID);
  renderSourceIncidentSnapshot(sourceID, snapshot);
}

function installSourceIncidentPanel() {
  if (sourceHealthState.incidentInstalled) {
    return;
  }
  const main = document.querySelector("main.grid");
  if (!main) {
    return;
  }
  sourceHealthState.incidentInstalled = true;

  const section = document.createElement("section");
  section.className = "panel wide incident-panel";
  section.innerHTML = `
    <div class="panel-head">
      <div>
        <h2>Source incident snapshot</h2>
        <p>Load a compact investigation view for one source.</p>
      </div>
      <button id="refreshIncidentSnapshot" class="secondary" type="button">Refresh</button>
    </div>
    <div class="filters incident-filters">
      <input id="incidentSourceId" value="" placeholder="source public id">
      <button id="useSelectedSourceForIncident" class="secondary" type="button">Use selected source</button>
    </div>
    <div id="sourceIncidentBody" class="incident-body">
      <div class="incident-empty">Select a source or paste a source public id.</div>
    </div>
  `;
  main.appendChild(section);

  document.getElementById("refreshIncidentSnapshot").addEventListener("click", () => refreshSourceIncidentSnapshot().catch((err) => log(err.message, "error")));
  document.getElementById("useSelectedSourceForIncident").addEventListener("click", () => {
    if (!state.selectedSourceId) {
      log("select a source first", "error");
      return;
    }
    document.getElementById("incidentSourceId").value = state.selectedSourceId;
    refreshSourceIncidentSnapshot().catch((err) => log(err.message, "error"));
  });
  document.getElementById("incidentSourceId").addEventListener("keydown", (event) => {
    if (event.key === "Enter") refreshSourceIncidentSnapshot().catch((err) => log(err.message, "error"));
  });
}

function installSourceHealthRefresh() {
  if (sourceHealthState.installed) {
    return;
  }
  sourceHealthState.installed = true;

  installSourceIncidentPanel();

  const originalLoadSources = window.loadSources;
  if (typeof originalLoadSources === "function") {
    async function loadSourcesWithHealth(...args) {
      const result = await originalLoadSources(...args);
      window.setTimeout(() => refreshSourceHealth().catch((err) => log(err.message, "error")), 100);
      return result;
    }
    window.loadSources = loadSourcesWithHealth;
  }

  document.getElementById("refreshSources")?.addEventListener("click", () => {
    window.setTimeout(() => refreshSourceHealth().catch((err) => log(err.message, "error")), 300);
  });

  document.getElementById("refreshDeliveries")?.addEventListener("click", () => {
    window.setTimeout(() => refreshSourceHealth().catch((err) => log(err.message, "error")), 300);
  });

  document.getElementById("saveSettings")?.addEventListener("click", () => {
    window.setTimeout(() => refreshSourceHealth().catch((err) => log(err.message, "error")), 600);
  });

  if (settingsReady()) {
    window.setTimeout(() => refreshSourceHealth().catch((err) => log(err.message, "error")), 800);
  }
}

installSourceHealthRefresh();
