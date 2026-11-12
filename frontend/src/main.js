// OmniFloat - Launcher Controller, App Search, History & Settings

document.addEventListener("DOMContentLoaded", async () => {
  const searchInput = document.getElementById("search-input");
  const mathCard = document.getElementById("math-card");
  const mathExpr = document.getElementById("math-expression");
  const mathRes = document.getElementById("math-result");
  const cpuPercentEl = document.getElementById("cpu-val");
  const cpuFillEl = document.getElementById("cpu-fill");
  const ramPercentEl = document.getElementById("ram-val");
  const ramFillEl = document.getElementById("ram-fill");
  const closeBtn = document.getElementById("btn-close");
  const launcherCapsule = document.getElementById("launcher-capsule");
  const glassBackdropImg = document.getElementById("glass-backdrop-img");

  const defaultViewContainer = document.getElementById("default-view-container");
  const searchResultsContainer = document.getElementById("search-results-container");
  const searchResultsList = document.getElementById("search-results-list");
  const resultsTitle = document.getElementById("results-title");

  const historySection = document.getElementById("history-section");
  const historyList = document.getElementById("history-list");
  const btnClearHistory = document.getElementById("btn-clear-history");
  const actionList = document.getElementById("action-list");

  // Settings elements
  const settingsModal = document.getElementById("settings-modal");
  const btnOpenSettings = document.getElementById("btn-open-settings");
  const btnCloseSettings = document.getElementById("btn-close-settings");
  const btnSaveSettings = document.getElementById("btn-save-settings");
  const btnResetSettings = document.getElementById("btn-reset-settings");

  const inputOpacity = document.getElementById("setting-opacity");
  const valOpacity = document.getElementById("val-opacity");
  const inputBlur = document.getElementById("setting-blur");
  const valBlur = document.getElementById("val-blur");
  const switchIsland = document.getElementById("setting-island-enabled");
  const switchWidgetNet = document.getElementById("setting-widget-net");
  const switchWidgetClock = document.getElementById("setting-widget-clock");
  const switchWidgetCpu = document.getElementById("setting-widget-cpu");
  const switchWidgetRam = document.getElementById("setting-widget-ram");
  const switchWidgetSearch = document.getElementById("setting-widget-search");
  const posButtons = document.querySelectorAll(".pos-btn");
  const themeButtons = document.querySelectorAll(".theme-pill");

  let currentConfig = {
    theme: "dark-glass",
    opacity: 0.88,
    blurIntensity: 28,
    island: {
      enabled: true,
      position: "bottom-right",
      showCPU: true,
      showRAM: true,
      showNet: true,
      showClock: true,
      showSearchBtn: false,
    },
  };
  let activeItems = [];
  let selectedIndex = 0;
  let searchDebounceTimer = null;
  let lastEvaluatedResult = null;
  let currentExpression = "";
  let updateDownloadURL = "";

  let isReFocusing = false;
  function focusSearchInput(forceReset = false) {
    if (settingsModal && settingsModal.classList.contains("active")) return;
    if (forceReset && document.activeElement === searchInput) {
      isReFocusing = true;
      searchInput.blur();
      isReFocusing = false;
    }
    searchInput.focus();
    const len = searchInput.value.length;
    try {
      searchInput.setSelectionRange(len, len);
    } catch (_) {}
  }

  // 1. Load initial configuration, history, and restore any saved search input
  await loadConfiguration();
  await loadHistory();
  try {
    const savedQuery = localStorage.getItem("omnifloat_last_query");
    if (savedQuery) {
      searchInput.value = savedQuery;
      const q = savedQuery.trim();
      if (q !== "") {
        if (isMathExpression(q)) {
          await evaluateExpression(q);
        }
        await performAppSearch(q);
      }
    }
  } catch (_) {}
  collectActiveItems();
  updateHighlight();
  focusSearchInput(true);
  setTimeout(() => focusSearchInput(true), 60);
  setTimeout(() => focusSearchInput(true), 150);

  window.addEventListener("focus", () => {
    focusSearchInput(true);
  });

  window.addEventListener("keyup", (e) => {
    if (e.key === "Alt" || e.key === " " || e.code === "Space") {
      focusSearchInput(true);
    }
  });

  // Keep searchInput permanently focused whenever settings modal is closed
  searchInput.addEventListener("blur", () => {
    if (isReFocusing) return;
    if (!settingsModal || !settingsModal.classList.contains("active")) {
      setTimeout(() => {
        if (!settingsModal || !settingsModal.classList.contains("active")) {
          focusSearchInput(false);
        }
      }, 10);
    }
  });

  // 2. Global keyboard handlers (navigation, focus, shortcuts)
  window.addEventListener("keydown", (e) => {
    // Ctrl + , opens settings
    if (e.ctrlKey && e.key === ",") {
      e.preventDefault();
      toggleSettings();
      return;
    }

    // Escape closes settings if open, otherwise hides launcher
    if (e.key === "Escape") {
      if (settingsModal.classList.contains("active")) {
        closeSettings();
      } else {
        hideLauncher();
      }
      return;
    }

    // When settings panel is open, do not intercept arrow keys or typing
    if (settingsModal.classList.contains("active")) {
      return;
    }

    // Global ArrowDown / ArrowUp / Enter navigation across active options
    if (e.key === "ArrowDown") {
      e.preventDefault();
      focusSearchInput();
      collectActiveItems();
      if (activeItems.length > 0) {
        selectedIndex = (selectedIndex + 1) % activeItems.length;
        updateHighlight();
      }
      return;
    }

    if (e.key === "ArrowUp") {
      e.preventDefault();
      focusSearchInput();
      collectActiveItems();
      if (activeItems.length > 0) {
        selectedIndex = (selectedIndex - 1 + activeItems.length) % activeItems.length;
        updateHighlight();
      }
      return;
    }

    if (e.key === "Enter") {
      e.preventDefault();
      collectActiveItems();
      if (activeItems.length > 0) {
        executeCurrentSelection();
      }
      return;
    }

    // Redirect any direct typing or Backspace to the search input automatically
    if (
      document.activeElement !== searchInput &&
      !e.ctrlKey &&
      !e.altKey &&
      !e.metaKey &&
      (e.key.length === 1 || e.key === "Backspace" || e.key === "Delete")
    ) {
      focusSearchInput();
    }
  });

  // 4. Wails Runtime Events
  if (window.runtime) {
    // Telemetry stats event
    window.runtime.EventsOn("system:stats", (stats) => {
      if (!stats) return;
      if (cpuPercentEl) cpuPercentEl.textContent = `${stats.cpuPercent}%`;
      if (cpuFillEl) cpuFillEl.style.width = `${Math.min(stats.cpuPercent, 100)}%`;

      if (ramPercentEl) ramPercentEl.textContent = `${stats.ramPercent}%`;
      if (ramFillEl) ramFillEl.style.width = `${Math.min(stats.ramPercent, 100)}%`;
    });

    // Launcher opened event (Alt+Space or Tray) — preserve existing input text!
    window.runtime.EventsOn("launcher:opened", async () => {
      closeSettings();
      focusSearchInput();
      const query = searchInput.value.trim();
      if (query === "") {
        hideMathCard();
        showDefaultView();
        await loadHistory();
      } else {
        if (isMathExpression(query)) {
          await evaluateExpression(query);
        } else {
          hideMathCard();
        }
        await performAppSearch(query);
      }
      collectActiveItems();
      updateHighlight();
      focusSearchInput(true);
      setTimeout(() => focusSearchInput(true), 60);
      setTimeout(() => focusSearchInput(true), 150);
    });

    window.runtime.EventsOn("launcher:focus_input", () => {
      focusSearchInput(true);
    });

    // Tray "Settings..." menu item event
    window.runtime.EventsOn("launcher:open_settings", () => {
      openSettings();
    });

    // Configuration updated
    window.runtime.EventsOn("config:updated", (cfg) => {
      if (cfg) {
        currentConfig = cfg;
        applyConfigToUI(cfg);
      }
    });

    // Live screen backdrop updated for real-time glass blur
    window.runtime.EventsOn("backdrop:updated", (dataUri) => {
      if (glassBackdropImg && dataUri) {
        glassBackdropImg.style.backgroundImage = `url("${dataUri}")`;
      }
    });
  }

  // 5. Search & Math Input Listener
  searchInput.addEventListener("input", () => {
    try {
      localStorage.setItem("omnifloat_last_query", searchInput.value);
    } catch (_) {}
    const query = searchInput.value.trim();

    clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(async () => {
      if (query === "") {
        hideMathCard();
        showDefaultView();
        await loadHistory();
        return;
      }

      // Check math
      if (isMathExpression(query)) {
        await evaluateExpression(query);
      } else {
        hideMathCard();
      }

      // Search apps and tools
      await performAppSearch(query);
    }, 40);
  });

  // 7. Math Card click execution
  mathCard.addEventListener("click", () => {
    if (lastEvaluatedResult) {
      applyMathResult(currentExpression, lastEvaluatedResult);
    }
  });

  // 8. Default System Suggestions click
  actionList.addEventListener("click", (e) => {
    const item = e.target.closest(".action-item");
    if (!item) return;
    executeItem(item);
  });

  // 9. History items click
  historyList.addEventListener("click", (e) => {
    const item = e.target.closest(".action-item");
    if (!item) return;
    executeItem(item);
  });

  // 10. Search Results click
  searchResultsList.addEventListener("click", (e) => {
    const item = e.target.closest(".action-item");
    if (!item) return;
    executeItem(item);
  });

  // 11. Clear History
  btnClearHistory.addEventListener("click", async (e) => {
    e.stopPropagation();
    if (window.go && window.go.main && window.go.main.App) {
      await window.go.main.App.ClearHistory();
      await loadHistory();
    }
  });

  // 12. Close Button
  closeBtn.addEventListener("click", hideLauncher);

  // ==========================================
  // Core Search & Execution Logic
  // ==========================================

  function showDefaultView() {
    defaultViewContainer.style.display = "block";
    searchResultsContainer.style.display = "none";
    selectedIndex = 0;
    collectActiveItems();
    updateHighlight();
  }

  function showSearchResultsView() {
    defaultViewContainer.style.display = "none";
    searchResultsContainer.style.display = "block";
    selectedIndex = 0;
  }

  async function performAppSearch(query) {
    showSearchResultsView();
    if (!window.go || !window.go.main || !window.go.main.App) return;

    try {
      const apps = await window.go.main.App.SearchApps(query);
      renderSearchResults(apps, query);
    } catch (err) {
      console.error("App search failed:", err);
    }
  }

  function renderSearchResults(apps, query) {
    searchResultsList.innerHTML = "";
    const hasApps = apps && apps.length > 0;

    if (!hasApps) {
      resultsTitle.textContent = `Sin aplicaciones locales — Buscar en la Web`;
    } else {
      resultsTitle.textContent = `Aplicaciones encontradas (${apps.length})`;
      apps.forEach((app) => {
        const div = document.createElement("div");
        div.className = "action-item";
        div.dataset.action = app.path;
        div.dataset.name = app.name;
        div.dataset.type = app.category || "app";

        const iconClass = getAppIconClass(app);
        div.innerHTML = `
          <div class="action-left">
            <div class="action-icon ${iconClass}">
              ${getAppSvgIcon(app)}
            </div>
            <div class="action-text-group">
              <span class="action-name">${escapeHtml(app.name)}</span>
              <span class="action-desc">${escapeHtml(app.description || "Aplicación de Windows")}</span>
            </div>
          </div>
          <span class="action-shortcut">↵ Abrir</span>
        `;
        searchResultsList.appendChild(div);
      });
    }

    // Always append Web Search option (primary option when no local apps match)
    const webDiv = document.createElement("div");
    webDiv.className = "action-item";
    webDiv.dataset.action = query;
    webDiv.dataset.name = query;
    webDiv.dataset.type = "web";
    webDiv.innerHTML = `
      <div class="action-left">
        <div class="action-icon icon-web">
          ${getWebSvg()}
        </div>
        <div class="action-text-group">
          <span class="action-name">Buscar "${escapeHtml(query)}" en el navegador</span>
          <span class="action-desc">Abrir búsqueda en Internet con tu navegador predeterminado</span>
        </div>
      </div>
      <span class="action-shortcut">↵ Buscar en Web</span>
    `;
    searchResultsList.appendChild(webDiv);

    collectActiveItems();
    selectedIndex = 0;
    updateHighlight();
  }

  async function loadHistory() {
    if (!window.go || !window.go.main || !window.go.main.App) return;
    try {
      const history = await window.go.main.App.GetHistory();
      renderHistory(history);
    } catch (err) {
      console.warn("Could not load history:", err);
    }
  }

  function renderHistory(items) {
    historyList.innerHTML = "";
    if (!items || items.length === 0) {
      historySection.style.display = "none";
      collectActiveItems();
      updateHighlight();
      return;
    }

    historySection.style.display = "block";
    items.forEach((item) => {
      const div = document.createElement("div");
      div.className = "action-item";
      div.dataset.action = item.action;
      div.dataset.name = item.title;
      div.dataset.type = item.type;
      div.dataset.raw = item.rawInput;

      const isMath = item.type === "math";
      const isWeb = item.type === "web";
      const iconClass = isMath ? "icon-math" : isWeb ? "icon-web" : "icon-app";
      const iconSvg = isMath ? getMathSvg() : isWeb ? getWebSvg() : getAppSvg();
      const shortcutText = isMath ? "↵ Copiar" : isWeb ? "↵ Buscar" : "↵ Abrir";
      div.innerHTML = `
        <div class="action-left">
          <div class="action-icon ${iconClass}">
            ${iconSvg}
          </div>
          <div class="action-text-group">
            <span class="action-name">${escapeHtml(item.title)}</span>
            <span class="action-desc">${escapeHtml(item.detail)}</span>
          </div>
        </div>
        <span class="action-shortcut">${shortcutText}</span>
      `;
      historyList.appendChild(div);
    });

    collectActiveItems();
    updateHighlight();
  }

  function collectActiveItems() {
    activeItems = [];
    if (mathCard.classList.contains("active")) {
      activeItems.push(mathCard);
    }

    if (searchResultsContainer.style.display !== "none") {
      const results = searchResultsList.querySelectorAll(".action-item");
      results.forEach((el) => activeItems.push(el));
    } else {
      const hist = historyList.querySelectorAll(".action-item");
      hist.forEach((el) => activeItems.push(el));
      const actions = actionList.querySelectorAll(".action-item");
      actions.forEach((el) => activeItems.push(el));
    }
  }

  function updateHighlight() {
    activeItems.forEach((el, idx) => {
      if (idx === selectedIndex) {
        el.classList.add("selected");
        el.scrollIntoView({ block: "nearest" });
      } else {
        el.classList.remove("selected");
      }
    });
  }

  function executeCurrentSelection() {
    if (selectedIndex < 0 || selectedIndex >= activeItems.length) return;
    const selectedEl = activeItems[selectedIndex];

    if (selectedEl === mathCard) {
      applyMathResult(currentExpression, lastEvaluatedResult);
      return;
    }

    executeItem(selectedEl);
  }

  function executeItem(el) {
    const action = el.dataset.action;
    const name = el.dataset.name;
    const type = el.dataset.type;

    if (type === "math") {
      applyMathResult(name, action);
      return;
    }

    if (type === "web") {
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SearchInBrowser(action);
      }
      return;
    }

    if (action === "settings") {
      openSettings();
      return;
    }

    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.LaunchApp(action, name);
    }
  }

  async function applyMathResult(expr, result) {
    if (!result) return;
    try {
      await navigator.clipboard.writeText(result);
    } catch (_) {}

    if (window.go && window.go.main && window.go.main.App) {
      await window.go.main.App.RecordCalculation(expr, result);
    }
    hideLauncher();
  }

  function isMathExpression(input) {
    const trimmed = input.trim();
    if (trimmed.length < 2) return false;
    // Standard math operators and functions
    return /^[0-9\.\s\+\-\*\/\^\(\)\%modpi|esqrtabslongciten]+$/i.test(trimmed) &&
           /[0-9]/.test(trimmed) &&
           /[\+\-\*\/\^\%]|sqrt|abs|sin|cos|tan|log/.test(trimmed);
  }

  async function evaluateExpression(expr) {
    currentExpression = expr;
    if (window.go && window.go.main && window.go.main.App) {
      try {
        const result = await window.go.main.App.EvaluateMath(expr);
        if (result && !result.startsWith("Error")) {
          lastEvaluatedResult = result;
          mathExpr.textContent = `${expr} =`;
          mathRes.textContent = result;
          mathCard.classList.add("active");
          collectActiveItems();
          return;
        }
      } catch (err) {
        console.warn("Math eval error:", err);
      }
    }
    hideMathCard();
  }

  function hideMathCard() {
    mathCard.classList.remove("active");
    lastEvaluatedResult = null;
    collectActiveItems();
  }

  function hideLauncher() {
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.HideLauncher();
    }
  }

  // ==========================================
  // Settings Panel Handling
  // ==========================================

  async function persistCurrentSettings() {
    if (!currentConfig) return;
    const rawVal = parseInt(inputOpacity.value, 10);
    const pct = Math.max(0, Math.min(100, isNaN(rawVal) ? 88 : rawVal));
    currentConfig.opacity = Number((pct / 100).toFixed(2));
    currentConfig.blurIntensity = parseInt(inputBlur.value, 10) || 28;
    if (!currentConfig.island) {
      currentConfig.island = {
        enabled: true,
        position: "bottom-right",
        showCPU: true,
        showRAM: true,
        showNet: true,
        showClock: true,
      };
    }
    currentConfig.island.enabled = Boolean(switchIsland.checked);
    currentConfig.island.showNet = Boolean(switchWidgetNet.checked);
    currentConfig.island.showClock = Boolean(switchWidgetClock.checked);
    currentConfig.island.showCPU = Boolean(switchWidgetCpu.checked);
    currentConfig.island.showRAM = Boolean(switchWidgetRam.checked);
    if (switchWidgetSearch) {
      currentConfig.island.showSearchBtn = Boolean(switchWidgetSearch.checked);
    }

    if (window.go && window.go.main && window.go.main.App) {
      try {
        await window.go.main.App.SaveConfig(currentConfig);
      } catch (err) {
        console.warn("Could not save config:", err);
      }
    }
  }

  btnOpenSettings.addEventListener("click", (e) => {
    e.stopPropagation();
    toggleSettings();
  });
  btnCloseSettings.addEventListener("click", (e) => {
    e.stopPropagation();
    closeSettings();
  });

  btnSaveSettings.addEventListener("click", async (e) => {
    e.stopPropagation();
    await persistCurrentSettings();
    closeSettings();
  });

  btnResetSettings.addEventListener("click", async (e) => {
    e.stopPropagation();
    if (window.go && window.go.main && window.go.main.App) {
      currentConfig = await window.go.main.App.ResetConfig();
      applyConfigToUI(currentConfig);
    }
  });

  let liveSaveTimer = null;
  function scheduleLivePersist() {
    clearTimeout(liveSaveTimer);
    liveSaveTimer = setTimeout(() => {
      persistCurrentSettings();
    }, 90);
  }

  function setGlassOpacityFromPercent(pct) {
    // Map user percentage (0% - 100%) directly to both the glass tint alpha (0.00 - 0.98)
    // and the blurred live backdrop opacity (0.00 - 1.00) so lowering opacity reveals
    // true transparency through the window.
    const norm = Math.max(0, Math.min(100, pct)) / 100;
    const tintAlpha = (norm * 0.98).toFixed(2);
    const imgAlpha = Math.min(1, norm * 1.15).toFixed(2);
    document.documentElement.style.setProperty("--glass-opacity", tintAlpha);
    document.documentElement.style.setProperty("--glass-img-opacity", imgAlpha);
  }

  inputOpacity.addEventListener("input", () => {
    const rawVal = parseInt(inputOpacity.value, 10);
    const pct = Math.max(0, Math.min(100, isNaN(rawVal) ? 88 : rawVal));
    valOpacity.textContent = `${pct}%`;
    setGlassOpacityFromPercent(pct);
    scheduleLivePersist();
  });

  inputOpacity.addEventListener("change", () => {
    persistCurrentSettings();
  });

  inputBlur.addEventListener("input", () => {
    valBlur.textContent = `${inputBlur.value}px`;
    document.documentElement.style.setProperty("--glass-blur", `${inputBlur.value}px`);
    scheduleLivePersist();
  });

  inputBlur.addEventListener("change", () => {
    persistCurrentSettings();
  });

  [switchIsland, switchWidgetNet, switchWidgetClock, switchWidgetCpu, switchWidgetRam, switchWidgetSearch].forEach((sw) => {
    if (!sw) return;
    sw.addEventListener("change", () => {
      persistCurrentSettings();
    });
  });

  posButtons.forEach((btn) => {
    btn.addEventListener("click", async (e) => {
      e.stopPropagation();
      posButtons.forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
      if (currentConfig && currentConfig.island) {
        currentConfig.island.position = btn.dataset.pos;
      }
      await persistCurrentSettings();
    });
  });

  themeButtons.forEach((pill) => {
    pill.addEventListener("click", async (e) => {
      e.stopPropagation();
      themeButtons.forEach((p) => p.classList.remove("active"));
      pill.classList.add("active");
      if (currentConfig) {
        currentConfig.theme = pill.dataset.theme;
        applyTheme(currentConfig.theme);
      }
      await persistCurrentSettings();
    });
  });

  function toggleSettings() {
    if (settingsModal.classList.contains("active")) {
      closeSettings();
    } else {
      openSettings();
    }
  }

  async function openSettings() {
    await loadConfiguration();
    settingsModal.classList.add("active");
    if (launcherCapsule) launcherCapsule.classList.add("settings-open");
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetSettingsOpen(true);
    }
  }

  function closeSettings() {
    settingsModal.classList.remove("active");
    if (launcherCapsule) launcherCapsule.classList.remove("settings-open");
    searchInput.focus();
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetSettingsOpen(false);
    }
  }

  async function loadConfiguration() {
    if (window.go && window.go.main && window.go.main.App) {
      try {
        const cfg = await window.go.main.App.GetConfig();
        if (cfg) {
          currentConfig = cfg;
          applyConfigToUI(currentConfig);
        }
      } catch (err) {
        console.warn("Could not load config:", err);
      }
    }
  }

  function applyConfigToUI(cfg) {
    if (!cfg) return;
    // Config stores opacity as 0.00–1.00, slider uses 0–100 integers
    let rawOp = Number(cfg.opacity);
    if (isNaN(rawOp) || rawOp < 0) rawOp = 0.88;
    let opacityPct = rawOp <= 1.0 ? Math.round(rawOp * 100) : Math.round(rawOp);
    opacityPct = Math.max(0, Math.min(100, opacityPct));

    inputOpacity.value = String(opacityPct);
    valOpacity.textContent = `${opacityPct}%`;
    setGlassOpacityFromPercent(opacityPct);

    const blurPx = Math.max(10, Math.min(50, Number(cfg.blurIntensity) || 28));
    inputBlur.value = String(blurPx);
    valBlur.textContent = `${blurPx}px`;
    document.documentElement.style.setProperty("--glass-blur", `${blurPx}px`);

    applyTheme(cfg.theme);

    if (cfg.island) {
      switchIsland.checked = Boolean(cfg.island.enabled);
      switchWidgetNet.checked = Boolean(cfg.island.showNet);
      switchWidgetClock.checked = Boolean(cfg.island.showClock);
      switchWidgetCpu.checked = Boolean(cfg.island.showCPU);
      switchWidgetRam.checked = Boolean(cfg.island.showRAM);
      if (switchWidgetSearch) {
        switchWidgetSearch.checked = Boolean(cfg.island.showSearchBtn);
      }

      posButtons.forEach((b) => {
        b.classList.toggle("active", b.dataset.pos === cfg.island.position);
      });
    }
  }

  function applyTheme(theme) {
    document.body.classList.remove("theme-tron", "theme-cyan-frost", "theme-violet-aurora", "theme-deep-obsidian");
    const activeTheme = theme === "tron" ? "tron" : "dark-glass";
    if (activeTheme === "tron") {
      document.body.classList.add("theme-tron");
    }
    themeButtons.forEach((pill) => {
      pill.classList.toggle("active", pill.dataset.theme === activeTheme);
    });
  }

  // Helpers
  function escapeHtml(str) {
    return String(str || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  function getAppIconClass(app) {
    const id = (app.id || "").toLowerCase();
    if (id.includes("terminal") || id.includes("powershell") || id.includes("cmd")) return "icon-terminal";
    if (id.includes("calc")) return "icon-calc";
    if (id.includes("notepad")) return "icon-notepad";
    if (id.includes("explorer")) return "icon-explorer";
    if (id.includes("taskmgr")) return "icon-taskmgr";
    if (id.includes("settings") || id.includes("control")) return "icon-settings";
    return "icon-app";
  }

  function getAppSvgIcon(app) {
    const id = (app.id || "").toLowerCase();
    if (id.includes("terminal") || id.includes("cmd") || id.includes("powershell")) {
      return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"></polyline><line x1="12" y1="19" x2="20" y2="19"></line></svg>`;
    }
    if (id.includes("calc")) {
      return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="2" width="16" height="20" rx="2"></rect><line x1="8" y1="6" x2="16" y2="6"></line></svg>`;
    }
    return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 2 7 12 12 22 7 12 2"></polygon><polyline points="2 17 12 22 22 17"></polyline><polyline points="2 12 12 17 22 12"></polyline></svg>`;
  }

  function getAppSvg() {
    return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 2 7 12 12 22 7 12 2"></polygon><polyline points="2 17 12 22 22 17"></polyline><polyline points="2 12 12 17 22 12"></polyline></svg>`;
  }

  function getMathSvg() {
    return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>`;
  }

  function getWebSvg() {
    return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="2" y1="12" x2="22" y2="12"></line><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path></svg>`;
  }
});
