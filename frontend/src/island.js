// OmniFloat - Dynamic Island HUD Controller & Context Menu

document.addEventListener("DOMContentLoaded", async () => {
  const cpuChip = document.getElementById("chip-cpu");
  const ramChip = document.getElementById("chip-ram");
  const netChip = document.getElementById("chip-net");
  const clockChip = document.getElementById("chip-clock");

  const islandCpuEl = document.getElementById("island-cpu");
  const islandRamEl = document.getElementById("island-ram");
  const islandNetDownEl = document.getElementById("island-net-down");
  const islandNetUpEl = document.getElementById("island-net-up");
  const islandClockEl = document.getElementById("island-clock");

  const searchBtn = document.getElementById("btn-island-search");
  const islandHud = document.getElementById("island-hud");
  const islandLeft = document.querySelector(".island-left");
  const islandBackdropImg = document.getElementById("island-backdrop-img");

  // Context menu elements
  const contextMenu = document.getElementById("island-context-menu");
  const btnPosBottomRight = document.getElementById("ctx-pos-bottom-right");
  const btnPosBottomLeft = document.getElementById("ctx-pos-bottom-left");
  const checkBottomRight = document.getElementById("check-bottom-right");
  const checkBottomLeft = document.getElementById("check-bottom-left");
  const btnOpenLauncher = document.getElementById("ctx-open-launcher");
  const btnOpenSettings = document.getElementById("ctx-open-settings");
  const btnHideIsland = document.getElementById("ctx-hide-island");

  let activeConfig = null;

  // 1. Load initial configuration to setup widgets visibility
  await loadIslandConfig();

  // 2. Hardware and Network telemetry listener
  if (window.runtime) {
    window.runtime.EventsOn("system:stats", (stats) => {
      if (!stats) return;

      if (islandCpuEl) islandCpuEl.textContent = `${stats.cpuPercent}%`;
      if (islandRamEl) islandRamEl.textContent = `${stats.ramPercent}%`;

      if (islandNetDownEl) islandNetDownEl.textContent = `↓ ${stats.netDownText}`;
      if (islandNetUpEl) islandNetUpEl.textContent = `↑ ${stats.netUpText}`;

      if (islandClockEl && stats.clockText) {
        islandClockEl.textContent = stats.clockText;
      }
      syncIslandWidth();
    });

    // Configuration updated listener
    window.runtime.EventsOn("config:updated", (cfg) => {
      if (cfg) {
        activeConfig = cfg;
        applyConfigToIsland(cfg);
      }
    });

    // Live screen backdrop updated for real-time glass blur
    window.runtime.EventsOn("backdrop:updated", (dataUri) => {
      if (islandBackdropImg && dataUri) {
        islandBackdropImg.style.backgroundImage = `url("${dataUri}")`;
      }
    });

    // Close context menu when user clicks outside the Island window
    window.runtime.EventsOn("island:close_menu", () => {
      if (contextMenu) {
        contextMenu.classList.remove("active");
      }
      if (islandHud) {
        islandHud.classList.remove("hidden");
      }
    });
  }

  // 3. Search button opens the centered launcher without closing the island
  if (searchBtn) {
    searchBtn.addEventListener("click", () => {
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.ShowLauncher();
      }
    });
  }

  // 4. Right-Click Context Menu
  document.addEventListener("contextmenu", (e) => {
    e.preventDefault();
    openContextMenu(e);
  });

  document.addEventListener("click", (e) => {
    if (contextMenu && !contextMenu.contains(e.target)) {
      closeContextMenu();
    }
  });

  if (btnPosBottomRight) {
    btnPosBottomRight.addEventListener("click", async () => {
      if (window.go && window.go.main && window.go.main.App) {
        await window.go.main.App.SetIslandPosition("bottom-right");
      }
      closeContextMenu();
    });
  }

  if (btnPosBottomLeft) {
    btnPosBottomLeft.addEventListener("click", async () => {
      if (window.go && window.go.main && window.go.main.App) {
        await window.go.main.App.SetIslandPosition("bottom-left");
      }
      closeContextMenu();
    });
  }

  if (btnOpenLauncher) {
    btnOpenLauncher.addEventListener("click", () => {
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.ShowLauncher();
      }
      closeContextMenu();
    });
  }

  if (btnOpenSettings) {
    btnOpenSettings.addEventListener("click", () => {
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.OpenLauncherSettings();
      }
      closeContextMenu();
    });
  }

  if (btnHideIsland) {
    btnHideIsland.addEventListener("click", async () => {
      closeContextMenu();
      if (activeConfig && window.go && window.go.main && window.go.main.App) {
        activeConfig.island.enabled = false;
        await window.go.main.App.SaveConfig(activeConfig);
      }
    });
  }

  function openContextMenu(e) {
    if (!contextMenu) return;
    if (islandHud) {
      islandHud.classList.add("hidden");
    }
    contextMenu.classList.add("active");
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetIslandMenuOpen(true);
    }
  }

  function closeContextMenu() {
    if (!contextMenu) return;
    if (contextMenu.classList.contains("active")) {
      contextMenu.classList.remove("active");
      if (islandHud) {
        islandHud.classList.remove("hidden");
      }
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SetIslandMenuOpen(false);
      }
    }
  }

  async function loadIslandConfig() {
    if (window.go && window.go.main && window.go.main.App) {
      try {
        const cfg = await window.go.main.App.GetConfig();
        activeConfig = cfg;
        applyConfigToIsland(cfg);
      } catch (err) {
        console.warn("Could not load island config:", err);
      }
    }
  }

  function applyConfigToIsland(cfg) {
    if (!cfg) return;

    // Apply CSS Variables for Glassmorphism over live blurred backdrop
    let rawOp = Number(cfg.opacity);
    if (isNaN(rawOp) || rawOp < 0) rawOp = 0.88;
    const normOp = Math.max(0, Math.min(1, rawOp > 1.0 ? rawOp / 100.0 : rawOp));
    const tintAlpha = (normOp * 0.98).toFixed(2);
    const imgAlpha = Math.min(1, normOp * 1.15).toFixed(2);
    document.documentElement.style.setProperty("--glass-opacity", tintAlpha);
    document.documentElement.style.setProperty("--glass-img-opacity", imgAlpha);
    document.documentElement.style.setProperty("--glass-blur", `${cfg.blurIntensity}px`);

    // Apply Theme (Dark or Tron)
    document.body.classList.remove("theme-tron", "theme-cyan-frost", "theme-violet-aurora", "theme-deep-obsidian");
    if (cfg.theme === "tron") {
      document.body.classList.add("theme-tron");
    }

    // Toggle widgets according to settings
    if (cfg.island) {
      toggleChip(cpuChip, cfg.island.showCPU);
      toggleChip(ramChip, cfg.island.showRAM);
      toggleChip(netChip, cfg.island.showNet);
      toggleChip(clockChip, cfg.island.showClock);
      toggleChip(searchBtn, Boolean(cfg.island.showSearchBtn));

      // Align pill to left or right corner
      const container = document.querySelector(".island-container");
      const isLeft = String(cfg.island.position || "").includes("left");
      if (container) {
        container.classList.toggle("dock-left", isLeft);
      }

      // Context menu position checks
      const isRight = !isLeft;
      if (checkBottomRight) checkBottomRight.textContent = isRight ? "✓" : "";
      if (checkBottomLeft) checkBottomLeft.textContent = !isRight ? "✓" : "";
    }

    requestAnimationFrame(syncIslandWidth);
  }

  let lastMeasuredWidth = 0;
  function syncIslandWidth() {
    if (!islandLeft || !islandHud || islandHud.classList.contains("hidden")) {
      return;
    }
    let contentW = Math.ceil(islandLeft.getBoundingClientRect().width);
    if (searchBtn && !searchBtn.classList.contains("hidden")) {
      contentW += Math.ceil(searchBtn.getBoundingClientRect().width) + 12;
    }
    const totalW = contentW + 28; // 14px left padding + 14px right padding
    if (totalW >= 50 && Math.abs(totalW - lastMeasuredWidth) >= 2) {
      lastMeasuredWidth = totalW;
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SetIslandContentWidth(totalW);
      }
    }
  }

  function toggleChip(el, show) {
    if (!el) return;
    if (show) {
      el.classList.remove("hidden");
    } else {
      el.classList.add("hidden");
    }
  }
});
