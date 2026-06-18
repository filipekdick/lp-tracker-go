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
    const s = Math.abs(n).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    return (n < 0 ? "-$" : "$") + s;
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
    if (Math.abs(n) >= 1) return n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 4 });
    return n.toPrecision(4);
  },
  amount(n) {
    if (n == null) return "—";
    return n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 6 });
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
    .map((c) => {
      let clickHandler = "";
      if (c.k === "Attractive" || c.k === "Fair" || c.k === "Unattractive") {
        clickHandler = `onclick="window.setVerdictFilter('${c.k.toLowerCase()}')" style="cursor: pointer" title="Click to view pools"`;
      }
      return `<div class="card ${c.cls}" ${clickHandler}><div class="k">${c.k}</div><div class="v">${c.v}</div></div>`;
    })
    .join("");
}

window.setVerdictFilter = function(verdict) {
  document.querySelectorAll('#verdict-filters .chip').forEach(btn => {
    if (btn.dataset.verdict === verdict) {
      btn.click();
    }
  });
  document.getElementById('pools').scrollIntoView({behavior: 'smooth'});
};

// ---- tracked position panel ------------------------------------------------

function kv(k, v) { return `<div class="kv"><span class="k">${k}</span><span class="v">${v}</span></div>`; }

// driftCell renders the hedge drift as both base-asset units and a percentage
// of the target short, coloured green when the hedge is in sync and red when it
// has drifted — so it's obvious at a glance whether the sync is keeping up.
function driftCell(h) {
  const target = Math.abs(h.targetShort || 0);
  const pct = target > 0 ? (h.drift / target) * 100 : 0;
  const cls = h.inSync ? "ratio-pos" : "ratio-neg";
  const sign = h.drift > 0 ? "+" : "";
  return `<span class="${cls}">${sign}${fmt.amount(h.drift)} ${h.exposureSymbol} · ${sign}${pct.toFixed(2)}%</span>`;
}

function renderPosition() {
  const section = document.getElementById("tracked");
  const p = state.position;
  if (!p) { section.hidden = true; return; }
  section.hidden = false;

  document.getElementById("tracked-sub").innerHTML =
    `${p.protocol} · ${p.chain} <span class="layer ${p.chainKind}">${p.chainKind}</span> · token #${p.tokenId}`
    + (p.error ? ` · <span style="color:var(--red)">${p.error}</span>` : "");

  const a = p.analysis || {};

  // ---- derived position metrics --------------------------------------------
  const price0 = p.price0 || 0, price1 = p.price1 || 0;
  // Uncollected = live claimable fees; collected = already withdrawn over the
  // position's life. Fall back to the legacy tokensOwed alias if needed.
  const unc0 = p.uncollectedFees0 != null ? p.uncollectedFees0 : (p.tokensOwed0 || 0);
  const unc1 = p.uncollectedFees1 != null ? p.uncollectedFees1 : (p.tokensOwed1 || 0);
  const col0 = p.collectedFees0 || 0, col1 = p.collectedFees1 || 0;
  const fees0 = unc0 * price0, fees1 = unc1 * price1;
  const feesUsd = fees0 + fees1;
  const colUsd0 = col0 * price0, colUsd1 = col1 * price1;
  const collectedUsd = colUsd0 + colUsd1;
  const curV0 = (p.amount0 || 0) * price0;
  const curV1 = (p.amount1 || 0) * price1;
  const totalV = (curV0 + curV1) || p.valueUsd || 0;
  const hedgePnl = (p.hedges || []).reduce((s, h) => s + (h.unrealizedPnl || 0), 0);

  // HODL comparison & impermanent loss need a captured baseline.
  let depositVal = null, hodlVal = null, il = null, hodlV0 = null, hodlV1 = null;
  let netPnl = null, roi = null;
  if (p.initialState) {
    hodlV0 = (p.initialState.amount0 || 0) * price0;
    hodlV1 = (p.initialState.amount1 || 0) * price1;
    hodlVal = hodlV0 + hodlV1;
    depositVal = p.initialState.valueUsd;
    il = totalV - hodlVal;
  }
  if (p.history && p.history.length) {
    netPnl = p.history[p.history.length - 1].netPnl;
    if (depositVal > 0) roi = (netPnl / depositVal) * 100;
  }
  const pnlV = netPnl != null ? netPnl
    : (totalV - (depositVal != null ? depositVal : totalV)) + hedgePnl + feesUsd;

  const cls = n => (n >= 0 ? "ratio-pos" : "ratio-neg");
  const signed = n => (n >= 0 ? "+" : "") + fmt.usd(n);
  const rangePill = p.inRange ? '<span class="pill in">in range</span>' : '<span class="pill out">out of range</span>';

  // ---- top summary cards (Krystal-style big numbers) -----------------------
  const summaryCards = `
    <div class="pcard">
      <div class="pcard-head"><span class="pcard-ico">💧</span>Total value ${rangePill}</div>
      <div class="pcard-big">${fmt.usd(totalV)}</div>
      <div class="pcard-subs">
        ${kv("Deposits", depositVal != null ? fmt.usd(depositVal) : "—")}
        ${kv(p.symbol0, fmt.usd(curV0))}
        ${kv(p.symbol1, fmt.usd(curV1))}
      </div>
    </div>
    <div class="pcard">
      <div class="pcard-head"><span class="pcard-ico">🪙</span>Earning</div>
      <div class="pcard-big ratio-pos">${fmt.usd(feesUsd + collectedUsd)}</div>
      <div class="pcard-subs">
        ${kv("Unclaimed", fmt.usd(feesUsd))}
        ${kv("Claimed", fmt.usd(collectedUsd))}
        ${kv("Pool fee APR", fmt.pct(a.feeApr))}
      </div>
    </div>
    <div class="pcard">
      <div class="pcard-head"><span class="pcard-ico">📈</span>Profit &amp; loss</div>
      <div class="pcard-big ${cls(pnlV)}">${signed(pnlV)}</div>
      <div class="pcard-subs">
        ${kv("vs HODL (IL)", il != null ? `<span class="${cls(il)}">${fmt.usd(il)}</span>` : "—")}
        ${kv("ROI", roi != null ? `<span class="${cls(roi)}">${roi.toFixed(2)}%</span>` : "—")}
        ${kv("Hedge PnL", `<span class="${cls(hedgePnl)}">${fmt.usd(hedgePnl)}</span>`)}
      </div>
    </div>`;

  // ---- Liquidity: current allocation vs HODL -------------------------------
  const liqRow = (sym, curAmt, curUsd, hodlAmt, hodlUsd) => {
    const curPct = totalV > 0 ? (curUsd / totalV) * 100 : 0;
    const hodlPct = hodlVal > 0 ? (hodlUsd / hodlVal) * 100 : 0;
    const hodlCell = hodlAmt != null
      ? `<span class="liq-amt">${fmt.amount(hodlAmt)} <span class="liq-badge">${hodlPct.toFixed(0)}%</span></span><span class="liq-usd">${fmt.usd(hodlUsd)}</span>`
      : `<span class="liq-amt">—</span>`;
    return `<div class="liq-row">
      <div class="liq-sym">${sym}</div>
      <div class="liq-col"><span class="liq-amt">${fmt.amount(curAmt)} <span class="liq-badge">${curPct.toFixed(0)}%</span></span><span class="liq-usd">${fmt.usd(curUsd)}</span></div>
      <div class="liq-col">${hodlCell}</div>
    </div>`;
  };
  const liq = `<div class="tcard">
    <h3>Liquidity ${rangePill}</h3>
    <div class="liq-head"><span class="liq-sym">${p.poolName} · ${fmt.pct(p.feeTier)}</span><span>Current</span><span>HODL</span></div>
    ${liqRow(p.symbol0, p.amount0, curV0, p.initialState ? p.initialState.amount0 : null, hodlV0)}
    ${liqRow(p.symbol1, p.amount1, curV1, p.initialState ? p.initialState.amount1 : null, hodlV1)}
    <div class="kv liq-total"><span class="k">Impermanent loss</span><span class="v">${il != null ? `<span class="${cls(il)}">${fmt.usd(il)}</span>` : "—"}</span></div>
    <p class="hint" style="margin-top:8px">Tick ${p.tickLower} … ${p.tickUpper} (now ${p.tickNow}) · Pool TVL ${fmt.usd(p.tvlUsd)}</p>
  </div>`;

  // ---- Fees & Rewards: unclaimed (uncollected) vs claimed (collected) ------
  const feeRow = (sym, unAmt, unUsd, clAmt, clUsd) => `<div class="liq-row">
    <div class="liq-sym">${sym}</div>
    <div class="liq-col"><span class="liq-amt">${fmt.amount(unAmt)}</span><span class="liq-usd">${fmt.usd(unUsd)}</span></div>
    <div class="liq-col"><span class="liq-amt">${fmt.amount(clAmt)}</span><span class="liq-usd">${fmt.usd(clUsd)}</span></div>
  </div>`;
  const feesRewards = `<div class="tcard">
    <h3>Fees &amp; Rewards</h3>
    <div class="liq-head"><span></span><span>Unclaimed</span><span>Claimed</span></div>
    ${feeRow(p.symbol0, unc0, fees0, col0, colUsd0)}
    ${feeRow(p.symbol1, unc1, fees1, col1, colUsd1)}
    <div class="kv liq-total"><span class="k">Total fees &amp; rewards</span><span class="v"><span class="ratio-pos">${fmt.usd(feesUsd + collectedUsd)}</span></span></div>
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
        ${kv("Drift", h.available ? driftCell(h) : "—")}
        ${kv("Notional", h.available && h.notionalUsd ? fmt.usd(h.notionalUsd) : "—")}
        ${kv("Entry / mark", (h.entryPrice ? fmt.price(h.entryPrice) : "—") + " / " + (h.markPrice ? fmt.price(h.markPrice) : "—"))}
        ${kv("Short PnL", `<span class="${pnlCls}">${h.available ? fmt.usd(h.unrealizedPnl) : "—"}</span>`)}`;
    });
  }

  const allSync = hedges.length > 0 && hedges.every(h => h.inSync);
  const rebalancePill = hedges.length > 0 ? (allSync ? '<span class="pill sync">in sync</span>' : '<span class="pill drift">rebalance</span>') : "";
  const anyDryRun = hedges.some(h => h.dryRun);
  const notes = hedges.map(h => h.note).filter(n => n).filter((v, i, self) => self.indexOf(v) === i).join(" · ");

  let hedge = `<div class="tcard">
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

  document.getElementById("position-summary").innerHTML = summaryCards;
  document.getElementById("tracked-grid").innerHTML = liq + feesRewards + hedge + fee;

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

  const ordersSec = document.getElementById("open-orders-section");
  if (p.openLimitOrders && p.openLimitOrders.length > 0) {
    ordersSec.hidden = false;
    document.getElementById("open-orders-body").innerHTML = p.openLimitOrders.map(o => {
      // Find mark price from open shorts if available
      let distStr = "—";
      const pos = p.openShorts?.find(s => s.symbol === o.symbol);
      if (pos && pos.markPrice > 0) {
        const dist = (o.price - pos.markPrice) / pos.markPrice;
        distStr = `<span class="${Math.abs(dist) < 0.01 ? "ratio-pos" : "ratio-neg"}">${fmt.pct(dist)}</span>`;
      }
      
      const fillPct = o.origQty > 0 ? (o.executedQty / o.origQty) : 0;
      
      return `<tr>
        <td>${o.symbol}</td>
        <td>${o.side}</td>
        <td class="num">${fmt.price(o.price)}</td>
        <td class="num">${distStr}</td>
        <td class="num">${fmt.amount(o.origQty)}</td>
        <td class="num">${fmt.pct(fillPct)}</td>
        <td class="num"><button onclick="window.cancelOrder('${o.symbol}', ${o.orderId})" style="cursor:pointer; padding: 2px 8px; border-radius: 4px; border: 1px solid var(--border); background: var(--bg); color: #ff5555;">Cancel</button></td>
      </tr>`;
    }).join("");
  } else {
    ordersSec.hidden = true;
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
        <h3 style="margin-top:0; display:flex; align-items:center;">
          Strategy Net Profit
          <button onclick="document.getElementById('pnl-chart-wrapper').style.display = document.getElementById('pnl-chart-wrapper').style.display === 'none' ? 'block' : 'none'" style="margin-left:1rem; padding:2px 8px; font-size:12px; cursor:pointer; background:var(--bg); border:1px solid var(--border); color:var(--fg); border-radius:4px;">Toggle Graph</button>
        </h3>
        <p class="hint">Since start (${new Date(p.initialState.timestamp).toLocaleString()})</p>
      </div>
      ${kv("Total PnL", `<span class="${pnlCls}">${fmt.usd(netPnl)}</span>`)}
      ${kv("Return", `<span class="${pnlCls}">${pct.toFixed(2)}%</span>`)}
      ${kv("APR", `<span class="${pnlCls}">${aprStr}</span>`)}
      ${kv("Collected Fees", fmt.usd(cur.feesUsd))}
    </div>`;

    let canvasHtml = `<div id="pnl-chart-wrapper" style="position: relative; height: 250px; width: 100%;"><canvas id="pnl-chart"></canvas></div>`;
    graphsSec.innerHTML = pnlHtml + canvasHtml;
    
    renderChart(p.history);
  } else {
    graphsSec.hidden = true;
  }
}

function renderChart(history) {
  const ctx = document.getElementById('pnl-chart');
  if (!ctx) return;

  // Chart.js loads from a CDN; if it's blocked or still loading, degrade
  // gracefully instead of throwing and aborting the whole render.
  if (typeof Chart === "undefined") {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent = "Chart.js could not be loaded (offline?) — graph unavailable.";
    ctx.replaceWith(note);
    return;
  }

  if (window.pnlChart) { window.pnlChart.destroy(); }

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
          pointRadius: 2
        },
        {
          label: 'Fees (USD)',
          data: feesData,
          borderColor: 'rgb(255, 159, 64)',
          tension: 0.1,
          borderWidth: 2,
          borderDash: [5, 5],
          pointRadius: 2
        }
      ]
    },
    options: {
      animation: false,
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
window.cancelOrder = async function(symbol, orderId) {
  try {
    const r = await fetch(`/api/orders?symbol=${encodeURIComponent(symbol)}&orderId=${orderId}`, { method: 'DELETE' });
    if (!r.ok) throw new Error(await r.text());
    refresh();
  } catch (e) {
    alert('Failed to cancel order: ' + e);
  }
};
