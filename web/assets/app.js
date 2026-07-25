"use strict";

// Frontend for the LP volatility analyzer. Polls the Go API, renders the pool
// table and a per-pool fee-vs-volatility breakdown. No build step, no deps.

const state = {
  pools: [],
  meta: null,
  position: null,
  nextScan: null,
  scanning: false,
  filters: { kind: "", verdict: "", protocol: "", search: "" },
  sort: { key: "score", dir: -1 },
  selected: null, // "chainSlug/address"
  protocolsSeen: new Set(),
  method: "close7d", // selected realized-volatility method
  horizonDays: 1, // horizon T (days) for the ±σ range bands and expected edge
  k: 1, // band width in sigmas (1 or 2) for the bands and expected edge
  page: new URLSearchParams(window.location.search).get("page") === "pools" ? "pools" : "positions",
};

// view returns the analysis fields for a pool under the currently selected
// volatility method. The server precomputes every method (one OHLCV call per
// pool) into analysis.methods; when that method is usable we read its sigma and
// the metrics recomputed from it, otherwise we fall back to the legacy top-level
// (7-day) fields so older payloads still render.
function view(p) {
  const a = (p && p.analysis) || {};
  const m = a.methods && a.methods[state.method];
  const base = {
    feeApr: a.feeApr,
    positionFeeApr: a.positionFeeApr,
    impliedVol: a.impliedVol,
    hasImplied: a.hasImplied,
    concentratedSim: a.concentratedSim,
  };
  if (m && m.ok) {
    return Object.assign(base, {
      realizedVol: m.realizedVol,
      feeImpliedVol: m.feeImpliedVol,
      lvrCost: m.lvrCost,
      netEdgeApr: m.netEdgeApr,
      feeYieldRatio: m.feeYieldRatio,
      verdict: m.verdict,
      ok: true,
    });
  }
  return Object.assign(base, {
    realizedVol: a.realizedVol,
    feeImpliedVol: a.feeImpliedVol,
    lvrCost: a.lvrCostRealized,
    netEdgeApr: a.netEdgeApr,
    feeYieldRatio: a.feeYieldRatio,
    verdict: a.verdict,
    ok: a.realizedVol > 0,
  });
}

// volHeadroom mirrors analyzer.VolHeadroom: how many times the breakeven
// (fee-implied) volatility exceeds the realized volatility, σ_be/σ, where the
// breakeven σ_be = √(8·feeAPR) is the server's fee-implied σ (a.feeImpliedVol).
// It is concentration-invariant — fees AND impermanent loss are amplified by the
// same factor C, so C and the position width cancel (the full-range form of the
// Gemini concentrated breakeven 2·√(feeAPR·w)). Above 1× means fees overpay for
// the volatility that actually happened. It is the √ of the old fee/vol APR ratio.
function volHeadroom(feeImpliedVol, realizedVol) {
  if (!feeImpliedVol || !realizedVol || realizedVol <= 0) return null;
  return feeImpliedVol / realizedVol;
}

// expectedTimeInRange mirrors analyzer.ExpectedTimeInRangeDays: the idealized
// mean time the price stays inside a ±kσ band before touching an edge, under
// driftless GBM with instant re-centring. The first-passage mean δ²/σ² with the
// band's own width δ=k·σ·√(T/365) cancels σ and gives k²·T days. Informational
// only — it does NOT depend on volatility (the band is sized by σ) and never
// feeds the verdict. Idealized mean, not a guarantee; the price can re-touch the
// edge several times.
function expectedTimeInRange(k, T) {
  if (!k || k <= 0 || !T || T <= 0) return null;
  return k * k * T; // days
}

// bandSim returns the concentrated-APR projection for the currently selected
// ±kσ band (the server computes bands "1" and "2"), or null when none was
// attached (no vol signal / no fee flow).
function bandSim(a, k) {
  const cs = a.concentratedSim;
  if (!cs || !cs.bands) return null;
  return cs.bands[String(k)] || null;
}

// rangeBands returns the lognormal ±1σ and ±2σ containment bands for a sigma
// over the selected horizon T (days): variance grows linearly in time so the
// log half-width is z·σ·√(T/365); price bounds are multiplicative (exp), so the
// up move is slightly larger in magnitude than the down move. These mirror the
// server's analyzer.LognormalBands so the horizon toggle is instant.
function rangeBands(sigma, T) {
  if (!sigma || sigma <= 0 || !T || T <= 0) return null;
  const s = sigma * Math.sqrt(T / 365);
  return {
    up1: Math.exp(s) - 1,
    dn1: 1 - Math.exp(-s),
    up2: Math.exp(2 * s) - 1,
    dn2: 1 - Math.exp(-2 * s),
  };
}

const fmt = {
  usd(n) {
    if (n == null) return "—";
    const s = Math.abs(n).toLocaleString("en-US", {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
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
  // duration formats an expected time in range: days, or hours when under a day.
  duration(days) {
    if (days == null || !isFinite(days)) return "—";
    if (days < 1) return (days * 24).toFixed(days * 24 < 10 ? 1 : 0) + "h";
    return days.toFixed(days < 10 ? 1 : 0) + "d";
  },
  price(n) {
    if (n == null) return "—";
    if (Math.abs(n) >= 1)
      return n.toLocaleString("en-US", {
        minimumFractionDigits: 2,
        maximumFractionDigits: 4,
      });
    return n.toPrecision(4);
  },
  amount(n) {
    if (n == null) return "—";
    return n.toLocaleString("en-US", {
      minimumFractionDigits: 2,
      maximumFractionDigits: 6,
    });
  },
};

async function getJSON(url, opts) {
  const r = await fetch(url, opts);
  if (!r.ok) throw new Error(url + " -> " + r.status);
  return r.json();
}

async function refresh() {
  try {
    if (state.page === "positions") {
      const [meta, posResp] = await Promise.all([
        getJSON("/api/meta"),
        getJSON("/api/position").catch(() => ({ tracking: false })),
      ]);
      state.meta = meta;
      state.position = posResp && posResp.tracking ? posResp.position : null;
      renderStatus(meta, meta);
      renderPosition();
      return;
    }

    const [meta, poolsResp] = await Promise.all([
      getJSON("/api/meta"),
      getJSON("/api/pools"),
    ]);
    state.meta = meta;
    state.pools = poolsResp.pools || [];
    state.nextScan = poolsResp.nextScan ? new Date(poolsResp.nextScan) : null;
    state.scanning = !!poolsResp.scanning;
    renderStatus(meta, poolsResp);
    renderSummary(meta);
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
  dot.className =
    "dot " +
    (poolsResp.scanning
      ? "scanning"
      : meta.source === "demo"
        ? "demo"
        : "live");
}

function renderSummary(meta) {
  // Recompute verdict counts client-side under the selected method, so the
  // summary cards stay consistent with the table when the method changes.
  const c = { attractive: 0, fair: 0, unattractive: 0, unknown: 0 };
  for (const p of state.pools) {
    const v = view(p).verdict || "unknown";
    if (c[v] != null) c[v]++;
  }
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

window.setVerdictFilter = function (verdict) {
  document.querySelectorAll("#verdict-filters .chip").forEach((btn) => {
    if (btn.dataset.verdict === verdict) {
      btn.click();
    }
  });
  document.getElementById("pools").scrollIntoView({ behavior: "smooth" });
};

// ---- tracked position panel ------------------------------------------------

function kv(k, v) {
  return `<div class="kv"><span class="k">${k}</span><span class="v">${v}</span></div>`;
}

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

// renderPortfolioCard lists every tracked LP position and the per-asset exposure
// summed across them (synthetics folded into their underlying), which the single
// simplified short hedges. Returns "" for a single position so the original
// three-card layout is unchanged.
function renderPortfolioCard(p) {
  const positions = p.positions || [];
  const exposures = p.exposures || [];
  if (positions.length <= 1) return "";

  const posRows = positions
    .map((pos) => {
      const inPill = pos.inRange
        ? '<span class="pill in">in</span>'
        : '<span class="pill out">out</span>';
      const apr = pos.analysis
        ? fmt.pct(pos.analysis.positionFeeApr || pos.analysis.feeApr)
        : "—";
      return `<div class="liq-row">
        <div class="liq-sym">${pos.poolName} ${inPill}</div>
        <div class="liq-col"><span class="liq-amt">#${pos.tokenId}</span><span class="liq-usd">${fmt.usd(pos.valueUsd)}</span></div>
        <div class="liq-col"><span class="liq-amt">${pos.protocol || ""}</span><span class="liq-usd">APR ${apr}</span></div>
      </div>`;
    })
    .join("");

  const expRows = exposures
    .map((ex) => {
      const srcSyms = (ex.sources || [])
        .map((s) => s.symbol)
        .filter((v, i, self) => self.indexOf(v) === i)
        .join(", ");
      return `<div class="liq-row">
        <div class="liq-sym">${ex.asset} <span class="liq-badge">${ex.perp}</span></div>
        <div class="liq-col"><span class="liq-amt">${fmt.amount(ex.amount)} ${ex.asset}</span><span class="liq-usd">${ex.sources ? ex.sources.length : 0} pos · ${srcSyms}</span></div>
      </div>`;
    })
    .join("");

  return `<div class="tcard">
    <h3>Portfolio · ${positions.length} positions</h3>
    <p class="hint" style="margin:4px 0 8px">Exposure to the same asset across every LP — including synthetic variants — is summed and hedged with one short per asset.</p>
    <div class="liq-rows">${posRows}</div>
    <div style="border-top: 1px solid var(--border); margin: 12px 0;"></div>
    <h3 style="margin-bottom:6px">Aggregated exposure</h3>
    <div class="liq-rows">${expRows}</div>
  </div>`;
}

function renderPosition() {
  const section = document.getElementById("tracked");
  const p = state.position;
  if (!p) {
    section.hidden = true;
    return;
  }
  section.hidden = false;

  const posCount = (p.positions || []).length;
  const idLabel =
    posCount > 1
      ? `${posCount} positions · single aggregated hedge`
      : `token #${p.tokenId}`;
  document.getElementById("tracked-sub").innerHTML =
    `${p.protocol} · ${p.chain} <span class="layer ${p.chainKind}">${p.chainKind}</span> · ${idLabel}` +
    (p.error ? ` · <span style="color:var(--red)">${p.error}</span>` : "");

  const a = p.analysis || {};

  // ---- derived position metrics --------------------------------------------
  const price0 = p.price0 || 0,
    price1 = p.price1 || 0;
  // Uncollected = live claimable fees. Fall back to the legacy tokensOwed alias.
  const unc0 =
    p.uncollectedFees0 != null ? p.uncollectedFees0 : p.tokensOwed0 || 0;
  const unc1 =
    p.uncollectedFees1 != null ? p.uncollectedFees1 : p.tokensOwed1 || 0;
  const fees0 = unc0 * price0,
    fees1 = unc1 * price1;
  const feesUsd = fees0 + fees1;
  const curV0 = (p.amount0 || 0) * price0;
  const curV1 = (p.amount1 || 0) * price1;
  const totalV = curV0 + curV1 || p.valueUsd || 0;
  const hedgeUnrealized = (p.hedges || []).reduce(
    (s, h) => s + (h.unrealizedPnl || 0),
    0,
  );
  // Hedge income ledger (cumulative USD since inception, re-derived from Binance
  // income history). Signs: realized & funding signed; commissions a paid cost.
  const hedgeRealized = p.hedgeRealizedPnl || 0;
  const hedgeFunding = p.hedgeFundingUsd || 0;
  const hedgeCommissions = p.hedgeCommissionsUsd || 0;
  // Complete hedge PnL = cumulative realized (closed legs) + current unrealized
  // (open position). The two are disjoint, so they add without double-counting.
  const hedgePnl = hedgeRealized + hedgeUnrealized;
  // LP fee ledger: what a harvest realizes now vs the cumulative total that
  // survives harvests. Fall back to the live uncollected USD for old payloads.
  const feesToCollect =
    p.feesToCollectUsd != null ? p.feesToCollectUsd : feesUsd;
  const feesTotal = p.feesTotalUsd != null ? p.feesTotalUsd : feesUsd;

  // HODL comparison & impermanent loss need a captured baseline.
  let depositVal = null,
    hodlVal = null,
    il = null,
    hodlV0 = null,
    hodlV1 = null;
  let netPnl = null,
    roi = null;
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
  const pnlV =
    netPnl != null
      ? netPnl
      : totalV -
        (depositVal != null ? depositVal : totalV) +
        hedgePnl +
        hedgeFunding -
        hedgeCommissions +
        feesToCollect;

  const cls = (n) => (n >= 0 ? "ratio-pos" : "ratio-neg");
  const signed = (n) => (n >= 0 ? "+" : "") + fmt.usd(n);
  const rangePill = p.inRange
    ? '<span class="pill in">in range</span>'
    : '<span class="pill out">out of range</span>';

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
      <div class="pcard-big ratio-pos">${fmt.usd(feesTotal)}</div>
      <div class="pcard-subs">
        ${kv("Fees to collect", fmt.usd(feesToCollect))}
        ${kv("Total since start", fmt.usd(feesTotal))}
        ${kv("Fee APR", fmt.pct(a.positionFeeApr || a.feeApr))}
      </div>
    </div>
    <div class="pcard">
      <div class="pcard-head"><span class="pcard-ico">📈</span>Profit &amp; loss</div>
      <div class="pcard-big ${cls(pnlV)}">${signed(pnlV)}</div>
      <div class="pcard-subs">
        ${kv("vs HODL (IL)", il != null ? `<span class="${cls(il)}">${fmt.usd(il)}</span>` : "—")}
        ${kv("ROI", roi != null ? `<span class="${cls(roi)}">${roi.toFixed(2)}%</span>` : "—")}
        ${kv("Hedge PnL (real.+unr.)", `<span class="${cls(hedgePnl)}">${fmt.usd(hedgePnl)}</span>`)}
      </div>
    </div>`;

  // ---- Liquidity: current allocation vs HODL -------------------------------
  const liqRow = (sym, curAmt, curUsd, hodlAmt, hodlUsd) => {
    const curPct = totalV > 0 ? (curUsd / totalV) * 100 : 0;
    const hodlPct = hodlVal > 0 ? (hodlUsd / hodlVal) * 100 : 0;
    const hodlCell =
      hodlAmt != null
        ? `<span class="liq-amt">${fmt.amount(hodlAmt)} <span class="liq-badge">${hodlPct.toFixed(0)}%</span></span><span class="liq-usd">${fmt.usd(hodlUsd)}</span>`
        : `<span class="liq-amt">—</span>`;
    return `<div class="liq-row">
      <div class="liq-sym">${sym}</div>
      <div class="liq-col"><span class="liq-amt">${fmt.amount(curAmt)} <span class="liq-badge">${curPct.toFixed(0)}%</span></span><span class="liq-usd">${fmt.usd(curUsd)}</span></div>
      <div class="liq-col">${hodlCell}</div>
    </div>`;
  };
  // Position range, folded into the Liquidity card. Notional prices come from the
  // on-chain ticks (server-side via v3math, no API calls); within-range reads 0%
  // at the lower tick and 100% at the upper tick.
  let within = p.rangePositionPct;
  if (within == null && p.tickUpper !== p.tickLower) {
    within = ((p.tickNow - p.tickLower) / (p.tickUpper - p.tickLower)) * 100;
  }
  const clampedWithin = Math.max(0, Math.min(100, within == null ? 0 : within));
  const rangeBlock = `
    <div class="kv liq-total" style="margin-top:8px"><span class="k">Range (${p.symbol1}/${p.symbol0})</span>
      <span class="v">${p.rangeLowerPrice != null ? fmt.price(p.rangeLowerPrice) : "—"} … ${p.rangeUpperPrice != null ? fmt.price(p.rangeUpperPrice) : "—"}</span></div>
    <div class="kv"><span class="k">Current price</span><span class="v">${p.rangeCurrentPrice != null ? fmt.price(p.rangeCurrentPrice) : "—"}</span></div>
    <div class="kv"><span class="k">Within range</span><span class="v">${within == null ? "—" : clampedWithin.toFixed(1) + "%"}</span></div>
    <div class="bar-track" style="margin-top:6px"><span class="bar-fill realized" style="width:${clampedWithin}%"></span></div>`;
  const liq = `<div class="tcard">
    <h3>Liquidity Position ${rangePill}</h3>
    <div class="liq-head"><span class="liq-sym">${p.poolName} · ${fmt.pct(p.feeTier)}</span><span>Current</span><span>HODL</span></div>
    ${liqRow(p.symbol0, p.amount0, curV0, p.initialState ? p.initialState.amount0 : null, hodlV0)}
    ${liqRow(p.symbol1, p.amount1, curV1, p.initialState ? p.initialState.amount1 : null, hodlV1)}
    <div class="kv liq-total"><span class="k">Impermanent loss</span><span class="v">${il != null ? `<span class="${cls(il)}">${fmt.usd(il)}</span>` : "—"}</span></div>
    <div class="kv liq-total" style="margin-top: 8px; font-weight: bold; color: var(--fg);">
      <span class="k">Live Token Prices</span>
      <span class="v">${p.symbol0} ${fmt.price(p.price0)} &nbsp;&middot;&nbsp; ${p.symbol1} ${fmt.price(p.price1)}</span>
    </div>
    ${rangeBlock}
    <p class="hint" style="margin-top:8px">Tick ${p.tickLower} … ${p.tickUpper} (now ${p.tickNow}) · Pool TVL ${fmt.usd(p.tvlUsd)}</p>
  </div>`;

  // ---- Fees & Rewards (with the fee-vs-volatility analysis merged in) -------
  const feeRow = (sym, unAmt, unUsd) => `<div class="liq-row">
    <div class="liq-sym">${sym}</div>
    <div class="liq-col"><span class="liq-amt">${fmt.amount(unAmt)}</span><span class="liq-usd">${fmt.usd(unUsd)}</span></div>
  </div>`;

  // Fee-vs-volatility analysis (uses the selected σ method / horizon / band).
  const av = view(p);
  const vols = [
    { label: "Breakeven σ", val: av.feeImpliedVol, cls: "fees" },
    { label: "Realized σ", val: av.realizedVol, cls: "realized" },
  ];
  if (av.hasImplied)
    vols.push({ label: "Deribit IV", val: av.impliedVol, cls: "implied" });
  const maxVol = Math.max(...vols.map((v) => v.val || 0), 0.0001);
  const verdictCls = av.verdict || "unknown";
  const pb = rangeBands(av.realizedVol, state.horizonDays);
  const Tlbl = state.horizonDays < 1 ? state.horizonDays * 24 + "h" : state.horizonDays + "d";
  const upK = pb ? (state.k === 2 ? pb.up2 : pb.up1) : null;
  const dnK = pb ? (state.k === 2 ? pb.dn2 : pb.dn1) : null;
  const bandLine = pb
    ? kv(
        `LP range ±${state.k}σ / ${Tlbl}`,
        `<span class="ratio-pos">+${fmt.pct(upK)}</span> / <span class="ratio-neg">-${fmt.pct(dnK)}</span>`,
      )
    : "";
  const tir = expectedTimeInRange(state.k, state.horizonDays);
  const tirLine = kv(`Time in range ±${state.k}σ`, fmt.duration(tir));
  const headroom = volHeadroom(av.feeImpliedVol, av.realizedVol);
  const headLine = kv(
    "Vol headroom",
    headroom == null
      ? "—"
      : `<span class="${headroom >= 1 ? "ratio-pos" : "ratio-neg"}">${headroom.toFixed(2)}×</span>`,
  );

  const feesRewards = `<div class="tcard fees-card">
    <h3>Fees &amp; Rewards <span class="verdict ${verdictCls}">${av.verdict || "—"}</span></h3>
    <div class="liq-head"><span></span><span>Unclaimed</span></div>
    ${feeRow(p.symbol0, unc0, fees0)}
    ${feeRow(p.symbol1, unc1, fees1)}
    <div class="kv liq-total"><span class="k">Fees to collect (now)</span><span class="v"><span class="ratio-pos">${fmt.usd(feesToCollect)}</span></span></div>
    <div class="kv liq-total"><span class="k" title="Cumulative collected + uncollected since strategy start. Survives harvests — it never resets when fees are collected.">Total fees since start</span><span class="v"><span class="ratio-pos">${fmt.usd(feesTotal)}</span></span></div>
    <div class="kv liq-total" style="margin-top:8px"><span class="k">Fee APR</span><span class="v">${fmt.pct(av.positionFeeApr || av.feeApr)}</span></div>
    ${kv("Net edge APR", `<span class="${(av.netEdgeApr || 0) >= 0 ? "ratio-pos" : "ratio-neg"}">${fmt.pct(av.netEdgeApr)}</span>`)}
    ${headLine}
    ${bandLine}
    ${tirLine}
    <div class="bars" style="margin-top:10px">
      ${vols
        .map(
          (v) => `<div class="bar-row">
        <span class="label">${v.label}</span>
        <span class="bar-track"><span class="bar-fill ${v.cls}" style="width:${Math.max(2, (v.val / maxVol) * 100)}%"></span></span>
        <span class="val">${fmt.pct(v.val)}</span></div>`,
        )
        .join("")}
    </div>
    <p class="hint" style="margin-top:8px">±σ bands are range-sizing guidance (≈68%/≈95% in range over T), not an edge. Breakeven σ = √(8·feeAPR) is optimistic — it assumes full range; the pool's real on-chain concentration would lower it — and counts swap fees only (excludes farm/gauge incentives like AERO). Time in range is an idealized mean, not a guarantee, and undercounts edge re-touches.</p>
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
      const syncPill = h.inSync
        ? '<span class="pill sync">in sync</span>'
        : '<span class="pill drift">drift</span>';
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

  // Cumulative hedge PnL ledger (re-derived from Binance income history since
  // inception), shown once for the whole hedge rather than per-leg. Realized and
  // funding are signed; commissions are a paid cost shown negative.
  if (hedges.length > 0) {
    const partialNote = p.hedgeIncomePartial
      ? ' <span class="pill drift" title="Strategy inception predates the income lookback window, so these totals are partial.">partial</span>'
      : "";
    hedgeBody += `<div style="border-top: 1px solid var(--border); margin: 12px 0;"></div>
      ${kv("Realized PnL (closed legs)", `<span class="${cls(hedgeRealized)}">${fmt.usd(hedgeRealized)}</span>`)}
      ${kv("Unrealized PnL (open)", `<span class="${cls(hedgeUnrealized)}">${fmt.usd(hedgeUnrealized)}</span>`)}
      ${kv("Complete hedge PnL", `<span class="${cls(hedgePnl)}">${fmt.usd(hedgePnl)}</span>`)}
      ${kv("Funding (signed)" + partialNote, `<span class="${cls(hedgeFunding)}">${fmt.usd(hedgeFunding)}</span>`)}
      ${kv("Commissions paid", `<span class="ratio-neg">${fmt.usd(-hedgeCommissions)}</span>`)}`;
  }

  const allSync = hedges.length > 0 && hedges.every((h) => h.inSync);
  const rebalancePill =
    hedges.length > 0
      ? allSync
        ? '<span class="pill sync">in sync</span>'
        : '<span class="pill drift">rebalance</span>'
      : "";
  const anyDryRun = hedges.some((h) => h.dryRun);
  const notes = hedges
    .map((h) => h.note)
    .filter((n) => n)
    .filter((v, i, self) => self.indexOf(v) === i)
    .join(" · ");

  let hedge = `<div class="tcard">
    <h3>Perp short hedge ${rebalancePill}</h3>
    ${hedgeBody}
    ${notes ? `<p class="hint" style="margin-top:8px">${anyDryRun ? "🔒 dry-run · " : ""}${notes}</p>` : ""}
  </div>`;

  // Portfolio card: every tracked LP position and the per-asset exposure summed
  // across them (synthetics folded in), which the single simplified short hedges.
  const portfolio = renderPortfolioCard(p);

  document.getElementById("position-summary").innerHTML = summaryCards;
  // Second row: Liquidity Position (with range details), Fees & Rewards (with the
  // fee-vs-volatility analysis), the Perp hedge, and — when several positions are
  // tracked — the aggregated portfolio card.
  document.getElementById("tracked-grid").innerHTML =
    liq + feesRewards + hedge + portfolio;

  const shortsSec = document.getElementById("open-shorts-section");
  if (p.openShorts && p.openShorts.length > 0) {
    shortsSec.hidden = false;
    document.getElementById("open-shorts-body").innerHTML = p.openShorts
      .map((s) => {
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
      })
      .join("");
  } else {
    shortsSec.hidden = true;
  }

  const ordersSec = document.getElementById("open-orders-section");
  if (p.openLimitOrders && p.openLimitOrders.length > 0) {
    ordersSec.hidden = false;
    document.getElementById("open-orders-body").innerHTML = p.openLimitOrders
      .map((o) => {
        // Find mark price from open shorts if available
        let distStr = "—";
        const pos = p.openShorts?.find((s) => s.symbol === o.symbol);
        if (pos && pos.markPrice > 0) {
          const dist = (o.price - pos.markPrice) / pos.markPrice;
          distStr = `<span class="${Math.abs(dist) < 0.01 ? "ratio-pos" : "ratio-neg"}">${fmt.pct(dist)}</span>`;
        }

        const fillPct = o.origQty > 0 ? o.executedQty / o.origQty : 0;

        return `<tr>
        <td>${o.symbol}</td>
        <td>${o.side}</td>
        <td class="num">${fmt.price(o.price)}</td>
        <td class="num">${distStr}</td>
        <td class="num">${fmt.amount(o.origQty)}</td>
        <td class="num">${fmt.pct(fillPct)}</td>
        <td class="num"><button onclick="window.cancelOrder('${o.symbol}', ${o.orderId})" style="cursor:pointer; padding: 2px 8px; border-radius: 4px; border: 1px solid var(--border); background: var(--bg); color: #ff5555;">Cancel</button></td>
      </tr>`;
      })
      .join("");
  } else {
    ordersSec.hidden = true;
  }

  renderDailyReturns(p);
}

function renderDailyReturns(p) {
  const section = document.getElementById("history-section");
  const body = document.getElementById("daily-returns-body");
  const note = document.getElementById("daily-returns-note");
  const days = p.dailyReturns || [];
  if (!section || !body || !days.length) {
    if (section) section.hidden = true;
    return;
  }

  section.hidden = false;
  const todayUTC = new Date().toISOString().slice(0, 10);
  body.innerHTML = [...days]
    .reverse()
    .map((d) => {
      const value = (n) => `<span class="${n >= 0 ? "ratio-pos" : "ratio-neg"}">${fmt.usd(n)}</span>`;
      const gauge = d.gaugeRewardsAvailable
        ? value(d.gaugeRewardsUsd || 0)
        : '<span class="muted" title="Aerodrome gauge reward collection is not connected yet">—</span>';
      const partial = d.date === todayUTC ? ' <span class="pill drift">partial</span>' : "";
      return `<tr>
        <td>${d.date}${partial}</td>
        <td class="num">${value(d.lpValueChangeUsd || 0)}</td>
        <td class="num">${value(d.lpFeesUsd || 0)}</td>
        <td class="num">${gauge}</td>
        <td class="num">${value(d.hedgePnlUsd || 0)}</td>
        <td class="num">${value(d.fundingUsd || 0)}</td>
        <td class="num"><span class="ratio-neg">${fmt.usd(-(d.tradingFeesPaidUsd || 0))}</span></td>
        <td class="num daily-net">${value(d.netReturnUsd || 0)}</td>
        <td class="num daily-net"><span class="${(d.returnPct || 0) >= 0 ? "ratio-pos" : "ratio-neg"}">${fmt.pct(d.returnPct || 0)}</span></td>
      </tr>`;
    })
    .join("");

  const hasGauge = days.some((d) => d.gaugeRewardsAvailable);
  note.textContent = hasGauge
    ? "Net return = LP value change + LP fees + gauge rewards + hedge P&L + funding − trading fees."
    : "Aerodrome gauge rewards are not connected to the live event indexer yet, so that column is shown as unavailable rather than as a false $0. Other values are persisted in PostgreSQL when DATABASE_URL is configured.";
}

function renderChart(p) {
  const ctx = document.getElementById("pnl-chart");
  if (!ctx) return;

  // Chart.js loads from a CDN; if it's blocked or still loading, degrade
  // gracefully instead of throwing and aborting the whole render.
  if (typeof Chart === "undefined") {
    const note = document.createElement("p");
    note.className = "hint";
    note.textContent =
      "Chart.js could not be loaded (offline?) — graph unavailable.";
    ctx.replaceWith(note);
    return;
  }

  if (window.pnlChart) {
    window.pnlChart.destroy();
  }

  const history = p.history || [];
  const sym0 = p.symbol0 || "Token 0";
  const sym1 = p.symbol1 || "Token 1";
  // Fees and net PnL are tracked as a *change* since inception (see live.go),
  // so measure accrued fees against the baseline too — the curve then starts at
  // zero like the PnL line. Falls back to the absolute balance if no baseline.
  const baseFees = (p.initialState && p.initialState.feesUsd) || 0;

  const labels = history.map((h) => new Date(h.timestamp).toLocaleTimeString());
  // Left axis: per-token USD value, stacked so the top of the band is the whole
  // LP position's worth over time.
  const token0Value = history.map((h) => (h.amount0 || 0) * (h.price0 || 0));
  const token1Value = history.map((h) => (h.amount1 || 0) * (h.price1 || 0));
  // Right axis: absolute USD lines.
  const feesData = history.map((h) => (h.feesUsd || 0) - baseFees);
  const pnlData = history.map((h) => h.netPnl || 0);
  // The slice of PnL that fees don't explain is the LP–hedge mismatch: net PnL
  // minus the cumulative fees. With the complete hedge PnL now in net PnL (LP
  // change + hedge realized + unrealized + funding − commissions), this is the
  // true impermanent-loss residual, not the old accounting gap that ignored the
  // realized PnL banked out of the open short at each rebalance.
  const lpHedgeDiff = history.map(
    (h) => (h.netPnl || 0) - ((h.feesUsd || 0) - baseFees),
  );

  const usdAxis = (value) =>
    "$" + value.toLocaleString(undefined, { maximumFractionDigits: 0 });
  const usdTip = (value) =>
    "$" +
    value.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });

  window.pnlChart = new Chart(ctx.getContext("2d"), {
    type: "line",
    data: {
      labels: labels,
      datasets: [
        // ---- Left axis: stacked LP value -----------------------------------
        {
          label: `${sym0} value`,
          data: token0Value,
          yAxisID: "y",
          stack: "lp",
          fill: true,
          backgroundColor: "rgba(56, 139, 253, 0.45)",
          borderColor: "rgba(56, 139, 253, 0.9)",
          borderWidth: 1,
          tension: 0.3,
          pointRadius: 0,
          order: 3,
        },
        {
          label: `${sym1} value`,
          data: token1Value,
          yAxisID: "y",
          stack: "lp",
          fill: true,
          backgroundColor: "rgba(63, 185, 80, 0.45)",
          borderColor: "rgba(63, 185, 80, 0.9)",
          borderWidth: 1,
          tension: 0.3,
          pointRadius: 0,
          order: 3,
        },
        // ---- Right axis: absolute USD lines --------------------------------
        {
          label: "Accrued fees (USD)",
          data: feesData,
          yAxisID: "y1",
          borderColor: "rgb(255, 159, 64)",
          backgroundColor: "rgb(255, 159, 64)",
          tension: 0.3,
          borderWidth: 2,
          pointRadius: 0,
          pointHoverRadius: 4,
          order: 1,
        },
        {
          label: "Net PnL (USD)",
          data: pnlData,
          yAxisID: "y1",
          borderColor: "rgb(45, 212, 191)",
          backgroundColor: "rgb(45, 212, 191)",
          tension: 0.3,
          borderWidth: 2.5,
          pointRadius: 0,
          pointHoverRadius: 4,
          order: 0,
        },
        {
          label: "LP − hedge mismatch (USD)",
          data: lpHedgeDiff,
          yAxisID: "y1",
          borderColor: "rgb(188, 140, 255)",
          backgroundColor: "rgb(188, 140, 255)",
          tension: 0.3,
          borderWidth: 2,
          borderDash: [5, 4],
          pointRadius: 0,
          pointHoverRadius: 4,
          order: 2,
        },
      ],
    },
    options: {
      animation: false,
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: "index", intersect: false },
      plugins: {
        legend: { position: "top", labels: { boxWidth: 12, usePointStyle: true } },
        tooltip: {
          enabled: true,
          callbacks: {
            label: (c) => `${c.dataset.label}: ${usdTip(c.parsed.y)}`,
            footer: (items) => {
              // Show the full LP value (sum of the stacked legs) on hover.
              const lp = items
                .filter((i) => i.dataset.stack === "lp")
                .reduce((s, i) => s + (i.parsed.y || 0), 0);
              return items.some((i) => i.dataset.stack === "lp")
                ? `Total LP value: ${usdTip(lp)}`
                : "";
            },
          },
        },
      },
      scales: {
        y: {
          type: "linear",
          position: "left",
          stacked: true,
          beginAtZero: true,
          title: { display: true, text: "LP value (USD)" },
          ticks: { callback: usdAxis },
        },
        y1: {
          type: "linear",
          position: "right",
          stacked: false,
          title: { display: true, text: "PnL / fees / mismatch (USD)" },
          grid: { drawOnChartArea: false },
          ticks: { callback: usdAxis },
        },
      },
    },
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
  const rest = [...state.protocolsSeen]
    .filter((p) => !focus.includes(p))
    .sort();
  const ordered = [...focus.filter((p) => state.protocolsSeen.has(p)), ...rest];
  const cur = state.filters.protocol;
  sel.innerHTML =
    '<option value="">All protocols</option>' +
    ordered
      .map(
        (p) =>
          `<option value="${p}"${p === cur ? " selected" : ""}>${p}</option>`,
      )
      .join("");
}

function countdown() {
  const el = document.getElementById("scan-label");
  if (state.scanning) {
    el.textContent = "scanning pools…";
    return;
  }
  // Pool scanning is manual-only; there is no scheduled next scan to count down.
  const secs = state.nextScan
    ? Math.round((state.nextScan - new Date()) / 1000)
    : -1;
  if (Number.isNaN(secs) || secs <= 0) {
    el.textContent = "pool scan: manual";
  } else {
    const m = Math.floor(secs / 60);
    const s = secs % 60;
    el.textContent = "next scan in " + (m > 0 ? m + "m " : "") + s + "s";
  }
}

// ---- table -----------------------------------------------------------------

function sortKey(p) {
  const a = view(p);
  switch (state.sort.key) {
    case "name":
      return p.name.toLowerCase();
    case "chain":
      return p.chain.toLowerCase();
    case "tvl":
      return p.tvlUsd;
    case "vol":
      return p.volume24hUsd;
    case "feeApr":
      return a.feeApr;
    case "realizedVol":
      return a.realizedVol;
    case "feeImpliedVol":
      return a.feeImpliedVol;
    case "band": {
      const b = rangeBands(a.realizedVol, state.horizonDays);
      return b ? (state.k === 2 ? b.up2 : b.up1) : -1;
    }
    case "estApr": {
      const bs = bandSim(a, state.k);
      return bs ? bs.apr : -Infinity;
    }
    case "netEdge":
      return a.netEdgeApr;
    case "headroom": {
      const h = volHeadroom(a.feeImpliedVol, a.realizedVol);
      return h == null ? -Infinity : h;
    }
    case "verdict":
      return a.verdict;
    default:
      return (p.analysis && p.analysis.score) || 0;
  }
}

function visiblePools() {
  const f = state.filters;
  let pools = state.pools.filter((p) => {
    if (f.kind && p.chainKind !== f.kind) return false;
    if (f.verdict && view(p).verdict !== f.verdict) return false;
    if (f.protocol && p.protocol !== f.protocol) return false;
    if (f.search) {
      const hay = (
        p.name +
        " " +
        p.chain +
        " " +
        p.dex +
        " " +
        p.baseSymbol +
        " " +
        p.quoteSymbol
      ).toLowerCase();
      if (!hay.includes(f.search.toLowerCase())) return false;
    }
    return true;
  });
  pools.sort((a, b) => {
    const ka = sortKey(a),
      kb = sortKey(b);
    if (ka < kb) return -1 * state.sort.dir;
    if (ka > kb) return 1 * state.sort.dir;
    return 0;
  });
  return pools;
}

function poolId(p) {
  return p.chainSlug + "/" + p.address;
}

function renderTable() {
  const pools = visiblePools();
  const body = document.getElementById("pools-body");
  document.getElementById("empty").hidden = pools.length > 0;

  body.innerHTML = pools
    .map((p) => {
      const a = view(p);
      const edgeCls = a.netEdgeApr >= 0 ? "ratio-pos" : "ratio-neg";
      const b = rangeBands(a.realizedVol, state.horizonDays);
      const up = b ? (state.k === 2 ? b.up2 : b.up1) : null;
      const dn = b ? (state.k === 2 ? b.dn2 : b.dn1) : null;
      const bandCell = b
        ? `<span class="ratio-pos">+${fmt.pct(up)}</span> / <span class="ratio-neg">-${fmt.pct(dn)}</span>`
        : "—";
      const bs = bandSim(a, state.k);
      const estCell = bs ? fmt.pct(bs.apr) : "—";
      const headroom = volHeadroom(a.feeImpliedVol, a.realizedVol);
      const headCls = (headroom || 0) >= 1 ? "ratio-pos" : "ratio-neg";
      const headCell = headroom == null ? "—" : headroom.toFixed(2) + "×";
      const sel = poolId(p) === state.selected ? " selected" : "";
      return `<tr class="row${sel}" data-id="${poolId(p)}">
        <td><div class="pool-cell"><span class="pool-name">${p.name}</span><span class="pool-dex">${p.protocol || p.dex}</span></div></td>
        <td><span class="chain-tag">${p.chain}<span class="layer ${p.chainKind}">${p.chainKind}</span></span></td>
        <td class="num">${fmt.usd(p.tvlUsd)}</td>
        <td class="num">${fmt.usd(p.volume24hUsd)}</td>
        <td class="num">${fmt.pct(a.positionFeeApr || a.feeApr)}</td>
        <td class="num">${fmt.pct(a.realizedVol)}</td>
        <td class="num">${fmt.pct(a.feeImpliedVol)}</td>
        <td class="num">${bandCell}</td>
        <td class="num">${estCell}</td>
        <td class="num ${edgeCls}">${fmt.pct(a.netEdgeApr)}</td>
        <td class="num ${headCls}">${headCell}</td>
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

  const a = view(p);
  const methodLabel = {
    close7d: "Close/close · 7d",
    close14d: "Close/close · 14d",
    gk: "Garman-Klass · 14d",
  }[state.method];
  const vols = [
    { label: "Breakeven σ", val: a.feeImpliedVol, cls: "fees" },
    { label: "Realized σ", val: a.realizedVol, cls: "realized" },
  ];
  if (a.hasImplied)
    vols.push({
      label: "Options-implied σ",
      val: a.impliedVol,
      cls: "implied",
    });
  const maxVol = Math.max(...vols.map((v) => v.val), 0.0001);

  const headroom = volHeadroom(a.feeImpliedVol, a.realizedVol);

  const b = rangeBands(a.realizedVol, state.horizonDays);
  const T = state.horizonDays;
  const Tlabel = T < 1 ? T * 24 + "h" : T + "d";
  const tir1 = expectedTimeInRange(1, T);
  const tir2 = expectedTimeInRange(2, T);
  const bandsHtml = b
    ? `<div class="bars-title" title="±kσ range-sizing guidance: where to set the LP to stay in range ≈68% (k=1) / ≈95% (k=2) of the time over T=${Tlabel}. A containment target, NOT an edge or a return.">LP range — ±σ containment over ${Tlabel}</div>
       <div class="metric-grid">
         <div class="metric"><div class="k">±1σ (≈68%)</div><div class="v"><span class="ratio-pos">+${fmt.pct(b.up1)}</span> / <span class="ratio-neg">-${fmt.pct(b.dn1)}</span></div></div>
         <div class="metric"><div class="k">±2σ (≈95%)</div><div class="v"><span class="ratio-pos">+${fmt.pct(b.up2)}</span> / <span class="ratio-neg">-${fmt.pct(b.dn2)}</span></div></div>
         <div class="metric"><div class="k" title="Expected time in range = k²·T days under driftless GBM with instant re-centring; σ cancels because the band is sized by σ. Informational only — an idealized mean, not a guarantee; the price can re-touch the edge several times.">Time in range ±1σ</div><div class="v">${fmt.duration(tir1)}</div></div>
         <div class="metric"><div class="k" title="Expected time in range = k²·T days. Informational only; does not feed the verdict.">Time in range ±2σ</div><div class="v">${fmt.duration(tir2)}</div></div>
       </div>`
    : "";

  const verdictText = {
    attractive:
      "Fees are pricing in more volatility than the asset actually shows — LPs are being overpaid for the risk.",
    fair: "Fee income roughly matches the cost of the asset's volatility.",
    unattractive:
      "The asset's volatility costs more than the pool's fees pay — LPs likely lose to rebalancing.",
    unknown: "Not enough data to judge this pool.",
  }[a.verdict];

  // Range simulation: estimated fee APR for a $1,000 deposit into the selected
  // ±kσ band, as the deposit's share of the REAL in-range liquidity. Tracks the
  // band chips (state.k) and shows whether it came from chain or an estimate.
  const sim = bandSim(a, state.k);
  const src = a.concentratedSim ? a.concentratedSim.source : null;
  const simHtml = sim
    ? `<div class="bars-title" title="Estimated fee APR for a $1,000 deposit into the selected ±kσ band: the deposit's share of the in-range liquidity. share = Lyou/(Lexisting+Lyou); APR = volume·fee·365·share/deposit. On-chain liquidity where available.">Range simulation · est. APR for $1k (±${state.k}σ)${src ? ` · <span class="src ${src}">${src}</span>` : ""}</div>
       <div class="metric-grid">
         <div class="metric"><div class="k">Est. APR</div><div class="v ratio-pos">${fmt.pct(sim.apr)}</div></div>
         <div class="metric"><div class="k" title="Capital efficiency of this band vs full range.">Concentration</div><div class="v">${sim.concentrationX.toFixed(1)}×</div></div>
         <div class="metric"><div class="k">On deposit</div><div class="v">${fmt.usd(a.concentratedSim ? a.concentratedSim.depositUsd : 1000)}</div></div>
         <div class="metric"><div class="k">Est. fees / day</div><div class="v">${fmt.usd(sim.dailyFeesUsd)}</div></div>
       </div>
       <p class="hint" style="margin:6px 0 0">Range ${fmt.price(sim.lowerPrice)} – ${fmt.price(sim.upperPrice)} · competing in-range liquidity ≈ ${fmt.usd(sim.activeLiquidityUsd)} · your share ≈ ${fmt.pct(sim.share)}</p>`
    : "";

  content.innerHTML = `
    <h2>${p.name}</h2>
    <div class="sub">${p.dex} · ${p.chain} <span class="layer ${p.chainKind}">${p.chainKind}</span> · fee ${fmt.pct(p.feeTier)}</div>
    <div class="sub">σ method: <strong>${methodLabel}</strong></div>

    <div class="verdict-banner ${a.verdict}">
      <strong style="text-transform:capitalize">${a.verdict}.</strong> ${verdictText}
    </div>

    <div class="metric-grid">
      <div class="metric"><div class="k">Fee APR</div><div class="v" title="Pool-wide APR: ${fmt.pct(a.feeApr)}">${fmt.pct(a.positionFeeApr || a.feeApr)}</div></div>
      <div class="metric"><div class="k" title="Volatility headroom = breakeven σ / realized σ = √(8·feeAPR)/σ. Above 1× means the pool's fees overpay for the realized volatility. It is the √ of the old fee/vol APR ratio.">Vol headroom</div><div class="v ${(headroom || 0) >= 1 ? "ratio-pos" : "ratio-neg"}">${headroom == null ? "—" : headroom.toFixed(2) + "×"}</div></div>
      <div class="metric"><div class="k" title="Full-range net edge = feeAPR − σ²/8, the APR-space attractiveness magnitude.">Net edge APR</div><div class="v ${a.netEdgeApr >= 0 ? "ratio-pos" : "ratio-neg"}">${fmt.pct(a.netEdgeApr)}</div></div>
      <div class="metric"><div class="k">LVR cost (σ²⁄8)</div><div class="v">${fmt.pct(a.lvrCost)}</div></div>
    </div>

    ${bandsHtml}

    ${simHtml}

    <div class="bars-title" title="Breakeven σ = √(8·feeAPR) is the concentration-invariant (full-range) breakeven — OPTIMISTIC, since the pool's real on-chain concentration would lower it, and it counts swap fees only (excludes farm/gauge incentives like AERO).">Volatility comparison (annualized)</div>
    <div class="bars">
      ${vols
        .map(
          (v) => `
        <div class="bar-row">
          <span class="label">${v.label}</span>
          <span class="bar-track"><span class="bar-fill ${v.cls}" style="width:${Math.max(2, (v.val / maxVol) * 100)}%"></span></span>
          <span class="val">${fmt.pct(v.val)}</span>
        </div>`,
        )
        .join("")}
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
  if (!closes || closes.length < 2)
    return '<p class="hint">No price history.</p>';
  const w = 320,
    h = 70,
    pad = 4;
  const min = Math.min(...closes),
    max = Math.max(...closes);
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
  document.querySelectorAll("#method-filters .chip").forEach((b) => {
    b.addEventListener("click", () => {
      state.method = b.dataset.method;
      setActive("#method-filters", b);
      renderSummary(state.meta || {});
      renderTable();
      renderPosition();
      if (state.selected) renderDetail(findSelected());
    });
  });
  document.querySelectorAll("#horizon-filters .chip").forEach((b) => {
    b.addEventListener("click", () => {
      state.horizonDays = parseFloat(b.dataset.horizon);
      setActive("#horizon-filters", b);
      renderTable();
      renderPosition();
      if (state.selected) renderDetail(findSelected());
    });
  });
  document.querySelectorAll("#k-filters .chip").forEach((b) => {
    b.addEventListener("click", () => {
      state.k = parseFloat(b.dataset.k);
      setActive("#k-filters", b);
      renderTable();
      renderPosition();
      if (state.selected) renderDetail(findSelected());
    });
  });
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
      else
        state.sort = {
          key,
          dir: key === "name" || key === "chain" || key === "verdict" ? 1 : -1,
        };
      renderTable();
    });
  });
  document.getElementById("scan-now").addEventListener("click", async (e) => {
    e.target.disabled = true;
    e.target.textContent = "Scanning…";
    try {
      await fetch("/api/scan", { method: "POST" });
    } catch (_) {}
    setTimeout(async () => {
      await refresh();
      e.target.disabled = false;
      e.target.textContent = "Scan now";
    }, 1500);
  });

  wireTrackedEditor();
}

// wireTrackedEditor lets the user change which LP position token IDs are tracked,
// and add multiple, from the dashboard. It POSTs the comma-separated list to
// /api/tracked and refreshes so the new portfolio and its single aggregated
// hedge appear.
function wireTrackedEditor() {
  const editor = document.getElementById("tracked-editor");
  const input = document.getElementById("tracked-input");
  const msg = document.getElementById("tracked-editor-msg");
  const editBtn = document.getElementById("edit-tracked");
  if (!editor || !input || !editBtn) return;

  const close = () => {
    editor.hidden = true;
    msg.textContent = "";
  };
  const open = async () => {
    msg.textContent = "";
    editor.hidden = false;
    // Prefill with the authoritative tracked set from the server (falls back to
    // the IDs visible on the current position payload).
    let ids = [];
    try {
      const resp = await getJSON("/api/tracked");
      ids = resp.tokenIds || [];
    } catch (_) {
      const p = state.position;
      if (p) ids = (p.positions || []).map((x) => x.tokenId);
      if (!ids.length && p && p.tokenId) ids = [p.tokenId];
    }
    input.value = ids.join(", ");
    input.focus();
    input.select();
  };

  const save = async () => {
    const raw = input.value.trim();
    const ids = (raw.match(/\d+/g) || []).map((n) => parseInt(n, 10));
    if (!ids.length) {
      msg.textContent = "Enter at least one token ID.";
      msg.className = "tracked-editor-msg err";
      return;
    }
    const saveBtn = document.getElementById("tracked-save");
    saveBtn.disabled = true;
    msg.textContent = "Saving…";
    msg.className = "tracked-editor-msg";
    try {
      const r = await fetch("/api/tracked", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tokenIds: ids }),
      });
      if (!r.ok) throw new Error(await r.text());
      const data = await r.json();
      msg.textContent =
        "Now tracking " + (data.tokenIds || []).length + " position(s).";
      close();
      await refresh();
    } catch (e) {
      msg.textContent = "Failed: " + e;
      msg.className = "tracked-editor-msg err";
    } finally {
      saveBtn.disabled = false;
    }
  };

  editBtn.addEventListener("click", () => {
    if (editor.hidden) open();
    else close();
  });
  document.getElementById("tracked-save").addEventListener("click", save);
  document.getElementById("tracked-cancel").addEventListener("click", close);
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") save();
    else if (e.key === "Escape") close();
  });
}

function setPage(page) {
  state.page = page === "pools" ? "pools" : "positions";
  document.getElementById("positions-page").hidden = state.page !== "positions";
  document.getElementById("pools-page").hidden = state.page !== "pools";
  const scanStatus = document.getElementById("pool-scan-status");
  if (scanStatus) scanStatus.hidden = state.page !== "pools";
  document.querySelectorAll("[data-page-link]").forEach((link) => {
    link.classList.toggle("active", link.dataset.pageLink === state.page);
  });
}

function setActive(group, btn) {
  document
    .querySelectorAll(group + " .chip")
    .forEach((b) => b.classList.remove("active"));
  btn.classList.add("active");
}

setPage(state.page);
wireControls();
refresh();
setInterval(refresh, 5000);
setInterval(countdown, 1000);
window.cancelOrder = async function (symbol, orderId) {
  try {
    const r = await fetch(
      `/api/orders?symbol=${encodeURIComponent(symbol)}&orderId=${orderId}`,
      { method: "DELETE" },
    );
    if (!r.ok) throw new Error(await r.text());
    refresh();
  } catch (e) {
    alert("Failed to cancel order: " + e);
  }
};
