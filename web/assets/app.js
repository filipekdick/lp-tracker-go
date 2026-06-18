"use strict";

// Frontend for the LP volatility analyzer. Polls the Go API, renders the pool
// table and a per-pool fee-vs-volatility breakdown. No build step, no deps.

const state = {
  pools: [],
  meta: null,
  position: null,
  nextScan: null,
  filters: { kind: "", verdict: "", protocol: "", search: "" },
  sort: { key: "score", dir: -1 },
  selected: null, // "chainSlug/address"
  protocolsSeen: new Set(),
};

const fmt = {
  usd(n) {
    if (n == null) return "—";
    const abs = Math.abs(n);
    if (abs >= 1e9) return "$" + (n / 1e9).toFixed(2) + "B";
    if (abs >= 1e6) return "$" + (n / 1e6).toFixed(2) + "M";
    if (abs >= 1e3) return "$" + (n / 1e3).toFixed(1) + "K";
    return "$" + n.toFixed(0);
  },
  pct(x) {
    if (x == null) return "—";
    return (x * 100).toFixed(x < 0.1 ? 2 : 1) + "%";
  },
  ratio(x) {
    if (!x || !isFinite(x)) return "—";
    return x.toFixed(2) + "×";
  },
  price(n) {
    if (n == null) return "—";
    if (n >= 1000) return n.toLocaleString(undefined, { maximumFractionDigits: 0 });
    if (n >= 1) return n.toFixed(2);
    if (n >= 0.01) return n.toFixed(4);
    return n.toPrecision(3);
  },
  amount(n) {
    if (n == null) return "—";
    const abs = Math.abs(n);
    if (abs >= 1000) return n.toLocaleString(undefined, { maximumFractionDigits: 0 });
    if (abs >= 1) return n.toFixed(3);
    if (abs >= 0.0001) return n.toFixed(5);
    return n.toPrecision(3);
  },
};

async function getJSON(url, opts) {
  const r = await fetch(url, opts);
  if (!r.ok) throw new Error(url + " -> " + r.status);
  return r.json();
}

async function refresh() {
  try {
    const [meta, poolsResp, posResp] = await Promise.all([
      getJSON("/api/meta"),
      getJSON("/api/pools"),
      getJSON("/api/position").catch(() => ({ tracking: false })),
    ]);
    state.meta = meta;
    state.pools = poolsResp.pools || [];
    state.position = posResp && posResp.tracking ? posResp.position : null;
    state.nextScan = poolsResp.nextScan ? new Date(poolsResp.nextScan) : null;
    renderStatus(meta, poolsResp);
    renderSummary(meta);
    renderPosition();
    syncProtocolFilter();
    renderTable();
    if (state.selected) renderDetail(findSelected());
  } catch (e) {
    document.getElementById("source-label").textContent = "API unreachable";
  }
}

function renderStatus(meta, poolsResp) {
  const dot = document.getElementById("source-dot");
  const label = document.getElementById("source-label");
  label.textContent = "source: " + meta.source;
  dot.className = "dot " + (poolsResp.scanning ? "scanning" : meta.source === "demo" ? "demo" : "live");
}

function renderSummary(meta) {
  const c = meta.counts || {};
  const total = state.pools.length;
  const cards = [
    { k: "Pools tracked", v: total, cls: "" },
    { k: "Attractive", v: c.attractive || 0, cls: "attractive" },
    { k: "Fair", v: c.fair || 0, cls: "fair" },
    { k: "Unattractive", v: c.unattractive || 0, cls: "unattractive" },
    { k: "Chains", v: (meta.chains || []).length, cls: "" },
  ];
  document.getElementById("summary").innerHTML = cards
    .map((c) => `<div class="card ${c.cls}"><div class="k">${c.k}</div><div class="v">${c.v}</div></div>`)
    .join("");
}

// ---- tracked position panel ------------------------------------------------

function kv(k, v) { return `<div class="kv"><span class="k">${k}</span><span class="v">${v}</span></div>`; }

function renderPosition() {
  const section = document.getElementById("tracked");
  const p = state.position;
  if (!p) { section.hidden = true; return; }
  section.hidden = false;

  document.getElementById("tracked-sub").innerHTML =
    `${p.protocol} · ${p.chain} <span class="layer ${p.chainKind}">${p.chainKind}</span> · token #${p.tokenId}`
    + (p.error ? ` · <span style="color:var(--red)">${p.error}</span>` : "");

  const a = p.analysis || {};

  // LP card.
  const rangePill = p.inRange ? '<span class="pill in">in range</span>' : '<span class="pill out">out of range</span>';
  const lp = `<div class="tcard">
    <h3>Liquidity position ${rangePill}</h3>
    ${kv("Pool", p.poolName + " · " + fmt.pct(p.feeTier))}
    ${kv(p.symbol0, fmt.amount(p.amount0))}
    ${kv(p.symbol1, fmt.amount(p.amount1))}
    ${kv("Position value", fmt.usd(p.valueUsd))}
    ${kv("Pool TVL / Vol 24h", fmt.usd(p.tvlUsd) + " / " + fmt.usd(p.volume24hUsd))}
    ${kv("Tick range", p.tickLower + " … " + p.tickUpper + " (now " + p.tickNow + ")")}
  </div>`;

  // Hedge card.
  let hedgeBody = "";
  const hedges = p.hedges || [];
  if (hedges.length === 0) {
    const fallbackH = p.hedge || {};
    if (!fallbackH.available && !fallbackH.symbol) {
      hedgeBody = `<p class="hint">${fallbackH.note || "No hedgeable leg."}</p>`;
    } else {
      hedges.push(fallbackH);
    }
  }

  if (hedges.length > 0) {
    hedges.forEach((h, idx) => {
      if (idx > 0) {
        hedgeBody += `<div style="border-top: 1px solid var(--border); margin: 12px 0;"></div>`;
      }
      const syncPill = h.inSync ? '<span class="pill sync">in sync</span>' : '<span class="pill drift">drift</span>';
      const pnlCls = (h.unrealizedPnl || 0) >= 0 ? "ratio-pos" : "ratio-neg";
      hedgeBody += `${kv("Venue / perp", h.venue + " · " + h.symbol + " " + syncPill)}
        ${kv("LP exposure", fmt.amount(h.lpExposure) + " " + h.exposureSymbol)}
        ${kv("Target short", fmt.amount(h.targetShort) + " " + h.exposureSymbol)}
        ${kv("Current short", h.available ? fmt.amount(h.currentShort) + " " + h.exposureSymbol : "—")}
        ${kv("Drift", h.available ? fmt.amount(h.drift) : "—")}
        ${kv("Entry / mark", (h.entryPrice ? fmt.price(h.entryPrice) : "—") + " / " + (h.markPrice ? fmt.price(h.markPrice) : "—"))}
        ${kv("Short PnL", `<span class="${pnlCls}">${h.available ? fmt.usd(h.unrealizedPnl) : "—"}</span>`)}`;
    });
  }

  const allSync = hedges.length > 0 && hedges.every(h => h.inSync);
  const rebalancePill = hedges.length > 0 ? (allSync ? '<span class="pill sync">in sync</span>' : '<span class="pill drift">rebalance</span>') : "";
  const anyDryRun = hedges.some(h => h.dryRun);
  const notes = hedges.map(h => h.note).filter(n => n).filter((v, i, self) => self.indexOf(v) === i).join(" · ");

  const hedge = `<div class="tcard">
    <h3>Perp short hedge ${rebalancePill}</h3>
    ${hedgeBody}
    ${notes ? `<p class="hint" style="margin-top:8px">${anyDryRun ? "🔒 dry-run · " : ""}${notes}</p>` : ""}
  </div>`;

  // Fee-vs-volatility card with comparison bars.
  const vols = [
    { label: "Fee-implied σ", val: a.feeImpliedVol, cls: "fees" },
    { label: "Realized σ", val: a.realizedVol, cls: "realized" },
  ];
  if (a.hasImplied) vols.push({ label: "Deribit IV", val: a.impliedVol, cls: "implied" });
  const maxVol = Math.max(...vols.map((v) => v.val || 0), 0.0001);
  const verdictCls = a.verdict || "unknown";
  const fee = `<div class="tcard">
    <h3>Fees vs. volatility <span class="verdict ${verdictCls}">${a.verdict || "—"}</span></h3>
    ${kv("Fee APR", fmt.pct(a.feeApr))}
    ${kv("Net edge APR", `<span class="${(a.netEdgeApr||0)>=0?"ratio-pos":"ratio-neg"}">${fmt.pct(a.netEdgeApr)}</span>`)}
    <div class="bars" style="margin-top:10px">
      ${vols.map((v) => `<div class="bar-row">
        <span class="label">${v.label}</span>
        <span class="bar-track"><span class="bar-fill ${v.cls}" style="width:${Math.max(2,(v.val/maxVol)*100)}%"></span></span>
        <span class="val">${fmt.pct(v.val)}</span></div>`).join("")}
    </div>
  </div>`;

  document.getElementById("tracked-grid").innerHTML = lp + hedge + fee;

  const shortsSec = document.getElementById("open-shorts-section");
  if (p.openShorts && p.openShorts.length > 0) {
    shortsSec.hidden = false;
    document.getElementById("open-shorts-body").innerHTML = p.openShorts.map(s => {
      const side = s.size < 0 ? "SHORT" : "LONG";
      const cls = s.unrealizedPnl >= 0 ? "ratio-pos" : "ratio-neg";
      return `<tr>
        <td>${s.symbol}</td>
        <td>${side}</td>
        <td class="num">${fmt.amount(Math.abs(s.size))}</td>
        <td class="num">${fmt.price(s.entryPrice)}</td>
        <td class="num">${fmt.price(s.markPrice)}</td>
        <td class="num"><span class="${cls}">${fmt.usd(s.unrealizedPnl)}</span></td>
        <td class="num">${fmt.usd(Math.abs(s.size) * s.markPrice)}</td>
      </tr>`;
    }).join("");
  } else {
    shortsSec.hidden = true;
  }

  const graphsSec = document.getElementById("graphs-section");
  if (p.initialState && p.history && p.history.length > 0) {
    graphsSec.hidden = false;
    
    const cur = p.history[p.history.length - 1];
    const netPnl = cur.netPnl;
    const initialVal = Math.max(0.01, p.initialState.valueUsd);
    const pct = (netPnl / initialVal) * 100;
    
    const msElapsed = new Date(cur.timestamp) - new Date(p.initialState.timestamp);
    const days = msElapsed / (1000 * 3600 * 24);
    let aprStr = "—";
    if (days > 0.001) {
      const apr = (pct / days) * 365;
      aprStr = apr.toFixed(2) + "%";
    }
    
    const pnlCls = netPnl >= 0 ? "ratio-pos" : "ratio-neg";
    const pnlHtml = `<div style="display:flex; flex-wrap:wrap; gap:2rem; margin-bottom:1rem; padding:1rem; background:var(--bg-card); border-radius:8px; border:1px solid var(--border);">
      <div style="flex:1;">
        <h3 style="margin-top:0">Strategy Net Profit</h3>
        <p class="hint">Since start (${new Date(p.initialState.timestamp).toLocaleString()})</p>
      </div>
      ${kv("Total PnL", `<span class="${pnlCls}">${fmt.usd(netPnl)}</span>`)}
      ${kv("Return", `<span class="${pnlCls}">${pct.toFixed(2)}%</span>`)}
      ${kv("APR", `<span class="${pnlCls}">${aprStr}</span>`)}
      ${kv("Collected Fees", fmt.usd(cur.feesUsd))}
    </div>`;

    let canvasHtml = `<canvas id="pnl-chart" style="width:100%; height:250px;"></canvas>`;
    graphsSec.innerHTML = pnlHtml + canvasHtml;
    
    renderChart(p.history);
  } else {
    graphsSec.hidden = true;
  }
}

function renderChart(history) {
  if (window.pnlChart) { window.pnlChart.destroy(); }
  const ctx = document.getElementById('pnl-chart');
  if (!ctx) return;
  
  const labels = history.map(h => new Date(h.timestamp).toLocaleTimeString());
  const pnlData = history.map(h => h.netPnl);
  const feesData = history.map(h => h.feesUsd);
  
  window.pnlChart = new Chart(ctx.getContext('2d'), {
    type: 'line',
    data: {
      labels: labels,
      datasets: [
        {
          label: 'Net PnL (USD)',
          data: pnlData,
          borderColor: 'rgb(75, 192, 192)',
          tension: 0.1,
          borderWidth: 2,
          pointRadius: 0
        },
        {
          label: 'Fees (USD)',
          data: feesData,
          borderColor: 'rgb(255, 159, 64)',
          tension: 0.1,
          borderWidth: 2,
          borderDash: [5, 5],
          pointRadius: 0
        }
      ]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: { position: 'top' },
        tooltip: { enabled: true }
      },
      scales: {
        y: { ticks: { callback: function(value) { return '$' + value; } } }
      }
    }
  });
}

function syncProtocolFilter() {
  const sel = document.getElementById("protocol-filter");
  let changed = false;
  for (const p of state.pools) {
    if (p.protocol && !state.protocolsSeen.has(p.protocol)) {
      state.protocolsSeen.add(p.protocol);
      changed = true;
    }
  }
  if (!changed) return;
  // Put the focus protocols first, then the rest alphabetically.
  const focus = ["Aerodrome", "Velodrome", "PancakeSwap", "Uniswap"];
  const rest = [...state.protocolsSeen].filter((p) => !focus.includes(p)).sort();
  const ordered = [...focus.filter((p) => state.protocolsSeen.has(p)), ...rest];
  const cur = state.filters.protocol;
  sel.innerHTML = '<option value="">All protocols</option>' +
    ordered.map((p) => `<option value="${p}"${p === cur ? " selected" : ""}>${p}</option>`).join("");
}

function countdown() {
  const el = document.getElementById("scan-label");
  if (!state.nextScan) {
    el.textContent = "waiting for first scan…";
    return;
  }
  const secs = Math.round((state.nextScan - new Date()) / 1000);
  if (secs <= 0) {
    el.textContent = "scanning soon…";
  } else {
    const m = Math.floor(secs / 60);
    const s = secs % 60;
    el.textContent = "next scan in " + (m > 0 ? m + "m " : "") + s + "s";
  }
}

// ---- table -----------------------------------------------------------------

function sortKey(p) {
  switch (state.sort.key) {
    case "name": return p.name.toLowerCase();
    case "chain": return p.chain.toLowerCase();
    case "tvl": return p.tvlUsd;
    case "vol": return p.volume24hUsd;
    case "feeApr": return p.analysis.feeApr;
    case "realizedVol": return p.analysis.realizedVol;
    case "feeImpliedVol": return p.analysis.feeImpliedVol;
    case "netEdge": return p.analysis.netEdgeApr;
    case "ratio": return p.analysis.feeYieldRatio;
    case "verdict": return p.analysis.verdict;
    default: return p.analysis.score;
  }
}

function visiblePools() {
  const f = state.filters;
  let pools = state.pools.filter((p) => {
    if (f.kind && p.chainKind !== f.kind) return false;
    if (f.verdict && p.analysis.verdict !== f.verdict) return false;
    if (f.protocol && p.protocol !== f.protocol) return false;
    if (f.search) {
      const hay = (p.name + " " + p.chain + " " + p.dex + " " + p.baseSymbol + " " + p.quoteSymbol).toLowerCase();
      if (!hay.includes(f.search.toLowerCase())) return false;
    }
    return true;
  });
  pools.sort((a, b) => {
    const ka = sortKey(a), kb = sortKey(b);
    if (ka < kb) return -1 * state.sort.dir;
    if (ka > kb) return 1 * state.sort.dir;
    return 0;
  });
  return pools;
}

function poolId(p) { return p.chainSlug + "/" + p.address; }

function renderTable() {
  const pools = visiblePools();
  const body = document.getElementById("pools-body");
  document.getElementById("empty").hidden = pools.length > 0;

  body.innerHTML = pools
    .map((p) => {
      const a = p.analysis;
      const ratioCls = a.feeYieldRatio >= 1 ? "ratio-pos" : "ratio-neg";
      const edgeCls = a.netEdgeApr >= 0 ? "ratio-pos" : "ratio-neg";
      const sel = poolId(p) === state.selected ? " selected" : "";
      return `<tr class="row${sel}" data-id="${poolId(p)}">
        <td><div class="pool-cell"><span class="pool-name">${p.name}</span><span class="pool-dex">${p.protocol || p.dex}</span></div></td>
        <td><span class="chain-tag">${p.chain}<span class="layer ${p.chainKind}">${p.chainKind}</span></span></td>
        <td class="num">${fmt.usd(p.tvlUsd)}</td>
        <td class="num">${fmt.usd(p.volume24hUsd)}</td>
        <td class="num">${fmt.pct(a.feeApr)}</td>
        <td class="num">${fmt.pct(a.realizedVol)}</td>
        <td class="num">${fmt.pct(a.feeImpliedVol)}</td>
        <td class="num ${edgeCls}">${fmt.pct(a.netEdgeApr)}</td>
        <td class="num ${ratioCls}">${fmt.ratio(a.feeYieldRatio)}</td>
        <td><span class="verdict ${a.verdict}">${a.verdict}</span></td>
      </tr>`;
    })
    .join("");

  body.querySelectorAll("tr.row").forEach((tr) => {
    tr.addEventListener("click", () => {
      state.selected = tr.dataset.id;
      renderTable();
      renderDetail(findSelected());
    });
  });
}

function findSelected() {
  return state.pools.find((p) => poolId(p) === state.selected);
}

// ---- detail panel ----------------------------------------------------------

function renderDetail(p) {
  const empty = document.getElementById("detail-empty");
  const content = document.getElementById("detail-content");
  if (!p) {
    empty.hidden = false;
    content.hidden = true;
    return;
  }
  empty.hidden = true;
  content.hidden = false;

  const a = p.analysis;
  const vols = [
    { label: "Fee-implied σ", val: a.feeImpliedVol, cls: "fees" },
    { label: "Realized σ", val: a.realizedVol, cls: "realized" },
  ];
  if (a.hasImplied) vols.push({ label: "Options-implied σ", val: a.impliedVol, cls: "implied" });
  const maxVol = Math.max(...vols.map((v) => v.val), 0.0001);

  const verdictText = {
    attractive: "Fees are pricing in more volatility than the asset actually shows — LPs are being overpaid for the risk.",
    fair: "Fee income roughly matches the cost of the asset's volatility.",
    unattractive: "The asset's volatility costs more than the pool's fees pay — LPs likely lose to rebalancing.",
    unknown: "Not enough data to judge this pool.",
  }[a.verdict];

  content.innerHTML = `
    <h2>${p.name}</h2>
    <div class="sub">${p.dex} · ${p.chain} <span class="layer ${p.chainKind}">${p.chainKind}</span> · fee ${fmt.pct(p.feeTier)}</div>

    <div class="verdict-banner ${a.verdict}">
      <strong style="text-transform:capitalize">${a.verdict}.</strong> ${verdictText}
    </div>

    <div class="metric-grid">
      <div class="metric"><div class="k">Fee APR</div><div class="v">${fmt.pct(a.feeApr)}</div></div>
      <div class="metric"><div class="k">Fee / Vol ratio</div><div class="v ${a.feeYieldRatio >= 1 ? "ratio-pos" : "ratio-neg"}">${fmt.ratio(a.feeYieldRatio)}</div></div>
      <div class="metric"><div class="k">Net edge APR</div><div class="v ${a.netEdgeApr >= 0 ? "ratio-pos" : "ratio-neg"}">${fmt.pct(a.netEdgeApr)}</div></div>
      <div class="metric"><div class="k">LVR cost (σ²⁄8)</div><div class="v">${fmt.pct(a.lvrCostRealized)}</div></div>
    </div>

    <div class="bars-title">Volatility comparison (annualized)</div>
    <div class="bars">
      ${vols.map((v) => `
        <div class="bar-row">
          <span class="label">${v.label}</span>
          <span class="bar-track"><span class="bar-fill ${v.cls}" style="width:${Math.max(2, (v.val / maxVol) * 100)}%"></span></span>
          <span class="val">${fmt.pct(v.val)}</span>
        </div>`).join("")}
    </div>

    <div class="spark">
      <h3>${p.baseSymbol} price · 7d hourly</h3>
      ${sparkline(p.closes)}
    </div>

    <div class="metric-grid" style="margin-top:14px">
      <div class="metric"><div class="k">TVL</div><div class="v">${fmt.usd(p.tvlUsd)}</div></div>
      <div class="metric"><div class="k">Volume 24h</div><div class="v">${fmt.usd(p.volume24hUsd)}</div></div>
    </div>

    <div class="addr">${p.address}</div>
  `;
}

function sparkline(closes) {
  if (!closes || closes.length < 2) return '<p class="hint">No price history.</p>';
  const w = 320, h = 70, pad = 4;
  const min = Math.min(...closes), max = Math.max(...closes);
  const range = max - min || 1;
  const step = (w - pad * 2) / (closes.length - 1);
  const pts = closes.map((c, i) => {
    const x = pad + i * step;
    const y = pad + (1 - (c - min) / range) * (h - pad * 2);
    return x.toFixed(1) + "," + y.toFixed(1);
  });
  const up = closes[closes.length - 1] >= closes[0];
  const stroke = up ? "#3fb950" : "#f85149";
  const area = `${pad},${h - pad} ${pts.join(" ")} ${w - pad},${h - pad}`;
  return `<svg viewBox="0 0 ${w} ${h}" preserveAspectRatio="none">
    <polygon points="${area}" fill="${stroke}" opacity="0.08" />
    <polyline points="${pts.join(" ")}" fill="none" stroke="${stroke}" stroke-width="1.6" stroke-linejoin="round" />
  </svg>`;
}

// ---- wiring ----------------------------------------------------------------

function wireControls() {
  document.querySelectorAll("#kind-filters .chip").forEach((b) => {
    b.addEventListener("click", () => {
      state.filters.kind = b.dataset.kind;
      setActive("#kind-filters", b);
      renderTable();
    });
  });
  document.querySelectorAll("#verdict-filters .chip").forEach((b) => {
    b.addEventListener("click", () => {
      state.filters.verdict = b.dataset.verdict;
      setActive("#verdict-filters", b);
      renderTable();
    });
  });
  document.getElementById("search").addEventListener("input", (e) => {
    state.filters.search = e.target.value;
    renderTable();
  });
  document.getElementById("protocol-filter").addEventListener("change", (e) => {
    state.filters.protocol = e.target.value;
    renderTable();
  });
  document.querySelectorAll("thead th[data-sort]").forEach((th) => {
    th.addEventListener("click", () => {
      const key = th.dataset.sort;
      if (state.sort.key === key) state.sort.dir *= -1;
      else state.sort = { key, dir: key === "name" || key === "chain" || key === "verdict" ? 1 : -1 };
      renderTable();
    });
  });
  document.getElementById("scan-now").addEventListener("click", async (e) => {
    e.target.disabled = true;
    e.target.textContent = "Scanning…";
    try { await fetch("/api/scan", { method: "POST" }); } catch (_) {}
    setTimeout(async () => {
      await refresh();
      e.target.disabled = false;
      e.target.textContent = "Scan now";
    }, 1500);
  });
}

function setActive(group, btn) {
  document.querySelectorAll(group + " .chip").forEach((b) => b.classList.remove("active"));
  btn.classList.add("active");
}

wireControls();
refresh();
setInterval(refresh, 5000);
setInterval(countdown, 1000);
