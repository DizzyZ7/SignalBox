const sourceHealthState = {
  installed: false,
  timer: null,
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

function installSourceHealthRefresh() {
  if (sourceHealthState.installed) {
    return;
  }
  sourceHealthState.installed = true;

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
