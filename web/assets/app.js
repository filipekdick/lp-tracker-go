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
    const [meta, poolsResp, posResp] = await Promise.all([
      getJSON("/api/meta"),
      getJSON("/api/pools"),
      getJSON("/api/position").catch(() => ({ tracking: false })),
    ]);
    state.meta = meta;
    state.pools = poolsResp.pools || [];
    state.position = posResp && posResp.tracking ? posResp.position : null;
    state.nextScan = poolsResp.nextScan ? new Date(poolsResp.nextScan) : null;
    state.scanning = !!poolsResp.scanning;
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

function renderPosition() {
  const section = document.getElementById("tracked");
  const p = state.position;
  if (!p) {
    section.hidden = true;
    return;
  }
  section.hidden = false;

  document.getElementById("tracked-sub").innerHTML =
    `${p.protocol} · ${p.chain} <span class="layer ${p.chainKind}">${p.chainKind}</span> · token #${p.tokenId}` +
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
  const hedgePnl = (p.hedges || []).reduce(
    (s, h) => s + (h.unrealizedPnl || 0),
    0,
  );

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
        feesUsd;

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
      <div class="pcard-big ratio-pos">${fmt.usd(feesUsd)}</div>
      <div class="pcard-subs">
        ${kv("Unclaimed fees", fmt.usd(feesUsd))}
        ${kv("Fee APR", fmt.pct(a.positionFeeApr || a.feeApr))}
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
    <div class="kv liq-total"><span class="k">Total fees &amp; rewards</span><span class="v"><span class="ratio-pos">${fmt.usd(feesUsd)}</span></span></div>
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

  document.getElementById("position-summary").innerHTML = summaryCards;
  // Second row: exactly three cards — Liquidity Position (with range details),
  // Fees & Rewards (with the fee-vs-volatility analysis), and the Perp hedge.
  document.getElementById("tracked-grid").innerHTML = liq + feesRewards + hedge;

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

  const graphsSec = document.getElementById("graphs-section");
  if (p.initialState && p.history && p.history.length > 0) {
    graphsSec.hidden = false;

    const cur = p.history[p.history.length - 1];
    const netPnl = cur.netPnl;
    const initialVal = Math.max(0.01, p.initialState.valueUsd);
    const pct = (netPnl / initialVal) * 100;

    const msElapsed =
      new Date(cur.timestamp) - new Date(p.initialState.timestamp);
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

    let canvasHtml = `<div id="pnl-chart-wrapper" style="position: relative; height: 340px; width: 100%;"><canvas id="pnl-chart"></canvas></div>`;
    graphsSec.innerHTML = pnlHtml + canvasHtml;

    renderChart(p);
  } else {
    graphsSec.hidden = true;
  }
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
  // The slice of PnL that fees don't explain is the residual from the LP and
  // the hedge not staying perfectly in sync (impermanent loss + delta drift).
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
    case "timeInRange": {
      // k²·T is the same for every pool (σ cancels); kept so the column is
      // clickable, but it does not reorder rows.
      const t = expectedTimeInRange(state.k, state.horizonDays);
      return t == null ? -Infinity : t;
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
      const tirCell = fmt.duration(expectedTimeInRange(state.k, state.horizonDays));
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
        <td class="num">${tirCell}</td>
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
}

function setActive(group, btn) {
  document
    .querySelectorAll(group + " .chip")
    .forEach((b) => b.classList.remove("active"));
  btn.classList.add("active");
}

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
